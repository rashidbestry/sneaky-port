package localapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// fixtureSavedAPI adds an isolated owned store; real guard/router code remains the test boundary.
func fixtureSavedAPI(t *testing.T, runner Runner) *Server {
	t.Helper()
	s := fixtureAPI(t, runner)
	store, err := control.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.store = store
	return s
}

// savedRequest constructs exact versioned documents without depending on browser number conversion.
func savedRequest(t *testing.T, s *Server, method, path, id string, fields map[string]any, code int) envelope {
	t.Helper()
	fields["schema_version"], fields["request_id"] = 1, id
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return invoke(t, s, request(s, method, path, id, string(b)), code)
}

// TestSavedProfilesFreezeActiveRun proves validation is unsaved, saved selection is explicit and edits stay future-only.
func TestSavedProfilesFreezeActiveRun(t *testing.T) {
	plans := make(chan profile.Plan, 2)
	release := make(chan struct{})
	s := fixtureSavedAPI(t, func(ctx context.Context, p profile.Plan) (control.RunResult, error) { // Capture the runner's private plan while allowing edits through independent HTTP requests.
		plans <- p
		select {
		case <-release:
		case <-ctx.Done():
		}
		return control.RunResult{Status: "completed"}, nil
	})
	p := testutil.Plan()
	p.ID = "saved"
	savedRequest(t, s, "POST", "/api/v1/profiles/validate", "validate", map[string]any{"profile_revision": "", "profile": p}, 200)
	if ids, _ := s.store.SavedIDs("profiles"); len(ids) != 0 {
		t.Fatal("validation persisted unsaved edits")
	}
	e := savedRequest(t, s, "PUT", "/api/v1/profiles/saved", "save", map[string]any{"profile_revision": "", "profile": p}, 200)
	b, _ := json.Marshal(e.Result)
	var old control.SavedProfile
	json.Unmarshal(b, &old)
	start := map[string]any{"profile_id": p.ID, "profile_revision": old.Revision}
	e = savedRequest(t, s, "POST", "/api/v1/runs", "begin", start, 200)
	status := statusResult(t, e)
	select {
	case frozen := <-plans:
		if frozen.ID != p.ID {
			t.Fatal("saved selection lost")
		}
	case <-time.After(time.Second):
		t.Fatal("runner not admitted")
	}
	p.SpikeThreshold++
	savedRequest(t, s, "PUT", "/api/v1/profiles/saved", "edit", map[string]any{"profile_revision": old.Revision, "profile": p}, 200)
	savedRequest(t, s, "PUT", "/api/v1/profiles/saved", "stale", map[string]any{"profile_revision": old.Revision, "profile": p}, 409)
	active := statusResult(t, invoke(t, s, request(s, "GET", "/api/v1/runs/"+status.ID, "status", ""), 200))
	if active.Revision != old.Revision {
		t.Fatal("save changed active snapshot")
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		busy := s.active != ""
		s.mu.Unlock()
		if !busy {
			break
		}
		time.Sleep(time.Millisecond)
	}
	savedRequest(t, s, "POST", "/api/v1/runs", "begin", start, 200) // Retry remains original acknowledgment after saved revision changes.
	savedRequest(t, s, "POST", "/api/v1/runs", "old-start", map[string]any{"profile_id": p.ID, "profile_revision": old.Revision}, 409)
	current, _ := s.store.Profile(p.ID)
	savedRequest(t, s, "POST", "/api/v1/runs", "next", map[string]any{"profile_id": p.ID, "profile_revision": current.Revision}, 200)
	select {
	case frozen := <-plans:
		if frozen.SpikeThreshold != p.SpikeThreshold {
			t.Fatal("next run ignored explicit save")
		}
	case <-time.After(time.Second):
		t.Fatal("next runner not admitted")
	}
}

