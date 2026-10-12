package control

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// ReplayedReport retains frozen settings, exact validated envelopes and independently recomputed quantities.
// Integrity is a local consistency check, never authentication of imported or remote evidence.
type ReplayedReport struct {
	Version         int                `json:"schema_version"`
	Manifest        EvidenceManifest   `json:"manifest"`
	Summary         EvidenceSummary    `json:"recomputed_summary"`
	Status          string             `json:"status"`
	Reason          string             `json:"reason"`
	Verification    string             `json:"saved_summary_verification"`
	Events          []json.RawMessage  `json:"-"`
	TailBytes       profile.Decimal    `json:"discarded_tail_bytes"`
	Integrity       string             `json:"raw_integrity"`
	Units           string             `json:"units"`
	Accounting      []string           `json:"accounting_errors"`
	Evidence        engine.Result      `json:"-"`
	Reconciliation  *Reconciliation    `json:"reconciliation,omitempty"`
	ServerReport    *ReplayedReport    `json:"server_report,omitempty"`
	TransferReceipt *RemoteTransfer    `json:"server_evidence_transfer,omitempty"`
	Clock           *ClockSnapshot     `json:"clock_sampling,omitempty"`
	Presentation    ReportPresentation `json:"evidence_presentation"`
}

// decodeReportObject rejects ambiguity, missing/unknown fields and null scalars while allowing saved nil collections.
func decodeReportObject(b []byte, out any) error {
	if err := uniqueJSON(b); err != nil {
		return err
	}
	if err := requiredJSONFields(b, reflect.TypeOf(out).Elem(), true); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}

// reportFile reads only regular, nonsymlink fixed filenames within an explicit aggregate input-byte budget.
// This is offline inspection: it neither restricts permissions nor mutates the source report.
func reportFile(dir, name string, remaining *uint64) ([]byte, error) {
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || uint64(info.Size()) > *remaining {
		return nil, errors.New("report file is not regular or exceeds input budget")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("report file changed while opening")
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(*remaining)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(b)) > *remaining {
		return nil, errors.New("report input budget exhausted")
	}
	*remaining -= uint64(len(b))
	return b, nil
}

// ReplayReport validates run/plan/schema and replays a bounded immutable snapshot of local files.
// Only a finalized saved manifest and matching cutoff can authorize complete endpoint observations.
// A torn last line in failed/unfinalized storage is excluded, leaving a visibly partial valid prefix.
// An optional startup registry permits registered extensions; unknown contracts never fall back.
func ReplayReport(dir string, maxBytes uint64, registries ...*adapter.Registry) (*ReplayedReport, error) {
	if maxBytes == 0 || maxBytes > 1<<30 {
		return nil, errors.New("max-bytes must be explicit and between 1 and 1073741824")
	}
	remaining := maxBytes
	return replayReport(dir, &remaining, true, registries...)
}

