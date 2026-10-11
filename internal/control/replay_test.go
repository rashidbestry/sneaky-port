package control

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/worker"
)

// partialReplayFixture creates real restricted files with >2^53 raw times and a recomputable empty partial ledger.
// Synthetic times do not establish timer precision, live clock calibration or native network support.
func partialReplayFixture(t *testing.T, observations ...engine.Record) string {
	t.Helper()
	store, p, ready := evidenceFixture(t)
	p.Calibration.MaxError = 9007199254740993
	h, _ := p.Hash()
	ready.PlanHash = hex.EncodeToString(h[:])
	w, err := newEvidence(store, p, ready, "client")
	if err != nil {
		t.Fatal(err)
	}
	c, err := engine.NewCollector(p, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) == 0 {
		observations = []engine.Record{{Kind: "invalid", At: 9007199254740993, Reason: "comma, quote\" and\nnewline"}}
	}
	for _, record := range observations {
		if err = c.ReplayRecord(record); err != nil {
			t.Fatal(err)
		}
	}
	r := c.ReplayFinish(1, 123, false)
	for _, b := range r.Records {
		if err = w.Append(b); err != nil {
			t.Fatal(err)
		}
	}
	if err = w.Finalize(r); err != nil {
		t.Fatal(err)
	}
	return w.path
}

