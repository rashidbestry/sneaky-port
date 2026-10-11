package control

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// afterRunEvidence anchors interruption fixtures to first saved measurement evidence, excluding setup/fsync delay.
// Each caller uses a fresh isolated client store; cancellation joins the polling/timer owner.
func afterRunEvidence(t *testing.T, root string, delay time.Duration, action func()) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { // Poll only this fresh fixture store; never use startup wall time as a measurement-phase anchor.
		defer close(done)
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			paths, _ := filepath.Glob(filepath.Join(root, "reports", "*-client", "events.jsonl"))
			started := false
			for _, path := range paths {
				if info, err := os.Stat(path); err == nil && info.Size() > 0 {
					started = true
					break
				}
			}
			if started {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			action()
		}
	}()
	return func() { // Stop and join the fixture trigger before releasing its temporary endpoint stores.
		cancel()
		<-done
	}
}

// faultEvidenceFile injects a partial disk write or a blocked filesystem call while retaining a real owned file.
type faultEvidenceFile struct {
	*os.File
	partial          bool
	entered, release chan struct{}
}

// Write exposes accepted prefix bytes or waits for an explicit fixture release before the actual file write.
func (f *faultEvidenceFile) Write(b []byte) (int, error) {
	if f.entered != nil {
		close(f.entered)
		<-f.release
	}
	if f.partial {
		n, err := f.File.Write(b[:7])
		return n, errors.Join(err, errors.New("injected partial disk write"))
	}
	return f.File.Write(b)
}

// TestEvidenceFileExhaustion retains the exact admitted prefix and a reserved final cutoff after raw budget exhaustion.
func TestEvidenceFileExhaustion(t *testing.T) {
	store, p, ready := evidenceFixture(t)
	w, err := newEvidence(store, p, ready, "client")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(engine.Record{Kind: "invalid", At: 9007199254740993})
	b, _ := w.event("invalid", 9007199254740993, raw)
	w.rawLimit = uint64(len(b) + 1)
	if err = w.Append(raw); err != nil {
		t.Fatal(err)
	}
	if err = w.Append(raw); err == nil || !strings.Contains(err.Error(), "file budget") {
		t.Fatal("envelope file limit ignored")
	}
	if err = w.Finalize(engine.Result{ObservationCutoff: 9007199254740994}); err != nil {
		t.Fatal(err)
	}
	m, s, events := readEvidence(t, w.path)
	if m.Complete || s.Local.Complete || len(events) != 2 || !bytes.Equal(events[0].Details, raw) {
		t.Fatal("file exhaustion dropped admitted evidence or changed completeness")
	}
}

// TestEvidenceDiskBoundaries verifies partial-byte retention and a bounded join on an uninterruptible disk call.
// A timed-out writer owns its file until the call returns; it never publishes a complete manifest afterward.
func TestEvidenceDiskBoundaries(t *testing.T) {
	for _, kind := range []string{"partial", "blocked"} {
		t.Run(kind, func(t *testing.T) { // Only fixture-created files are wrapped or blocked.
			store, p, ready := evidenceFixture(t)
			w, err := newEvidence(store, p, ready, "client")
			if err != nil {
				t.Fatal(err)
			}
			file := &faultEvidenceFile{File: w.file.(*os.File), partial: kind == "partial"}
			if kind == "blocked" {
				file.entered, file.release = make(chan struct{}), make(chan struct{})
			}
			w.file = file
			w.timeout = 20 * time.Millisecond
			raw, _ := json.Marshal(engine.Record{Kind: "invalid", At: 1})
			if err = w.Append(raw); err != nil {
				t.Fatal(err)
			}
			if kind == "blocked" {
				<-file.entered
			}
			if err = w.Finalize(engine.Result{Complete: true, ObservationCutoff: 123}); err == nil {
				t.Fatal("disk failure hidden")
			}
			if kind == "blocked" {
				close(file.release)
				<-w.done
			}
			b, _ := os.ReadFile(filepath.Join(w.path, "manifest.json"))
			var m EvidenceManifest
			json.Unmarshal(b, &m)
			if m.Complete {
				t.Fatal("fault marked complete")
			}
			if kind == "partial" && (m.RawBytes != 7 || m.Events != 0 || m.State != "partial") {
				t.Fatal("partial raw write bytes concealed")
			}
			if kind == "blocked" && m.State != "incomplete" {
				t.Fatal("join timeout finalized files")
			}
		})
	}
}

