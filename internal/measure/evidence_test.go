package measure

import (
	"reflect"
	"slices"
	"sneaky-port/internal/profile"
	"testing"
)

// TestEvidenceTargetsPreserveValues exercises exact advisory boundaries and independent packet/receiver gates.
func TestEvidenceTargetsPreserveValues(t *testing.T) {
	p := DefaultEvidencePolicy()
	for _, n := range []profile.Decimal{0, 19, 20, 199, 200, 999, 1000} {
		value := "9007199254740993"
		s := Summary{Planned: n + 1, RTTSamples: n, RTTP95: &value, RTTP99: &value, RTTVariation: []profile.Decimal{1, 2}, VariationGaps: 1}
		before := s
		labels := LabelSummary(s, true, p)
		if !reflect.DeepEqual(s, before) || labels["RTTP95"].Completed != n || slices.Contains(labels["RTTMedian"].Reasons, "small_sample") != (n < 20) || slices.Contains(labels["RTTP95"].Reasons, "small_sample") != (n < 200) || slices.Contains(labels["RTTP99"].Reasons, "small_sample") != (n < 1000) {
			t.Fatal("value or target boundary changed", n)
		}
		if labels["Received"].Coverage != "unavailable" || labels["TransportPacketLoss"].Coverage != "unavailable" || labels["RTTP95"].Coverage != map[bool]string{true: "unavailable", false: "complete"}[n == 0] {
			t.Fatal("independent metric gate changed", n)
		}
		if labels["VariationP95"].Completed != 2 || labels["VariationP95"].Gaps != 1 {
			t.Fatal("variation population lost")
		}
	}
}

// TestEvidencePartialAndPolicy checks complete poor delivery, partial observation and bounded explicit overrides.
func TestEvidencePartialAndPolicy(t *testing.T) {
	p := DefaultEvidencePolicy()
	p.P95 = 20
	p.P99 = 20
	if p.Validate() != nil {
		t.Fatal("explicit preference rejected")
	}
	s := Summary{Planned: 100, Skipped: 80, LocalMisses: 80, RTTSamples: 20, Late: 20, DeadlineMisses: 100}
	l := LabelSummary(s, true, p)["RTTP95"]
	if l.Coverage != "complete" || l.Eligible != 20 || l.Missing != 0 || slices.Contains(l.Reasons, "small_sample") {
		t.Fatal("poor quality mistaken for incomplete evidence", l)
	}
	if LabelSummary(s, false, p)["RTTP95"].Coverage != "partial" {
		t.Fatal("partial observation upgraded")
	}
	for _, bad := range []EvidencePolicy{{0, 20, 200, 1000, 3}, {1, 0, 200, 1000, 3}, {1, 20, 19, 1000, 3}, {1, 20, 200, 1000001, 3}, {1, 20, 200, 1000, 9}} {
		if bad.Validate() == nil {
			t.Fatal("invalid preferences accepted", bad)
		}
	}
}
