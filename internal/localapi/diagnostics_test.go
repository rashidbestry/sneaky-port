package localapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"sneaky-port/internal/control"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// decimalPointer makes independent exact host-local timestamps for synthetic cutoff fixtures.
func decimalPointer(n profile.Decimal) *profile.Decimal { return &n }

// diagnosticFixture holds synthetic raw ledger outcomes, never a claim of measured clock precision.
func diagnosticFixture() *control.ReplayedReport {
	p := testutil.Plan()
	p.Warmup = 10
	p.Phases = append(p.Phases, p.Phases[0])
	p.Phases[1].ID = "quiet"
	return &control.ReplayedReport{Manifest: control.EvidenceManifest{Plan: p, Endpoint: "client"}, Status: "partial", Integrity: "synthetic",
		Evidence: engine.Result{ObservationCutoff: 1000, Entries: []measure.Entry{
			{Identity: measure.Identity{Lifecycle: 1}, Scheduled: 0, Attempt: decimalPointer(1), Receipt: decimalPointer(9)},
			{Identity: measure.Identity{Lifecycle: 2, Sequence: 1}, Scheduled: 10, Deadline: 50, Attempt: decimalPointer(20), Receipt: decimalPointer(100), State: "written", WriteCompleted: decimalPointer(21)},
			{Identity: measure.Identity{Lifecycle: 2, Sequence: 2}, Scheduled: 110, Deadline: 300, Attempt: decimalPointer(120), Receipt: decimalPointer(200), State: "written", WriteCompleted: decimalPointer(121)},
			{Identity: measure.Identity{Lifecycle: 2, Sequence: 3}, Scheduled: 210, Deadline: 400, Attempt: decimalPointer(220), Receipt: decimalPointer(1000)},
			{Identity: measure.Identity{Lifecycle: 2, Sequence: 4}, Scheduled: 310, Deadline: 500, State: "skipped_local", Reason: "queue saturation"},
			{Identity: measure.Identity{Lifecycle: 2, Sequence: 5}, Scheduled: 1000, Deadline: 1100},
		}}}
}

// TestDiagnosticCutoffsScopesAndMissing retains late samples, local skips and strict cutoff equality.
func TestDiagnosticCutoffsScopesAndMissing(t *testing.T) {
	r := diagnosticFixture()
	v, err := buildDiagnostic(r, "main")
	if err != nil || v.Complete || len(v.Time) != 4 || len(v.RTT) != 2 || v.Time[2].Missing != 1 || v.Time[2].RTT.Min != nil || v.Time[3].Local != 1 {
		t.Fatalf("cutoff/local populations: %#v %v", v, err)
	}
	if v.Summary["RTTSamples"] != "2" || *v.Summary["VariationP95"].(*string) != "0" || v.Summary["Late"] != "1" || v.Summary["TransportPacketLoss"] != nil {
		t.Fatal("exact summary or unknown evidence lost", v.Summary)
	}
	if *v.Time[0].RTT.Min != 80 || *v.Time[0].Scheduled.Min != 90 || v.RTT[1].Last != 2 {
		t.Fatal("distinct duration/rank populations lost")
	}
	warmup, err := buildDiagnostic(r, "warmup")
	if err != nil || warmup.Summary["RTTSamples"] != "1" {
		t.Fatal("warmup conflated", err)
	}
	quiet, err := buildDiagnostic(r, "1")
	if err != nil || quiet.Summary["Planned"] != "0" || len(quiet.Time) != 0 || len(quiet.RTT) != 0 {
		t.Fatal("quiet invented samples", err)
	}
	for _, scope := range []string{"01", "-1", "2", "unknown"} {
		if _, err := buildDiagnostic(r, scope); err == nil {
			t.Fatal("invalid scope", scope)
		}
	}
}

// TestDiagnosticBoundedExtremaAndExactQuantiles covers decimation, ties and values beyond JavaScript precision.
func TestDiagnosticBoundedExtremaAndExactQuantiles(t *testing.T) {
	r := diagnosticFixture()
	r.Evidence.Entries = nil
	r.Evidence.ObservationCutoff = 9007199255740993
	r.Manifest.Plan.Phases = make([]profile.Phase, 64)
	for i := range r.Manifest.Plan.Phases {
		r.Manifest.Plan.Phases[i] = testutil.Plan().Phases[0]
		r.Manifest.Plan.Phases[i].ID = fmt.Sprintf("p%d", i)
	}
	values := []uint64{}
	for i := 0; i < 4096; i++ {
		at := profile.Decimal(i * 10)
		elapsed := profile.Decimal(9007199254740993 + uint64(i%137))
		r.Evidence.Entries = append(r.Evidence.Entries, measure.Entry{Identity: measure.Identity{Lifecycle: 2, Phase: uint32(i / 64), Sequence: profile.Decimal(i)}, Scheduled: at, Attempt: decimalPointer(at + 1), Receipt: decimalPointer(at + 1 + elapsed), Deadline: r.Evidence.ObservationCutoff})
		values = append(values, uint64(elapsed))
	}
	v, err := buildDiagnostic(r, "main")
	if err != nil || len(v.Time) > 128 || len(v.RTT) > 128 {
		t.Fatal("render cap", err, len(v.Time), len(v.RTT))
	}
	var count profile.Decimal
	for _, group := range v.Time {
		count += group.RTT.Count
		if uint64(group.Start)/640 != uint64(group.End)/640 {
			t.Fatal("time range crosses phase")
		}
	}
	if count != 4096 || v.Summary["RTTP99"] != *measure.Percentile(values, 99) || *v.RTT[0].Min != 9007199254740993 {
		t.Fatal("population/quantile/extrema lost")
	}
	count = 0
	for _, group := range v.RTT {
		count += group.Count
		if group.Last-group.First+1 != group.Count {
			t.Fatal("rank count mismatch")
		}
	}
	if count != 4096 || v.RTT[len(v.RTT)-1].Last != 4096 {
		t.Fatal("ECDF population lost")
	}
	b, _ := json.Marshal(v)
	if !strings.Contains(string(b), "\"9007199254740993\"") {
		t.Fatal("exact decimals lost")
	}
	r.Manifest.Plan.Phases = append(r.Manifest.Plan.Phases, r.Manifest.Plan.Phases[0])
	if _, err := buildDiagnostic(r, "main"); err == nil {
		t.Fatal("phase cap silently truncates")
	}
}

