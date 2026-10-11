package control

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
)

// EvidenceManifest identifies local files and frozen public settings; no authentication object is serialized.
// Incomplete is the initial durable state; only a successful flush/publication can replace it.
type EvidenceManifest struct {
	Version         int              `json:"schema_version"`
	RunID           string           `json:"run_id"`
	SessionID       string           `json:"session_id"`
	Endpoint        string           `json:"endpoint"`
	PlanHash        string           `json:"plan_hash"`
	Plan            profile.Plan     `json:"requested_plan"`
	Settings        EvidenceSettings `json:"effective_settings"`
	Created         string           `json:"created_utc"`
	OS              string           `json:"os"`
	Arch            string           `json:"arch"`
	Go              string           `json:"go_version"`
	Revision        string           `json:"tool_revision"`
	Modified        string           `json:"tool_modified"`
	AdapterContract int              `json:"adapter_contract_version"`
	Clock           string           `json:"clock"`
	State           string           `json:"state"`
	Complete        bool             `json:"local_evidence_complete"`
	Remote          string           `json:"remote_evidence_status"`
	RawFile         string           `json:"raw_file"`
	SummaryFile     string           `json:"summary_file,omitempty"`
	RawBytes        profile.Decimal  `json:"raw_bytes"`
	Events          profile.Decimal  `json:"raw_events"`
	SHA256          string           `json:"raw_sha256,omitempty"`
	Storage         string           `json:"storage_status"`
}

// EvidenceSettings retains only frozen binding/marking settings, excluding later control counters and lifecycle status.
type EvidenceSettings struct {
	Client Effective `json:"client"`
	Server Effective `json:"server"`
}

// EvidenceEvent binds exact engine observations to host-local identities and decimal nanoseconds.
// Details reuse the bounded engine record; final cutoff details cannot revise earlier raw observations.
type EvidenceEvent struct {
	Version   int               `json:"schema_version"`
	ID        profile.Decimal   `json:"event_id"`
	RunID     string            `json:"run_id"`
	SessionID string            `json:"session_id"`
	Endpoint  string            `json:"endpoint"`
	Epoch     uint32            `json:"clock_epoch_id"`
	At        profile.Decimal   `json:"time_ns"`
	Type      string            `json:"event_type"`
	Identity  *measure.Identity `json:"original_identity,omitempty"`
	Direction byte              `json:"original_direction,omitempty"`
	Details   json.RawMessage   `json:"details"`
}

// EvidenceSummary links endpoint-only derived quantities to the exact raw file and cutoff event.
// Remote delivery and TCP packet loss remain unknown; local replay verifies the derived quantities separately.
type EvidenceSummary struct {
	Version   int             `json:"schema_version"`
	RunID     string          `json:"run_id"`
	SessionID string          `json:"session_id"`
	Endpoint  string          `json:"endpoint"`
	PlanHash  string          `json:"plan_hash"`
	RawFile   string          `json:"raw_file"`
	Events    profile.Decimal `json:"raw_events"`
	SHA256    string          `json:"raw_sha256"`
	Local     *LocalRun       `json:"local"`
	Storage   string          `json:"storage_status"`
}

// evidenceWriter has one disk worker and independent active-inclusive queue/file accounting.
// Disk calls cannot be forcibly interrupted portably; an expired join leaves the initial incomplete manifest.
type evidenceWriter struct {
	mu                              sync.Mutex
	queue                           chan []byte
	done, failed                    chan struct{}
	err                             error
	closed                          bool
	pending, charged, queuedBytes   uint64
	sequence, written               uint64
	rawBytes                        uint64
	queueBytes, rawLimit            uint64
	manifestReserve, summaryReserve uint64
	file                            evidenceFile
	digest                          hash.Hash
	path                            string
	manifest                        EvidenceManifest
	timeout                         time.Duration
}

// evidenceFile keeps disk fault fixtures at the actual write/flush boundary without transport substitutions.
type evidenceFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

