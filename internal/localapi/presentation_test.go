package localapi

import (
	"encoding/json"
	"net/http/httptest"
	"sneaky-port/internal/control"
	"sneaky-port/internal/measure"
	"strings"
	"testing"
)

// TestPresentationProtectedQueries verifies independent explicit preferences, strict bounds and existing guards.
func TestPresentationProtectedQueries(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	importDiagnosticFixture(t, s)
	importComparisonFixture(t, s)
	for _, query := range []string{"p95_target=0", "p99_target=1000001", "repeat_target=9", "p95_target=0200", "p95_target=200&p95_target=200", "token=secret", "median_target=201"} {
		invoke(t, s, request(s, "GET", "/api/v1/reports/diagnostic/diagnostics?"+query, "bad", ""), 400)
		invoke(t, s, request(s, "GET", "/api/v1/reports/diagnostic/export?format=json&"+query, "bad", ""), 400)
	}
	query := "median_target=1&p95_target=1&p99_target=1&repeat_target=2"
	out := invoke(t, s, request(s, "GET", "/api/v1/reports/diagnostic/diagnostics?"+query, "labels", ""), 200)
	b, _ := json.Marshal(out.Result)
	var view diagnosticView
	if json.Unmarshal(b, &view) != nil || view.EvidencePolicy.P99 != 1 || view.Labels["Received"].Coverage != "unavailable" || view.ContextLabels["OuterMarkings"].Coverage != "unavailable" {
		t.Fatal("derived policy/independent evidence lost")
	}
	r := request(s, "GET", "/api/v1/reports/diagnostic/export?format=csv&"+query, "export", "")
	w := httptest.NewRecorder()
	s.handle(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "evidence_presentation.policy.p99_target") || strings.Contains(w.Body.String(), s.Token()) {
		t.Fatal("export labels or secrecy failed")
	}
	r = request(s, "GET", "/api/v1/reports/diagnostic/diagnostics?"+query, "guard", "")
	r.Header.Del("Authorization")
	invoke(t, s, r, 401)
	policy := measure.DefaultEvidencePolicy()
	policy.Repeated = 2
	envelope := savedRequest(t, s, "POST", "/api/v1/reports/compare", "compare-policy", map[string]any{"pairs": []control.TrialPair{{A: "diagnostic", B: "comparison"}}, "axes": []string{}, "endpoint": "local", "scope": "main", "evidence_policy": policy}, 200)
	b, _ = json.Marshal(envelope.Result)
	var comparison control.Comparison
	if json.Unmarshal(b, &comparison) != nil || comparison.EvidencePolicy.Repeated != 2 || comparison.Pairs[0].Metrics[0].Delta != nil || comparison.Repeated[0].Evidence.Coverage != "unavailable" {
		t.Fatal("preferences relaxed comparison gate")
	}
}
