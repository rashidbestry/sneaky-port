package control

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// comparisonFixture holds deterministic already-replayed populations; it makes no live-path claim.
func comparisonFixture(t *testing.T, id string, misses profile.Decimal, tail string) ComparisonTrial {
	t.Helper()
	p := testutil.Plan()
	p.MatchedMetadata["environment"] = "synthetic same owned server/path/workload"
	s := measure.Summary{Planned: 3, DeadlineMisses: misses, RTTP95: &tail, RTTP99: &tail, RTTSamples: 2, RTTVariation: []profile.Decimal{0}, Skipped: misses}
	r := &ReplayedReport{Status: "completed", Manifest: EvidenceManifest{Plan: p, Endpoint: "client", OS: "fixture", Arch: "amd64"}, Evidence: engine.Result{Complete: true, ObservationCutoff: 100, Entries: []measure.Entry{}}, Summary: EvidenceSummary{Local: &LocalRun{Observation: 100, Summary: s, Warmup: measure.Summary{}, Phases: []measure.Summary{s}}}}
	out, err := comparisonTrial(id, r, ComparisonRequest{Endpoint: "local", Scope: "main"})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestComparisonExactRepeatedOutcomes verifies >2^53 signed deltas, exact fractions and conflicting repeats.
func TestComparisonExactRepeatedOutcomes(t *testing.T) {
	q := ComparisonRequest{Pairs: []TrialPair{{"a1", "b1"}, {"a2", "b2"}}, Axes: []string{}, Endpoint: "local", Scope: "main"}
	trials := []ComparisonTrial{comparisonFixture(t, "a1", 1, "9007199254740993"), comparisonFixture(t, "b1", 2, "9007199254740994"), comparisonFixture(t, "a2", 1, "9007199254740993"), comparisonFixture(t, "b2", 2, "9007199254740994")}
	out := compareTrials(q, trials)
	if *out.Pairs[0].Metrics[9].Delta != "1" || *out.Pairs[0].Metrics[1].Delta != "1/3" || out.Repeated[9].State != "descriptive" || out.Repeated[2].State != "tie" || out.Repeated[16].State != "inconclusive" || out.Repeated[16].Min != nil {
		t.Fatal("exact/tie/unknown lost", out.Repeated)
	}
	trials[3].Metrics[9] = trials[0].Metrics[9]
	v := "9007199254740992"
	trials[3].Metrics[9] = &v
	out = compareTrials(q, trials)
	if out.Repeated[9].State != "inconclusive" || *out.Repeated[9].Min != "-1" || *out.Repeated[9].Max != "1" {
		t.Fatal("conflicting trials manufactured conclusion")
	}
	q.Pairs = q.Pairs[:1]
	out = compareTrials(q, trials[:2])
	if out.Repeated[9].State != "inconclusive" || out.Repeated[9].Trials != 1 {
		t.Fatal("single trial became repeated evidence")
	}
	a, b := [](*string){trials[0].Metrics[9]}, [](*string){nil}
	if metricDeltas(a, b, true)[0].Delta != nil {
		t.Fatal("null became zero")
	}
}

// TestComparisonCompatibilityAxesAndFailures checks that only explicitly predeclared axes can differ.
func TestComparisonCompatibilityAxesAndFailures(t *testing.T) {
	for _, axis := range []string{"adapter", "port", "client_dscp", "server_dscp"} {
		t.Run(axis, func(t *testing.T) { // Each axis permits only its declared setting changes.
			a, b := comparisonFixture(t, "a", 1, "80"), comparisonFixture(t, "b", 1, "80")
			a.Manifest.Plan.ComparisonAxes, b.Manifest.Plan.ComparisonAxes = []string{axis}, []string{axis}
			switch axis {
			case "adapter":
				b.Manifest.Plan.Adapter = "tcp"
				b.Manifest.Plan.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
			case "port":
				b.Manifest.Plan.DestinationPort++
				b.Manifest.Plan.SourcePort = "32001"
			case "client_dscp":
				b.Manifest.Plan.Phases[0].Client.DSCP = 46
			case "server_dscp":
				b.Manifest.Plan.Phases[0].Server.DSCP = 46
			}
			before, _ := json.Marshal(a.Manifest.Plan)
			if reasons := comparisonReasons(a, b, []string{axis}); len(reasons) > 0 {
				t.Fatal(reasons)
			}
			after, _ := json.Marshal(a.Manifest.Plan)
			if string(before) != string(after) {
				t.Fatal("normalized matching mutated plan")
			}
			if len(comparisonReasons(a, b, []string{})) == 0 {
				t.Fatal("undeclared variation accepted")
			}
			b.Manifest.Plan.Phases[0].Client.ContentBytes++
			if len(comparisonReasons(a, b, []string{axis})) == 0 {
				t.Fatal("axis concealed workload change")
			}
		})
	}
	for _, fault := range []string{"deadline", "resource", "clock", "environment", "server", "phase", "platform", "options", "partial", "missing_environment", "missing_server"} {
		t.Run(fault, func(t *testing.T) { // Incompatible or incomplete trials retain values and suppress deltas.
			a, b := comparisonFixture(t, "a", 1, "80"), comparisonFixture(t, "b", 2, "90")
			switch fault {
			case "deadline":
				b.Manifest.Plan.Deadline++
			case "resource":
				b.Manifest.Plan.Limits.MaxLag++
			case "clock":
				b.Manifest.Plan.Calibration.Expiry++
			case "environment":
				b.Manifest.Plan.MatchedMetadata["environment"] = "different"
			case "server":
				b.Manifest.Plan.ServerAddress = "192.0.2.1"
			case "phase":
				b.Manifest.Plan.Phases[0].Duration++
			case "platform":
				b.Manifest.OS = "different"
			case "options":
				b.Manifest.Plan.Options["other"] = json.RawMessage("true")
			case "partial":
				b.Complete = false
			case "missing_environment":
				a.Manifest.Plan.MatchedMetadata, b.Manifest.Plan.MatchedMetadata = map[string]string{}, map[string]string{}
			case "missing_server":
				b.Complete = false
				b.Status = "unavailable"
				b.Metrics = nil
			}
			out := compareTrials(ComparisonRequest{Pairs: []TrialPair{{"a", "b"}}, Axes: []string{}}, []ComparisonTrial{a, b})
			if out.Pairs[0].State != "incompatible" && out.Pairs[0].State != "inconclusive" || out.Pairs[0].Metrics[0].Delta != nil {
				t.Fatal("failure promoted", out.Pairs[0].State)
			}
		})
	}
}

// TestComparisonPhaseAndArmConsistency prevents aggregate ties hiding failures or changed repeated setups.
func TestComparisonPhaseAndArmConsistency(t *testing.T) {
	q := ComparisonRequest{Pairs: []TrialPair{{"a1", "b1"}, {"a2", "b2"}}, Axes: []string{"port"}}
	trials := []ComparisonTrial{}
	for _, id := range []string{"a1", "b1", "a2", "b2"} {
		v := comparisonFixture(t, id, 1, "80")
		v.Manifest.Plan.ComparisonAxes = q.Axes
		trials = append(trials, v)
	}
	x := "2"
	trials[1].PhaseMetrics[0] = append([]*string{}, trials[1].PhaseMetrics[0]...)
	trials[1].PhaseMetrics[0][0] = &x
	out := compareTrials(q, trials)
	if out.Pairs[0].Metrics[0].State != "tie" || out.Pairs[0].Phases[0][0].State != "descriptive" || out.Pairs[0].State == "tie" {
		t.Fatal("phase concealed")
	}
	trials[3].Manifest.Plan.DestinationPort++
	out = compareTrials(q, trials)
	if out.Pairs[1].State != "incompatible" || out.Repeated[0].State != "inconclusive" {
		t.Fatal("changing repeated configuration accepted")
	}
}

// savedComparisonFixture publishes real finalized synthetic event files through the shared collector/writer.
func savedComparisonFixture(t *testing.T, store *Store, n int) string {
	t.Helper()
	p := testutil.Plan()
	p.MatchedMetadata["environment"] = "synthetic deterministic fixture"
	h, _ := p.Hash()
	ready := ReadyResult{RunID: fmt.Sprintf("%032x", n), SessionID: strings.Repeat("ab", 16), PlanHash: hex.EncodeToString(h[:])}
	w, err := newEvidence(store, p, ready, "client")
	if err != nil {
		t.Fatal(err)
	}
	c, err := engine.NewCollector(p, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	r := c.ReplayFinish(1000000, 2000000, true)
	for _, raw := range r.Records {
		if err := w.Append(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finalize(r); err != nil {
		t.Fatal(err)
	}
	return filepath.Base(w.path)
}

// TestSavedComparisonReplayBudgetCancellationAndDuplicates exercises actual immutable file admission.
func TestSavedComparisonReplayBudgetCancellationAndDuplicates(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	q := ComparisonRequest{Axes: []string{}, Endpoint: "local", Scope: "main"}
	for n := 1; n <= 4; n += 2 {
		q.Pairs = append(q.Pairs, TrialPair{savedComparisonFixture(t, store, n), savedComparisonFixture(t, store, n+1)})
	}
	out, err := store.CompareSavedReports(context.Background(), q)
	if err != nil || out.Pairs[0].State != "tie" || out.Repeated[0].State != "tie" || out.Repeated[9].State != "inconclusive" {
		t.Fatalf("finalized replay: %v %+v", err, out)
	}
	if out.Repeated[1].Min != nil {
		t.Fatal("zero planned denominator became zero fraction")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.CompareSavedReports(ctx, q); err != context.Canceled {
		t.Fatal("cancellation ignored", err)
	}
	root, err := store.ownedDir("reports", false)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, name := range []string{"manifest.json", "events.jsonl", "summary.json"} {
		files[name], err = os.ReadFile(filepath.Join(root, q.Pairs[0].A, name))
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.ImportReport("alias", files); err != nil {
		t.Fatal(err)
	}
	duplicate := q
	duplicate.Pairs = []TrialPair{{q.Pairs[0].A, "alias"}}
	if _, err := store.CompareSavedReports(context.Background(), duplicate); err == nil {
		t.Fatal("import alias counted as repetition")
	}
	// Individually valid sub-MiB manifests exceed the single aggregate 8 MiB budget across sixteen runs.
	large := q
	large.Pairs = nil
	for n := 1; n <= 16; n++ {
		id := fmt.Sprintf("large%d", n)
		p := testutil.Plan()
		p.MatchedMetadata["environment"] = strings.Repeat("x", 530<<10)
		h, _ := p.Hash()
		m := EvidenceManifest{Version: 1, RunID: fmt.Sprintf("%032x", n+20), SessionID: strings.Repeat("cd", 16), PlanHash: hex.EncodeToString(h[:]), Plan: p, Endpoint: "client", AdapterContract: 1, State: "incomplete", RawFile: "events.jsonl", Storage: "not_finalized"}
		b, _ := json.Marshal(m)
		if _, err := store.ImportReport(id, map[string][]byte{"manifest.json": b, "events.jsonl": {}}); err != nil {
			t.Fatal(err)
		}
		if n%2 == 0 {
			large.Pairs = append(large.Pairs, TrialPair{fmt.Sprintf("large%d", n-1), id})
		}
	}
	if _, err := store.CompareSavedReports(context.Background(), large); err == nil {
		t.Fatal("aggregate read budget bypassed")
	}
}

// TestComparisonSelectionAndDelivery rejects ambiguous IDs/axes and preserves independent receiver evidence.
func TestComparisonSelectionAndDelivery(t *testing.T) {
	base := ComparisonRequest{Pairs: []TrialPair{{"a", "b"}}, Axes: []string{}, Endpoint: "local", Scope: "main"}
	oversized := &ReplayedReport{Manifest: EvidenceManifest{Plan: testutil.Plan()}}
	oversized.Manifest.Plan.Phases = make([]profile.Phase, 65)
	serverScope := base
	serverScope.Endpoint = "server"
	if _, err := comparisonTrial("oversized", oversized, serverScope); err == nil {
		t.Fatal("missing-server branch bypassed phase cap")
	}
	for _, bad := range []ComparisonRequest{{}, {Pairs: base.Pairs, Endpoint: "local", Scope: "main"}, {Pairs: []TrialPair{{"a", "a"}}, Axes: []string{}, Endpoint: "local", Scope: "main"}, {Pairs: base.Pairs, Axes: []string{"port", "port"}, Endpoint: "local", Scope: "main"}, {Pairs: base.Pairs, Axes: []string{"environment"}, Endpoint: "local", Scope: "main"}, {Pairs: base.Pairs, Axes: []string{}, Endpoint: "local", Scope: "01"}, {Pairs: base.Pairs, Axes: []string{}, Endpoint: "local", Scope: "64"}} {
		if bad.Validate() == nil {
			t.Fatal("ambiguous selection accepted", bad)
		}
	}
	a, b := reconcileFixture()
	a.Reconciliation = reconcileReports(a, b)
	a.ServerReport = b
	target := comparisonFixture(t, "a", 1, "80")
	before := append([]*string{}, target.Metrics...)
	comparisonDelivery(&target, a, base)
	if *target.Metrics[16] != "2" || *target.Metrics[17] != "1" || !reflect.DeepEqual(before[:16], target.Metrics[:16]) || target.Metrics[25] != nil {
		t.Fatal("delivery gating or local RTT changed")
	}
	a.Reconciliation = nil
	target = comparisonFixture(t, "a", 1, "80")
	comparisonDelivery(&target, a, base)
	if target.Metrics[16] != nil {
		t.Fatal("missing remote became zero")
	}
}