// assertPersistedRun checks both owned endpoints after real runs, including asynchronously closed control sessions.
// It compares the client's exact observation sequence/cutoffs without claiming receiver reconciliation.
func assertPersistedRun(t *testing.T, rig *testRig, result RunResult, complete bool) {
	t.Helper()
	m, s, events := readEvidence(t, result.ReportPath)
	if m.Complete != complete || s.Local.Complete != complete || s.Local.Generation != result.Local.Generation || s.Local.Observation != result.Local.Observation || len(events) != len(result.Evidence.Records)+1 {
		t.Fatal("persisted client state differs from joined engine")
	}
	for i, raw := range result.Evidence.Records {
		if !bytes.Equal(raw, events[i].Details) {
			t.Fatal("persisted event changed original evidence")
		}
	}
	dir := filepath.Join(rig.server.Root, "reports", result.Readiness.RunID+"-server")
	end := time.Now().Add(2 * time.Second)
	for {
		b, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
		var server EvidenceManifest
		json.Unmarshal(b, &server)
		if server.State == "completed" || server.State == "partial" {
			break
		}
		if time.Now().After(end) {
			t.Fatal("server local evidence not finalized")
		}
		time.Sleep(5 * time.Millisecond)
	}
	server, _, _ := readEvidence(t, dir)
	if server.RunID != m.RunID || server.SessionID != m.SessionID || server.PlanHash != m.PlanHash || complete && !server.Complete {
		t.Fatal("server evidence identity/completeness mismatch")
	}
	for _, path := range []string{result.ReportPath, dir} {
		replayed, err := ReplayReport(path, 4<<20)
		if err != nil || replayed.Verification != "matched" {
			t.Fatalf("local replay %s: %v; report=%+v", path, err, replayed)
		}
	}
}

