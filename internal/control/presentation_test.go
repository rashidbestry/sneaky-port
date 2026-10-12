package control

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"sneaky-port/internal/calibration"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"strings"
	"testing"
)

// TestPresentationReplayExportIndependence checks derived defaults, explicit overrides and all existing formats.
func TestPresentationReplayExportIndependence(t *testing.T) {
	dir := partialReplayFixture(t)
	r, err := ReplayReport(dir, 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(r.Summary)
	hash := r.Manifest.PlanHash
	p := measure.DefaultEvidencePolicy()
	p.Median = 1
	p.P95 = 1
	p.P99 = 1
	if err := ApplyPresentation(r, p); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(r.Summary)
	if !bytes.Equal(before, after) || r.Manifest.PlanHash != hash || r.Presentation.Policy != p {
		t.Fatal("presentation changed frozen evidence")
	}
	for _, format := range []string{"json", "jsonl", "csv"} {
		var out bytes.Buffer
		if err := ExportReport(&out, r, format); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "evidence_presentation") || !strings.Contains(out.String(), "markings_unobserved") {
			t.Fatal("labels missing from export", format)
		}
	}
	fresh, err := ReplayReport(dir, 8<<20)
	if err != nil || fresh.Presentation.Policy != measure.DefaultEvidencePolicy() || !reflect.DeepEqual(fresh.Summary, r.Summary) {
		t.Fatal("override persisted or replay changed", err)
	}
}

// TestPresentationContextIndependence keeps clock/marking faults separate from valid elapsed RTT and fixed public reasons.
func TestPresentationContextIndependence(t *testing.T) {
	value := "42"
	r := &ReplayedReport{Evidence: engine.Result{Complete: true, ObservationCutoff: 100, Records: []json.RawMessage{json.RawMessage(`{"Kind":"marking","At":"1","Error":"private-secret-path"}`)}}, Summary: EvidenceSummary{Local: &LocalRun{Summary: measure.Summary{Planned: 1000, RTTSamples: 1000, RTTP99: &value}}}, Reconciliation: &Reconciliation{OneWay: &OneWayReport{Directions: [2]OneWayCounts{{Direction: 1, Estimated: 1, Unavailable: 1}, {Direction: 2, Unavailable: 2}}, Results: []OneWayResult{{Direction: 1, Interval: calibration.Interval{Reason: "invalid/expired clock epoch"}}}}}}
	p := ReportLabels(r, measure.DefaultEvidencePolicy())
	if p.Main["RTTP99"].Coverage != "complete" || slices.Contains(p.Main["RTTP99"].Reasons, "clock_bounds_unverified") || !slices.Contains(p.Context["OneWayUpstream"].Reasons, "clock_epoch_or_expiry_unavailable") || !slices.Contains(p.Context["OuterMarkings"].Reasons, "marking_application_failed") {
		t.Fatal("independent context gate changed", p)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "private-secret-path") {
		t.Fatal("unbounded/private reason leaked")
	}
	r.TailBytes = 1
	p = ReportLabels(r, measure.DefaultEvidencePolicy())
	if p.Main["RTTP99"].Coverage != "partial" || !slices.Contains(p.Main["RTTP99"].Reasons, "discarded_tail") {
		t.Fatal("torn report presentation upgraded")
	}
}

// TestRepeatedEvidenceAdvisoryTargets retains the mandatory two-pair gate and unchanged tie/mixed decisions.
func TestRepeatedEvidenceAdvisoryTargets(t *testing.T) {
	zero, positive, negative := "0", "1", "-1"
	for _, deltas := range [][]*string{{&zero}, {&zero, &zero}, {&zero, &zero, &zero}, {&positive, &negative}, {&zero, nil}} {
		rows := [][]MetricDelta{}
		for _, delta := range deltas {
			row := make([]MetricDelta, len(comparisonNames))
			row[0] = MetricDelta{Metric: "DeadlineMisses", Delta: delta}
			rows = append(rows, row)
		}
		r := repeatDeltas(rows)
		state := r[0].State
		labelRepeatedMetrics(r, rows, measure.DefaultEvidencePolicy())
		if r[0].State != state || slices.Contains(r[0].Evidence.Reasons, "few_repeats") != (r[0].Trials < 3) || slices.Contains(r[0].Evidence.Reasons, "two_pairs_required") != (r[0].Trials < 2) {
			t.Fatal("advisory target altered gates", r[0])
		}
	}
}