// importDiagnosticFixture publishes an empty incomplete report through the actual offline consistency validator.
func importDiagnosticFixture(t *testing.T, s *Server) {
	t.Helper()
	hash, _ := s.plan.Hash()
	m := control.EvidenceManifest{Version: 1, RunID: strings.Repeat("1", 32), SessionID: strings.Repeat("2", 32), Endpoint: "client", Plan: s.plan, PlanHash: hex.EncodeToString(hash[:]), AdapterContract: s.plan.Contract, State: "incomplete", RawFile: "events.jsonl", Storage: "not_finalized"}
	b, _ := json.Marshal(m)
	if _, err := s.store.ImportReport("diagnostic", map[string][]byte{"manifest.json": b, "events.jsonl": {}}); err != nil {
		t.Fatal(err)
	}
}

// TestDiagnosticProtectedReplayAndPreflight uses actual routing/storage and denies unauthenticated or excessive work.
func TestDiagnosticProtectedReplayAndPreflight(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	importDiagnosticFixture(t, s)
	for _, path := range []string{"/api/v1/reports/diagnostic/diagnostics", "/api/v1/preflight"} {
		method := "GET"
		if strings.HasSuffix(path, "preflight") {
			method = "POST"
		}
		r := request(s, method, path, "guard", "")
		r.Header.Del("Authorization")
		invoke(t, s, r, 401)
		for _, guard := range []string{"host", "origin", "peer"} {
			r := request(s, method, path, "guard", "")
			switch guard {
			case "host":
				r.Host = "foreign"
			case "origin":
				r.Header.Set("Origin", "null")
			case "peer":
				r.RemoteAddr = "192.0.2.1:1234"
			}
			invoke(t, s, r, 403)
		}
	}
	result := invoke(t, s, request(s, "GET", "/api/v1/reports/diagnostic/diagnostics?scope=main&endpoint=local", "read", ""), 200)
	b, _ := json.Marshal(result.Result)
	if !strings.Contains(string(b), "\"complete\":false") || strings.Contains(string(b), s.store.Root) {
		t.Fatal("partial/secrecy lost")
	}
	for _, query := range []string{"scope=01", "scope=main&scope=warmup", "path=secret", "endpoint=unknown", "endpoint=server"} {
		invoke(t, s, request(s, "GET", "/api/v1/reports/diagnostic/diagnostics?"+query, "bad", ""), map[bool]int{true: 400, false: 422}[strings.Contains(query, "path=") || strings.Contains(query, "scope=main&") || strings.Contains(query, "unknown")])
	}
	s.savedMu.Lock()
	invoke(t, s, request(s, "GET", "/api/v1/reports/diagnostic/diagnostics", "busy", ""), 503)
	s.savedMu.Unlock()
	for _, duration := range []string{"0", "1000000001", "4097"} {
		savedRequest(t, s, "POST", "/api/v1/preflight", "bounds", map[string]any{"duration_ns": duration, "interval_ns": "1"}, 400)
	}
	out := savedRequest(t, s, "POST", "/api/v1/preflight", "timer", map[string]any{"duration_ns": "1000000", "interval_ns": "100000"}, 200)
	b, _ = json.Marshal(out.Result)
	if !strings.Contains(string(b), "accuracy bounds unverified") || !strings.Contains(string(b), "\"clock_details\"") || !strings.Contains(string(b), "\"independently_verified\":false") || !strings.Contains(string(b), "lag_p99_ns") {
		t.Fatal("preflight clock limitations absent")
	}
	r := request(s, "POST", "/api/v1/preflight", "cancel", `{"schema_version":1,"request_id":"cancel","duration_ns":"1000000000","interval_ns":"1000000"}`)
	ctx, cancel := context.WithCancel(r.Context())
	cancel()
	w := httptest.NewRecorder()
	s.handle(w, r.WithContext(ctx))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "interrupted") {
		t.Fatal("preflight cancellation ignored")
	}
}