// TestSavedLaunchPrecedence exposes the persisted revision for a colliding launch ID and rejects corrupt saved state.
func TestSavedLaunchPrecedence(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	p := testutil.Plan()
	p.SpikeThreshold++
	saved, err := s.store.SaveProfile(p.ID, "", p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Revision() != saved.Revision || s.Revision() == s.revision {
		t.Fatal("bootstrap revision did not select saved launch ID")
	}
	if err = os.WriteFile(filepath.Join(s.store.Root, "profiles", p.ID+".json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if api, err := ListenWithStore("127.0.0.1:0", testutil.Plan(), s.store, stoppedRunner); err == nil {
		api.Close(context.Background())
		t.Fatal("corrupt saved launch accepted")
	}
}

// TestPublicPairingAndClientKeyAPI checks explicit trust replacement, single-key ownership and public-only exports.
func TestPublicPairingAndClientKeyAPI(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	enrollment := savedRequest(t, s, "POST", "/api/v1/client-keys", "generate", map[string]any{"client_id": "client"}, 200)
	b, _ := json.Marshal(enrollment.Result)
	if strings.Contains(string(b), "PRIVATE") || strings.Contains(string(b), "key_ref") || strings.Contains(string(b), s.store.Root) {
		t.Fatal("private client material leaked")
	}
	savedRequest(t, s, "POST", "/api/v1/client-keys", "repeat", map[string]any{"client_id": "other"}, 409)
	invoke(t, s, request(s, "GET", "/api/v1/client-keys/client/public", "public", ""), 200)
	invoke(t, s, request(s, "GET", "/api/v1/client-keys/other/public", "wrong", ""), 404)
	pub, err := control.GenerateKey(filepath.Join(t.TempDir(), "host"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := control.Bundle{Version: 1, ServerID: "server", ControlAddress: "127.0.0.1", ControlPort: 22220, HostPublicKey: pub.PublicKey, HostFingerprint: pub.Fingerprint, Contracts: []int{1}}
	fields := map[string]any{"pairing_id": "peer", "replace": false, "bundle": bundle}
	savedRequest(t, s, "POST", "/api/v1/pairings/import", "import", fields, 200)
	savedRequest(t, s, "POST", "/api/v1/pairings/import", "duplicate", fields, 409)
	fields["replace"] = true
	savedRequest(t, s, "POST", "/api/v1/pairings/import", "replace", fields, 200)
	bundle.HostFingerprint = "bad"
	fields["bundle"] = bundle
	savedRequest(t, s, "POST", "/api/v1/pairings/import", "bad", fields, 422)
	invoke(t, s, request(s, "GET", "/api/v1/pairings?limit=1", "list", ""), 200)
	invoke(t, s, request(s, "GET", "/api/v1/pairings/peer", "read", ""), 200)
}

// TestSavedRouteSecurityBounds applies all boundary guards to added routes and rejects ambiguous/oversized bodies.
func TestSavedRouteSecurityBounds(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	for _, path := range []string{"/api/v1/profiles", "/api/v1/pairings", "/api/v1/reports", "/api/v1/reports/x/export?format=json", "/api/v1/profiles/validate", "/api/v1/pairings/import", "/api/v1/client-keys", "/api/v1/client-keys/client/public", "/api/v1/reports/import"} {
		for _, attack := range []string{"auth", "origin", "host", "peer"} {
			r := request(s, "POST", path, "attack", "{}")
			switch attack {
			case "auth":
				r.Header.Del("Authorization")
			case "origin":
				r.Header.Set("Origin", "null")
			case "host":
				r.Host = "evil"
			case "peer":
				r.RemoteAddr = "192.0.2.1:1"
			}
			code := 403
			if attack == "auth" {
				code = 401
			}
			invoke(t, s, r, code)
		}
	}
	for _, path := range []string{"/api/v1/profiles?limit=17", "/api/v1/reports?cursor=-1", "/api/v1/pairings?token=secret", "/api/v1/reports/x/export?format=json&format=csv", "/api/v1/reports/x/export?format=bad", "/api/v1/profiles/%2e%2e", "/api/v1/profiles/x?path=secret"} {
		invoke(t, s, request(s, "GET", path, "read", ""), 400)
	}
	for _, body := range []string{`{"schema_version":1,"request_id":"bad","client_id":"c","key_ref":"private"}`, `{"schema_version":1,"schema_version":1,"request_id":"bad","client_id":"c"}`, `{"schema_version":1,"request_id":"bad","client_id":null}`, `{"schema_version":2,"request_id":"bad","client_id":"c"}`} {
		invoke(t, s, request(s, "POST", "/api/v1/client-keys", "bad", body), 400)
	}
	invoke(t, s, request(s, "POST", "/api/v1/reports/import", "huge", strings.Repeat("x", profile.DocumentMax+1)), 413)
	s.savedMu.Lock()
	invoke(t, s, request(s, "GET", "/api/v1/reports", "busy", ""), 503)
	s.savedMu.Unlock()
	if _, err := os.Stat(filepath.Join(s.store.Root, "client.json")); !os.IsNotExist(err) {
		t.Fatal("rejected request generated identity")
	}
}

// TestReportAPINotFoundAndFormats rejects missing reports and unsupported formats before publishing raw success.
func TestReportAPINotFoundAndFormats(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	invoke(t, s, request(s, "GET", "/api/v1/reports/missing", "read", ""), 404)
	for _, format := range []string{"json", "jsonl", "csv"} {
		invoke(t, s, request(s, "GET", "/api/v1/reports/missing/export?format="+format, "export", ""), 404)
	}
	var out boundedExport
	if _, err := out.Write(make([]byte, 16<<20+1)); err == nil || out.Len() != 0 {
		t.Fatal("export expansion cap failed")
	}
	w := httptest.NewRecorder()
	s.handle(w, request(s, "GET", "/api/v1/reports?limit=1", "list", ""))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal("empty list unavailable")
	}
}
