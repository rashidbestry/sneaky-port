package localapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sneaky-port/internal/control"
)

// importComparisonFixture adds a distinct empty partial run through the actual offline report validator.
func importComparisonFixture(t *testing.T, s *Server) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.store.Root, "reports", "diagnostic", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m control.EvidenceManifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	m.RunID = strings.Repeat("3", 32)
	b, _ = json.Marshal(m)
	if _, err := s.store.ImportReport("comparison", map[string][]byte{"manifest.json": b, "events.jsonl": {}}); err != nil {
		t.Fatal(err)
	}
}

// TestComparisonProtectedStrictReplay checks actual router guards, request shape, serialization and tampering.
func TestComparisonProtectedStrictReplay(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	importDiagnosticFixture(t, s)
	importComparisonFixture(t, s)
	path := "/api/v1/reports/compare"
	fields := map[string]any{"pairs": []control.TrialPair{{A: "diagnostic", B: "comparison"}}, "axes": []string{}, "endpoint": "local", "scope": "main"}
	for _, guard := range []string{"auth", "host", "origin", "peer", "cross_site"} {
		r := request(s, "POST", path, "guard", "")
		switch guard {
		case "auth":
			r.Header.Del("Authorization")
		case "host":
			r.Host = "foreign"
		case "origin":
			r.Header.Set("Origin", "null")
		case "peer":
			r.RemoteAddr = "192.0.2.1:1234"
		case "cross_site":
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		code := 403
		if guard == "auth" {
			code = 401
		}
		invoke(t, s, r, code)
	}
	result := savedRequest(t, s, "POST", path, "compare", fields, 200)
	b, _ := json.Marshal(result.Result)
	if !strings.Contains(string(b), `"state":"inconclusive"`) || strings.Contains(string(b), s.store.Root) || strings.Contains(string(b), s.Token()) {
		t.Fatal("partial/secret boundary lost", string(b))
	}
	for _, body := range []string{`{}`, `{"schema_version":1,"request_id":"bad","pairs":[],"axes":[],"endpoint":"local","scope":"main"}`, `{"schema_version":1,"request_id":"bad","pairs":[{"a":"a","b":"a"}],"axes":[],"endpoint":"local","scope":"main"}`, `{"schema_version":1,"request_id":"bad","pairs":[{"a":"a","b":"b"}],"axes":null,"endpoint":"local","scope":"main"}`, `{"schema_version":1,"schema_version":1,"request_id":"bad","pairs":[],"axes":[],"endpoint":"local","scope":"main"}`, `{"schema_version":1,"request_id":"foreign","pairs":[{"a":"diagnostic","b":"comparison"}],"axes":[],"endpoint":"local","scope":"main"}`} {
		invoke(t, s, request(s, "POST", path, "bad", body), 400)
	}
	invoke(t, s, request(s, "POST", path+"?scope=main", "query", ""), 400)
	s.savedMu.Lock()
	invoke(t, s, request(s, "POST", path, "busy", ""), 503)
	s.savedMu.Unlock()
	fields["endpoint"] = "server"
	result = savedRequest(t, s, "POST", path, "server", fields, 200)
	b, _ = json.Marshal(result.Result)
	if !strings.Contains(string(b), `"status":"unavailable"`) {
		t.Fatal("server absence concealed")
	}
	fields["endpoint"] = "local"
	fields["pairs"] = []control.TrialPair{{A: "diagnostic", B: "missing"}}
	savedRequest(t, s, "POST", path, "missing", fields, 404)
	fields["pairs"] = []control.TrialPair{{A: "diagnostic", B: "comparison"}}
	if err := os.WriteFile(filepath.Join(s.store.Root, "reports", "comparison", "events.jsonl"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	savedRequest(t, s, "POST", path, "tamper", fields, 422)
}