// TestFiniteStoragePrestart rejects unusable client/server stores before either starts measurement traffic.
func TestFiniteStoragePrestart(t *testing.T) {
	for _, endpoint := range []string{"client", "server"} {
		t.Run(endpoint, func(t *testing.T) { // A reports-path collision represents an owned storage setup failure.
			r := finiteRig(t, "udp")
			root := r.client.Root
			if endpoint == "server" {
				root = r.server.Root
			}
			if err := os.WriteFile(filepath.Join(root, "reports"), []byte("preserve"), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := Run(context.Background(), r.client, r.signer, r.plan)
			if err == nil || result.Readiness.MeasurementStarted || result.Local != nil {
				t.Fatal("measurement started without local evidence files")
			}
			b, _ := os.ReadFile(filepath.Join(root, "reports"))
			if string(b) != "preserve" {
				t.Fatal("existing storage destination modified")
			}
			if endpoint == "server" {
				m, _, _ := readEvidence(t, result.ReportPath)
				if m.Complete {
					t.Fatal("failed start became complete")
				}
			}
		})
	}
}

// evidenceFixture creates an isolated restricted store and frozen nonsecret identities, without sockets.
func evidenceFixture(t *testing.T) (*Store, profile.Plan, ReadyResult) {
	t.Helper()
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := testutil.Plan()
	h, _ := p.Hash()
	return store, p, ReadyResult{Version: 1, RunID: strings.Repeat("ab", 16), SessionID: strings.Repeat("cd", 16), PlanHash: hex.EncodeToString(h[:])}
}

// readEvidence checks the final raw byte/hash binding and exact consecutive decimal event IDs.
// This fixture inspects persisted files; it does not implement product replay or remote authentication.
func readEvidence(t *testing.T, dir string) (EvidenceManifest, EvidenceSummary, []EvidenceEvent) {
	t.Helper()
	var m EvidenceManifest
	var s EvidenceSummary
	for name, target := range map[string]any{"manifest.json": &m, "summary.json": &s} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, target); err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"client_key", "server_key", "challenge", "PRIVATE KEY", "KeyRef", "SendAuth", "ReceiveAuth"} {
			if bytes.Contains(b, []byte(secret)) {
				t.Fatalf("secret field in %s", name)
			}
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(b)
	if uint64(m.RawBytes) != uint64(len(b)) || m.SHA256 != hex.EncodeToString(digest[:]) || s.SHA256 != m.SHA256 {
		t.Fatal("raw file checksum/byte binding mismatch")
	}
	scan := bufio.NewScanner(bytes.NewReader(b))
	scan.Buffer(make([]byte, 4096), 1<<20)
	events := []EvidenceEvent{}
	for scan.Scan() {
		var e EvidenceEvent
		if err := json.Unmarshal(scan.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if e.Version != 1 || e.ID != profile.Decimal(len(events)+1) || e.RunID != m.RunID || e.SessionID != m.SessionID || e.Endpoint != m.Endpoint || e.Epoch != 1 {
			t.Fatal("event identity mismatch")
		}
		events = append(events, e)
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if m.Events != profile.Decimal(len(events)) || s.Events != m.Events {
		t.Fatal("event count mismatch")
	}
	var final struct{ Generation, Observation profile.Decimal }
	last := events[len(events)-1]
	if last.Type != "final_cutoff" || last.At != s.Local.Observation {
		t.Fatal("final observation cutoff mismatch")
	}
	var cut map[string]json.RawMessage
	if err := json.Unmarshal(last.Details, &cut); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(cut["generation_cutoff_ns"], &final.Generation)
	json.Unmarshal(cut["observation_cutoff_ns"], &final.Observation)
	if final.Generation != s.Local.Generation || final.Observation != s.Local.Observation {
		t.Fatal("summary/raw cutoff mismatch")
	}
	return m, s, events
}

// TestEvidencePersistence preserves original raw records, >2^53 offsets and public settings for complete/partial endpoints.
func TestEvidencePersistence(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(stringTestBool(complete), func(t *testing.T) { // Endpoint completeness is independent of remote delivery evidence.
			store, p, ready := evidenceFixture(t)
			w, err := newEvidence(store, p, ready, "client")
			if err != nil {
				t.Fatal(err)
			}
			initial, err := os.ReadFile(filepath.Join(w.path, "manifest.json"))
			if err != nil || !bytes.Contains(initial, []byte(`"state":"incomplete"`)) {
				t.Fatal("initial incomplete manifest absent")
			}
			r := engine.Record{Kind: "arrival", At: 9007199254740993, Attempt: 9007199254740991, Direction: 2, Epoch: 1, PeerEpoch: 1}
			raw, _ := json.Marshal(r)
			if err = w.Append(raw); err != nil {
				t.Fatal(err)
			}
			result := engine.Result{Complete: complete, GenerationCutoff: 9007199254740994, ObservationCutoff: 9007199254740995}
			if err = w.Finalize(result); err != nil {
				t.Fatal(err)
			}
			m, s, events := readEvidence(t, w.path)
			if m.Complete != complete || s.Local.Complete != complete || !bytes.Equal(events[0].Details, raw) || events[0].At != r.At || m.Remote != "unknown; not transferred or reconciled" {
				t.Fatal("evidence or unknown status changed")
			}
			if _, err = newEvidence(store, p, ready, "client"); err == nil {
				t.Fatal("existing run overwritten")
			}
			if err = w.Append(raw); err == nil {
				t.Fatal("append after finalization")
			}
			if err = w.Finalize(result); err == nil {
				t.Fatal("duplicate finalization accepted")
			}
			files, _ := os.ReadDir(w.path)
			var total int64
			for _, file := range files {
				info, _ := file.Info()
				total += info.Size()
			}
			if uint64(total) > uint64(p.Limits.FileBytes) {
				t.Fatal("file budget exceeded")
			}
		})
	}
}

// stringTestBool supplies stable case names for endpoint completeness without product formatting behavior.
func stringTestBool(value bool) string {
	if value {
		return "complete"
	}
	return "partial"
}

// TestEvidenceAdmissionBounds proves active-inclusive count/byte limits and exact envelope-byte file charging.
// No disk goroutine is started, making saturation deterministic instead of depending on disk speed.
func TestEvidenceAdmissionBounds(t *testing.T) {
	raw, _ := json.Marshal(engine.Record{Kind: "invalid", At: 1})
	for _, kind := range []string{"messages", "bytes", "file"} {
		t.Run(kind, func(t *testing.T) { // Each independent cap must reject rather than block or discard silently.
			w := &evidenceWriter{queue: make(chan []byte, 1), queueBytes: 1 << 20, rawLimit: 1 << 20, failed: make(chan struct{}), manifest: EvidenceManifest{RunID: strings.Repeat("a", 32), SessionID: strings.Repeat("b", 32), Endpoint: "client"}}
			if kind == "bytes" {
				w.queueBytes = 1
			}
			if kind == "file" {
				w.rawLimit = 1
			}
			if kind == "messages" {
				if err := w.Append(raw); err != nil {
					t.Fatal(err)
				}
				b := <-w.queue // Simulate an active write: popping the channel must not release its budget.
				if w.charged != uint64(len(b)) || w.queuedBytes != uint64(len(b)) || w.pending != 1 {
					t.Fatal("JSONL bytes not charged")
				}
			}
			if err := w.Append(raw); err == nil {
				t.Fatal("evidence cap ignored")
			}
		})
	}
}

// TestEvidenceStorageFailures retains incomplete/partial output after disk/publication faults and invalid setup budgets.
func TestEvidenceStorageFailures(t *testing.T) {
	for _, kind := range []string{"disk", "summary", "manifest", "budget", "identity", "symlink"} {
		t.Run(kind, func(t *testing.T) { // Fault injection touches only this fixture's application-owned files.
			store, p, ready := evidenceFixture(t)
			if kind == "budget" {
				p.Limits.FileBytes = p.Limits.EventBytes
				h, _ := p.Hash()
				ready.PlanHash = hex.EncodeToString(h[:])
			}
			if kind == "identity" {
				ready.RunID = "../escape"
			}
			if kind == "symlink" {
				if err := os.Symlink(t.TempDir(), filepath.Join(store.Root, "reports")); err != nil {
					t.Skipf("symlink fixture unavailable: %v", err)
				}
			}
			w, err := newEvidence(store, p, ready, "client")
			if kind == "budget" || kind == "identity" || kind == "symlink" {
				if err == nil {
					w.Finalize(engine.Result{})
					t.Fatal("unsafe setup accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if kind == "disk" {
				w.file.Close()
				raw, _ := json.Marshal(engine.Record{Kind: "invalid", At: 1})
				if err = w.Append(raw); err != nil {
					t.Fatal(err)
				}
				select {
				case <-w.Failed():
				case <-time.After(time.Second):
					t.Fatal("disk failure not exposed")
				}
			} else {
				name := "summary.json.pending"
				if kind == "manifest" {
					name = "manifest.json.pending"
				}
				if err = os.WriteFile(filepath.Join(w.path, name), []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err = w.Finalize(engine.Result{Complete: true, ObservationCutoff: 123}); err == nil {
				t.Fatal("storage failure reported complete")
			}
			b, _ := os.ReadFile(filepath.Join(w.path, "manifest.json"))
			var m EvidenceManifest
			json.Unmarshal(b, &m)
			if m.Complete {
				t.Fatal("fault published complete manifest")
			}
			if kind == "disk" {
				var s EvidenceSummary
				b, _ = os.ReadFile(filepath.Join(w.path, "summary.json"))
				json.Unmarshal(b, &s)
				if s.Local == nil || s.Local.Complete || s.Local.Observation != 123 || s.Storage != "failed; raw evidence incomplete" {
					t.Fatal("disk failure lost cutoff/status")
				}
			}
		})
	}
}