// TestReplayAccountingFailure reproduces an error saved before live accounting stops, preserving its partial ledger.
func TestReplayAccountingFailure(t *testing.T) {
	store, p, ready := evidenceFixture(t)
	w, err := newEvidence(store, p, ready, "client")
	if err != nil {
		t.Fatal(err)
	}
	c, err := engine.NewCollector(p, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	e := worker.Event{Kind: "schedule_skip", Frame: probe.Frame{Kind: probe.Data, Direction: 1, Lifecycle: 2}, At: 1, Scheduled: 0, Count: 1, Reason: "fixture skip"}
	if err = c.Apply(e); err != nil {
		t.Fatal(err)
	}
	if err = c.Apply(e); err == nil {
		t.Fatal("fixture identity reuse accepted")
	}
	r, finishErr := c.Finish(err, 100)
	if finishErr == nil || r.Complete {
		t.Fatal("accounting failure promoted")
	}
	for _, raw := range r.Records {
		if err = w.Append(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err = w.Finalize(r); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReplayReport(w.path, 4<<20)
	if err != nil || replayed.Verification != "matched" || len(replayed.Accounting) != 1 {
		t.Fatalf("saved accounting failure: %v; %+v", err, replayed)
	}
}

// TestReplayFailedDiskPrefix keeps exact written bytes and excludes the unparseable partial envelope.
func TestReplayFailedDiskPrefix(t *testing.T) {
	store, p, ready := evidenceFixture(t)
	w, err := newEvidence(store, p, ready, "client")
	if err != nil {
		t.Fatal(err)
	}
	w.file = &faultEvidenceFile{File: w.file.(*os.File), partial: true}
	raw, _ := json.Marshal(engine.Record{Kind: "invalid", At: 1})
	if err = w.Append(raw); err != nil {
		t.Fatal(err)
	}
	if err = w.Finalize(engine.Result{ObservationCutoff: 123}); err == nil {
		t.Fatal("fixture disk failure hidden")
	}
	r, err := ReplayReport(w.path, 4<<20)
	if err != nil || r.Status != "partial" || r.TailBytes != 7 || len(r.Events) != 0 || !strings.HasPrefix(r.Verification, "mismatch") {
		t.Fatalf("failed disk prefix: %v; %+v", err, r)
	}
}

// TestReplayLargeRecord checks buffered envelope assembly beyond 4 KiB, including opaque adapter/settings text.
func TestReplayLargeRecord(t *testing.T) {
	dir := partialReplayFixture(t, engine.Record{Kind: "invalid", At: 9007199254740993, Reason: strings.Repeat("a", 4096), Error: strings.Repeat("b", 4096)})
	r, err := ReplayReport(dir, 4<<20)
	if err != nil || r.Verification != "matched" {
		t.Fatalf("large record replay: %v", err)
	}
	var e EvidenceEvent
	if err = json.Unmarshal(r.Events[0], &e); err != nil {
		t.Fatal(err)
	}
	var rec engine.Record
	if err = json.Unmarshal(e.Details, &rec); err != nil || rec.Reason != strings.Repeat("a", 4096) || rec.Error != strings.Repeat("b", 4096) {
		t.Fatal("buffered record corrupted")
	}
}

// TestReplayEnvelopeValidation rebinds local checksums so schema/identity validation must reject altered events.
// Checksums detect consistency only; they never authenticate an operator-supplied report.
func TestReplayEnvelopeValidation(t *testing.T) {
	for _, kind := range []string{"endpoint", "event_id", "epoch", "details", "unknown_kind", "decimal", "after_cutoff", "missing_cutoff"} {
		t.Run(kind, func(t *testing.T) { // Preserve each independent malformed-input case for reproducible validation.
			dir := partialReplayFixture(t)
			path := filepath.Join(dir, "events.jsonl")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "endpoint":
				raw = bytes.Replace(raw, []byte(`"endpoint":"client"`), []byte(`"endpoint":"server"`), 1)
			case "event_id":
				raw = bytes.Replace(raw, []byte(`"event_id":"1"`), []byte(`"event_id":"2"`), 1)
			case "epoch":
				raw = bytes.Replace(raw, []byte(`"clock_epoch_id":1`), []byte(`"clock_epoch_id":2`), 1)
			case "details":
				raw = bytes.Replace(raw, []byte(`"At":"9007199254740993"`), []byte(`"At":"9007199254740994"`), 1)
			case "unknown_kind":
				raw = bytes.Replace(raw, []byte(`"Kind":"invalid"`), []byte(`"Kind":"future"`), 1)
			case "decimal":
				raw = bytes.Replace(raw, []byte(`"time_ns":"9007199254740993"`), []byte(`"time_ns":9007199254740993`), 1)
			case "after_cutoff":
				raw = append(raw, raw[:bytes.IndexByte(raw, '\n')+1]...)
			case "missing_cutoff":
				raw = raw[:bytes.IndexByte(raw, '\n')+1]
			}
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var m EvidenceManifest
			if err = json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			h := sha256.Sum256(raw)
			m.SHA256, m.RawBytes, m.Events = hex.EncodeToString(h[:]), profile.Decimal(len(raw)), profile.Decimal(bytes.Count(raw, []byte{'\n'}))
			b, _ = json.Marshal(m)
			if err = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = ReplayReport(dir, 4<<20); err == nil {
				t.Fatal("invalid bound envelope accepted")
			}
		})
	}
}

// TestReplayExports verifies exact decimal values, units, escaping, partial status and explicit remote unknowns.
func TestReplayExports(t *testing.T) {
	dir := partialReplayFixture(t)
	r, err := ReplayReport(dir, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "partial" || r.Verification != "matched" || r.Summary.Local.Complete {
		t.Fatal("partial replay status lost")
	}
	for _, format := range []string{"json", "jsonl", "csv"} {
		var b bytes.Buffer
		if err = ExportReport(&b, r, format); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "partial") || !strings.Contains(b.String(), "unknown") {
			t.Fatalf("%s lost state", format)
		}
		if !strings.Contains(b.String(), "9007199254740993") {
			t.Fatalf("%s lost decimal precision", format)
		}
		if format == "jsonl" {
			d := json.NewDecoder(&b)
			var header struct {
				Type   string         `json:"record_type"`
				Report ReplayedReport `json:"report"`
			}
			if err = d.Decode(&header); err != nil || header.Type != "local_report" {
				t.Fatal("missing JSONL context")
			}
			for _, raw := range r.Events {
				var e struct {
					Type  string          `json:"record_type"`
					Event json.RawMessage `json:"event"`
				}
				if err = d.Decode(&e); err != nil || !bytes.Equal(e.Event, raw) {
					t.Fatal("JSONL raw values changed")
				}
			}
		}
		if format == "csv" {
			rows, err := csv.NewReader(&b).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			precise, unknown, escaped := false, false, false
			for _, row := range rows[1:] {
				if row[7] == "time_ns" && row[9] == "9007199254740993" && row[8] == "ns" {
					precise = true
				}
				if strings.HasSuffix(row[7], "Received") && row[9] == "" && row[10] == "unknown" && row[11] != "" {
					unknown = true
				}
				if row[7] == "details.Reason" && row[9] == "comma, quote\" and\nnewline" {
					escaped = true
				}
			}
			if !precise || !unknown || !escaped {
				t.Fatal("CSV precision/status/escaping lost")
			}
		}
	}
}

// TestReplayRejectsCorruption checks integrity, schema, identities, explicit bounds and saved derivation mismatch.
func TestReplayRejectsCorruption(t *testing.T) {
	for _, kind := range []string{"hash", "summary", "version", "duplicate_key", "unknown_field", "budget", "tail"} {
		t.Run(kind, func(t *testing.T) { // Every mutation belongs to an isolated fixture report, never user evidence.
			dir := partialReplayFixture(t)
			name := "manifest.json"
			if kind == "hash" || kind == "tail" {
				name = "events.jsonl"
			}
			if kind == "summary" {
				name = "summary.json"
			}
			path := filepath.Join(dir, name)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "hash":
				b = bytes.Replace(b, []byte("9007199254740993"), []byte("9007199254740994"), 1)
			case "tail":
				b = b[:len(b)-1]
			case "summary":
				b = bytes.Replace(b, []byte(`"generation_cutoff_ns":"1"`), []byte(`"generation_cutoff_ns":"2"`), 1)
			case "version":
				b = bytes.Replace(b, []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1)
			case "duplicate_key":
				b = append([]byte(`{"schema_version":1,`), b[1:]...)
			case "unknown_field":
				b = append([]byte(`{"unexpected":true,`), b[1:]...)
			}
			if err = os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			budget := uint64(4 << 20)
			if kind == "budget" {
				budget = 100
			}
			if _, err = ReplayReport(dir, budget); err == nil {
				t.Fatal("corrupt/unbounded replay accepted")
			}
		})
	}
}

// TestReplayUnfinalizedPrefix excludes a torn trailing write and cannot promote an initial manifest.
func TestReplayUnfinalizedPrefix(t *testing.T) {
	dir := partialReplayFixture(t)
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m EvidenceManifest
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	m.State, m.Storage, m.SummaryFile, m.SHA256, m.Events, m.RawBytes = "incomplete", "not_finalized", "", "", 0, 0
	b, _ = json.Marshal(m)
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw[:bytes.IndexByte(raw, '\n')+1], []byte(`{"schema_version":`)...)
	if err = os.WriteFile(filepath.Join(dir, "events.jsonl"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := ReplayReport(dir, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "partial" || r.Summary.Local.Complete || len(r.Events) != 1 || !strings.HasPrefix(r.Verification, "unavailable") {
		t.Fatal("unfinalized prefix promoted")
	}
}