// newEvidence creates exclusive restricted per-run files and syncs public settings before measurement.
// Final publication space, including simultaneously present temporary files, is reserved up front.
func newEvidence(store *Store, p profile.Plan, ready ReadyResult, endpoint string) (*evidenceWriter, error) {
	if endpoint != "client" && endpoint != "server" {
		return nil, errors.New("invalid evidence endpoint")
	}
	if _, err := hex.DecodeString(ready.RunID); err != nil || len(ready.RunID) != 32 {
		return nil, errors.New("invalid evidence run ID")
	}
	if _, err := hex.DecodeString(ready.SessionID); err != nil || len(ready.SessionID) != 32 {
		return nil, errors.New("invalid evidence session ID")
	}
	h, err := p.Hash()
	if err != nil || ready.PlanHash != hex.EncodeToString(h[:]) {
		return nil, errors.New("evidence plan mismatch")
	}
	m := EvidenceManifest{Version: 1, RunID: ready.RunID, SessionID: ready.SessionID, Endpoint: endpoint, PlanHash: ready.PlanHash, Plan: p, Settings: EvidenceSettings{ready.Client, ready.Server}, Created: time.Now().UTC().Format(time.RFC3339Nano), OS: runtime.GOOS, Arch: runtime.GOARCH, Go: runtime.Version(), Revision: "unknown", Modified: "unknown", AdapterContract: p.Contract, Clock: "Go host-local monotonic nanoseconds; epoch 1 zero at local finite start; wall label is storage creation only; uncalibrated", State: "incomplete", Remote: "unknown; not transferred or reconciled", RawFile: "events.jsonl", Storage: "not_finalized"}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			if setting.Key == "vcs.revision" {
				m.Revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				m.Modified = setting.Value
			}
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	// Detach public maps/slices so later caller edits cannot rewrite the persisted frozen settings.
	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	// Worst-case spike/stall identities are charged per ledger slot; phase summaries have separate room.
	summaryReserve := uint64(32768 + (len(p.Phases)+1)*8192 + p.Limits.LedgerMessages*1024)
	manifestReserve := uint64(len(b) + 4096)
	reserve := summaryReserve + 2*manifestReserve + 4096
	if reserve >= uint64(p.Limits.FileBytes) {
		return nil, errors.New("file_bytes cannot reserve local summary/manifest publication space")
	}
	root := filepath.Join(store.Root, "reports")
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("reports must be an owned real directory")
	}
	if err = restrictDirectory(root); err != nil {
		return nil, err
	}
	dir := filepath.Join(root, ready.RunID+"-"+endpoint)
	if err = os.Mkdir(dir, 0700); err != nil {
		return nil, errors.New("evidence destination already exists or cannot be created")
	}
	if err = restrictDirectory(dir); err != nil {
		return nil, err
	}
	if err = publishEvidence(dir, "manifest.json", b, false); err != nil {
		return nil, err
	}
	f, err := createPrivate(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		return nil, err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return nil, err
	}
	w := &evidenceWriter{queue: make(chan []byte, p.Limits.QueueMessages), done: make(chan struct{}), failed: make(chan struct{}), queueBytes: uint64(p.Limits.EventBytes), rawLimit: uint64(p.Limits.FileBytes) - reserve, manifestReserve: manifestReserve, summaryReserve: summaryReserve, file: f, digest: sha256.New(), path: dir, manifest: m, timeout: time.Duration(p.Limits.ShutdownTimeout)}
	go w.writeLoop()
	return w, nil
}

// publishEvidence syncs a same-directory exclusive staging file and atomically publishes it.
// Replacement is allowed only for this run's owned manifest; an existing summary is never overwritten.
func publishEvidence(dir, name string, b []byte, replace bool) error {
	path := filepath.Join(dir, name)
	if !replace {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return errors.New("evidence file already exists or cannot be inspected")
		}
	}
	f, err := createPrivate(path + ".pending")
	if err != nil {
		return err
	}
	n, writeErr := f.Write(b)
	if writeErr == nil && n != len(b) {
		writeErr = errors.New("short evidence publication write")
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	err = errors.Join(writeErr, f.Close())
	if err != nil {
		return err
	}
	return replaceFile(path+".pending", path)
}

// event wraps only the already filtered record, excluding keys, payloads, authentication tags and channels.
func (w *evidenceWriter) event(kind string, at profile.Decimal, details json.RawMessage) ([]byte, error) {
	e := EvidenceEvent{Version: 1, ID: profile.Decimal(w.sequence + 1), RunID: w.manifest.RunID, SessionID: w.manifest.SessionID, Endpoint: w.manifest.Endpoint, Epoch: 1, At: at, Type: kind, Details: details}
	if kind != "final_cutoff" {
		var r engine.Record
		if err := json.Unmarshal(details, &r); err != nil {
			return nil, err
		}
		if r.Identity.Lifecycle != 0 {
			e.Identity = &r.Identity
		}
		e.Direction = r.Direction
	}
	return json.Marshal(e)
}