// replayReport shares the caller's aggregate byte budget with optional transferred endpoint files.
// Remote sidecars cannot change the endpoint-only saved summary or confer offline authentication.
func replayReport(dir string, budget *uint64, remote bool, registries ...*adapter.Registry) (*ReplayedReport, error) {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("report must be a real directory")
	}
	remaining := *budget
	defer func() { // Charge all inspected fixed files, including failed replay attempts, to one aggregate budget.
		*budget = remaining
	}()
	b, err := reportFile(dir, "manifest.json", &remaining)
	if err != nil {
		return nil, err
	}
	if len(b) > profile.DocumentMax {
		return nil, errors.New("manifest exceeds schema budget")
	}
	var m EvidenceManifest
	if err = decodeReportObject(b, &m); err != nil {
		return nil, err
	}
	if m.Version != 1 || m.AdapterContract != m.Plan.Contract || m.RawFile != "events.jsonl" || m.SummaryFile != "" && m.SummaryFile != "summary.json" {
		return nil, errors.New("unsupported report schema/files")
	}
	for _, id := range []string{m.RunID, m.SessionID} {
		if _, err = hex.DecodeString(id); err != nil || len(id) != 32 || strings.ToLower(id) != id {
			return nil, errors.New("invalid report identity")
		}
	}
	if m.State != "incomplete" && m.State != "partial" && m.State != "completed" || m.Complete != (m.State == "completed") {
		return nil, errors.New("invalid manifest completeness")
	}
	if m.Storage != "saved" && m.Storage != "not_finalized" && m.Storage != "failed; raw evidence incomplete" || m.State == "incomplete" && m.Storage != "not_finalized" || m.State != "incomplete" && (m.Storage == "not_finalized" || m.SummaryFile != "summary.json") || m.Complete && m.Storage != "saved" {
		return nil, errors.New("invalid manifest storage status")
	}
	h, err := m.Plan.Hash()
	if err != nil || hex.EncodeToString(h[:]) != m.PlanHash {
		return nil, errors.New("report plan hash mismatch")
	}
	direction := byte(1)
	if m.Endpoint == "server" {
		direction = 2
	} else if m.Endpoint != "client" {
		return nil, errors.New("invalid report endpoint")
	}
	c, err := engine.NewCollector(m.Plan, direction, 0, registries...)
	if err != nil {
		return nil, err
	}
	raw, err := reportFile(dir, "events.jsonl", &remaining)
	if err != nil {
		return nil, err
	}
	if uint64(len(raw)) > uint64(m.Plan.Limits.FileBytes) {
		return nil, errors.New("raw file exceeds frozen file budget")
	}
	digest := sha256.Sum256(raw)
	sha := hex.EncodeToString(digest[:])
	finalized := m.State != "incomplete"
	if finalized && (m.RawBytes != profile.Decimal(len(raw)) || m.SHA256 != sha) {
		return nil, errors.New("raw checksum/byte mismatch")
	}
	r := &ReplayedReport{Version: 1, Manifest: m, Status: "partial", Reason: "incomplete local storage or observation; remote evidence remains unknown", Verification: "unavailable; manifest not finalized", Integrity: "unverified; initial manifest has no finalized checksum", Units: "time offsets and durations: ns in their host-local epoch; byte quantities: bytes; populations: message/sample/pair counts; rates: messages per denominator seconds; no calibrated one-way values", Accounting: []string{}}
	if finalized {
		r.Integrity = "matched finalized raw byte count and SHA-256; local consistency only"
	}
	var generation, observation profile.Decimal
	var cutoffComplete, cutoff bool
	var clockRetention string
	clock := clockWindow{policy: m.Plan.Calibration, recent: make([]ClockCapture, 0, m.Plan.Calibration.Window)}
	var clockFinished profile.Decimal
	scan := bufio.NewReaderSize(bytes.NewReader(raw), 4096)
	for {
		line, readErr := scan.ReadSlice('\n')
		// Envelopes can exceed the small reader buffer but never the schema's 1 MiB limit.
		for readErr == bufio.ErrBufferFull && len(line) <= FrameMax {
			line = bytes.Clone(line)
			part, e := scan.ReadSlice('\n')
			line = append(line, part...)
			readErr = e
		}
		if len(line) > FrameMax {
			return nil, errors.New("raw event exceeds schema budget")
		}
		if readErr == io.EOF {
			if len(line) != 0 && (m.Storage == "saved" || m.Complete) {
				return nil, errors.New("saved raw file has a torn tail")
			}
			r.TailBytes = profile.Decimal(len(line))
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		if cutoff {
			return nil, errors.New("event after final cutoff")
		}
		var e EvidenceEvent
		if err = decodeReportObject(line, &e); err != nil {
			return nil, fmt.Errorf("event %d: %w", len(r.Events)+1, err)
		}
		if e.Version != 1 || e.ID != profile.Decimal(len(r.Events)+1) || e.RunID != m.RunID || e.SessionID != m.SessionID || e.Endpoint != m.Endpoint || e.Epoch != 1 {
			return nil, errors.New("event envelope identity/version mismatch")
		}
		if e.Type == "final_cutoff" {
			var end struct {
				Generation     profile.Decimal `json:"generation_cutoff_ns"`
				Observation    profile.Decimal `json:"observation_cutoff_ns"`
				Complete       bool            `json:"endpoint_observation_complete"`
				ClockRetention string          `json:"clock_retention_reason,omitempty"`
			}
			if err = decodeReportObject(e.Details, &end); err != nil {
				return nil, err
			}
			if end.Observation != e.At || e.Identity != nil || e.Direction != 0 {
				return nil, errors.New("invalid final cutoff envelope")
			}
			generation, observation, cutoffComplete, cutoff = end.Generation, end.Observation, end.Complete, true
			if end.ClockRetention != "" && (m.Endpoint != "client" || end.ClockRetention != "calibration evidence budget/storage unavailable") {
				return nil, errors.New("invalid clock retention reason")
			}
			clockRetention = end.ClockRetention
		} else if e.Type == "clock_capture" {
			var capture ClockCapture
			if err = decodeReportObject(e.Details, &capture); err != nil {
				return nil, err
			}
			if len(line) > clockFrameMax || capture.Sent > clockFrameMax+4 || capture.Received > 2*(clockFrameMax+4) || capture.Skipped > profile.Decimal(clockBudget(m.Plan)) || m.Endpoint != "client" || e.Identity != nil || e.Direction != 0 || e.At != capture.Finished || clock.totals.Attempts >= clockBudget(m.Plan) || capture.ID != fmt.Sprintf("clock-%d", clock.totals.Attempts+1) || capture.ClientEpoch != e.Epoch || capture.Valid && (validateCapture(capture) != "" || capture.Reason != "" || capture.ResponseID != capture.ID || capture.ResponseRun != m.RunID || capture.ResponseSession != m.SessionID || capture.ResponseHash != m.PlanHash) || !capture.Valid && capture.Reason == "" || clock.totals.Attempts > 0 && capture.Finished < clockFinished {
				return nil, errors.New("invalid raw calibration capture")
			}
			clock.add(capture)
			clockFinished = capture.Finished
		} else {
			var record engine.Record
			if err = decodeReportObject(e.Details, &record); err != nil {
				return nil, err
			}
			switch record.Kind {
			case "invalid", "read_error", "marking", "generation_end", "observation_end", "schedule_skip", "admission", "unattempted", "attempt", "write", "arrival", "duplicate", "ticket_observed":
			default:
				return nil, errors.New("unsupported raw event kind")
			}
			if record.Kind != e.Type || record.At != e.At || record.Direction != e.Direction || record.Bytes > 1<<30 || record.Identity.Lifecycle != 0 && (e.Identity == nil || *e.Identity != record.Identity) || record.Identity.Lifecycle == 0 && e.Identity != nil {
				return nil, errors.New("event details/envelope mismatch")
			}
			if len(r.Accounting) > 0 && record.Kind != "ticket_observed" {
				return nil, errors.New("measurement event after accounting failure")
			}
			if err = c.ReplayRecord(record); err != nil {
				if m.Complete {
					return nil, fmt.Errorf("event %d accounting: %w", e.ID, err)
				}
				r.Accounting = append(r.Accounting, fmt.Sprintf("event %d: %v", e.ID, err))
			}
			if record.Kind == "generation_end" {
				generation = record.At
			}
			observation = max(observation, record.At)
		}
		r.Events = append(r.Events, bytes.Clone(bytes.TrimSpace(line)))
	}
	if finalized && m.Events != profile.Decimal(len(r.Events)) {
		return nil, errors.New("raw event count mismatch")
	}
	if m.Storage == "saved" && (!cutoff || cutoffComplete != m.Complete) {
		return nil, errors.New("manifest/final cutoff mismatch")
	}
	complete := finalized && m.Storage == "saved" && cutoff && cutoffComplete && m.Complete
	result := c.ReplayFinish(generation, observation, complete)
	result.Run, _ = probe.ID(m.RunID)
	result.Session, _ = probe.ID(m.SessionID)
	result.Epoch, result.PeerEpoch = 1, 1
	r.Evidence = result
	if clock.totals.Attempts > 0 {
		r.Clock = clock.snapshot(max(observation, clockFinished))
	}
	if clockRetention != "" {
		if r.Clock == nil {
			r.Clock = clock.snapshot(observation)
		}
		r.Clock.Reason = clockRetention
	}
	if complete != result.Complete {
		return nil, errors.New("complete report does not cover frozen observation window")
	}
	r.Summary = EvidenceSummary{1, m.RunID, m.SessionID, m.Endpoint, m.PlanHash, "events.jsonl", profile.Decimal(len(r.Events)), sha, localSnapshot(result), m.Storage}
	if complete {
		r.Status, r.Reason = "completed", "complete local observations; remote evidence remains unknown"
	}
	if finalized {
		b, err = reportFile(dir, "summary.json", &remaining)
		if err != nil {
			return nil, err
		}
		var saved EvidenceSummary
		if err = decodeReportObject(b, &saved); err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(saved, r.Summary) {
			if m.Storage == "saved" {
				return nil, errors.New("saved summary does not match raw replay")
			}
			r.Verification = "mismatch; failed storage retained only a partial raw prefix"
		} else {
			r.Verification = "matched"
		}
	}
	if remote && m.Endpoint == "client" {
		if err = replayTransferred(dir, r, &remaining, registries...); err != nil {
			return nil, err
		}
	}
	if err = ApplyPresentation(r, measure.DefaultEvidencePolicy()); err != nil {
		return nil, err
	}
	return r, nil
}