// Append charges exact JSONL bytes before nonblocking admission; saturation ends measurement visibly.
// The active disk write remains charged until it returns, so a stalled disk cannot grow the queue.
func (w *evidenceWriter) Append(record json.RawMessage) error {
	var r engine.Record
	if err := json.Unmarshal(record, &r); err != nil {
		return errors.New("invalid local evidence record")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.err != nil {
		return errors.New("local evidence writer unavailable")
	}
	b, err := w.event(r.Kind, r.At, record)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	n := uint64(len(b))
	if n > w.rawLimit-w.charged {
		return errors.New("local evidence file budget exhausted")
	}
	if w.pending >= uint64(cap(w.queue)) || n > w.queueBytes-w.queuedBytes {
		return errors.New("local evidence storage queue exhausted")
	}
	w.queue <- b // Active-inclusive capacity check guarantees this sole-owner admission cannot block.
	w.sequence++
	w.pending++
	w.charged += n
	w.queuedBytes += n
	return nil
}

// Failed exposes asynchronous storage failure so the collector stops even during a quiet phase.
func (w *evidenceWriter) Failed() <-chan struct{} { return w.failed }

// writeLoop writes FIFO records without a dashboard dependency, retaining partial-write byte evidence.
// After failure it drains admissions without claiming they were saved, then syncs/closes on finalization.
func (w *evidenceWriter) writeLoop() {
	defer close(w.done)
	for b := range w.queue {
		w.mu.Lock()
		failed := w.err != nil
		w.mu.Unlock()
		var n int
		var err error
		if !failed {
			n, err = w.file.Write(b)
			if n > 0 {
				w.digest.Write(b[:n])
				w.rawBytes += uint64(n)
			}
			if err == nil && n != len(b) {
				err = errors.New("short raw evidence write")
			}
			if err == nil {
				w.written++
			}
		}
		w.mu.Lock()
		w.pending--
		w.queuedBytes -= uint64(len(b))
		if err != nil && w.err == nil {
			w.err = errors.New("raw evidence disk write failed")
			close(w.failed)
		}
		w.mu.Unlock()
	}
	err := errors.Join(w.file.Sync(), w.file.Close())
	w.mu.Lock()
	if err != nil && w.err == nil {
		w.err = errors.New("raw evidence flush/close failed")
		close(w.failed)
	}
	w.mu.Unlock()
}

// Finalize drains accepted raw records, appends exact final cutoffs, then publishes summary and manifest.
// A failure retains partial/unknown status; a crash or join timeout leaves the initial incomplete manifest.
func (w *evidenceWriter) Finalize(r engine.Result) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errors.New("evidence already finalized")
	}
	w.closed = true
	close(w.queue)
	w.mu.Unlock()
	timer := time.NewTimer(w.timeout)
	defer timer.Stop()
	select {
	case <-w.done:
	case <-timer.C:
		return errors.New("evidence disk writer join timed out; files remain incomplete")
	}
	storageErr := w.err
	if storageErr != nil {
		r.Complete, r.Receiver.Complete = false, false
	}
	details, _ := json.Marshal(struct {
		Generation  profile.Decimal `json:"generation_cutoff_ns"`
		Observation profile.Decimal `json:"observation_cutoff_ns"`
		Complete    bool            `json:"endpoint_observation_complete"`
	}{r.GenerationCutoff, r.ObservationCutoff, r.Complete})
	b, err := w.event("final_cutoff", r.ObservationCutoff, details)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	// The final record has independent reserved space even after ordinary evidence exhausts its cap.
	if len(b) > 4096 {
		return errors.New("final evidence record exceeds reservation")
	}
	if storageErr == nil {
		f, openErr := os.OpenFile(filepath.Join(w.path, "events.jsonl"), os.O_WRONLY|os.O_APPEND, 0600)
		if openErr == nil {
			n, writeErr := f.Write(b)
			if n > 0 {
				w.digest.Write(b[:n])
				w.rawBytes += uint64(n)
			}
			if writeErr == nil && n != len(b) {
				writeErr = errors.New("short cutoff write")
			}
			storageErr = errors.Join(writeErr, f.Sync(), f.Close())
			if storageErr == nil {
				w.written++
			}
		} else {
			storageErr = openErr
		}
	}
	if storageErr != nil {
		r.Complete, r.Receiver.Complete = false, false
	}
	status := "saved"
	if storageErr != nil {
		status = "failed; raw evidence incomplete"
	}
	digest := hex.EncodeToString(w.digest.Sum(nil))
	summary := EvidenceSummary{1, w.manifest.RunID, w.manifest.SessionID, w.manifest.Endpoint, w.manifest.PlanHash, "events.jsonl", profile.Decimal(w.written), digest, localSnapshot(r), status}
	b, err = json.Marshal(summary)
	if err != nil || uint64(len(b)) > w.summaryReserve {
		return errors.New("local summary exceeds reserved space")
	}
	if err = publishEvidence(w.path, "summary.json", b, false); err != nil {
		return errors.New("local summary publication failed; manifest remains incomplete")
	}
	w.manifest.Complete = r.Complete
	w.manifest.State = "partial"
	if r.Complete {
		w.manifest.State = "completed"
	}
	w.manifest.SummaryFile, w.manifest.Storage = "summary.json", status
	w.manifest.RawBytes, w.manifest.Events, w.manifest.SHA256 = profile.Decimal(w.rawBytes), profile.Decimal(w.written), digest
	b, err = json.Marshal(w.manifest)
	if err != nil || uint64(len(b)) > w.manifestReserve {
		return errors.New("final manifest exceeds reserved space")
	}
	if err = publishEvidence(w.path, "manifest.json", b, true); err != nil {
		return errors.New("local manifest publication failed; files remain incomplete")
	}
	if storageErr != nil {
		return errors.New("local evidence storage failed")
	}
	return nil
}
