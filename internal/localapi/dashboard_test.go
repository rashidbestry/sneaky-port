package localapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/control"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
	"sneaky-port/internal/testutil/extension"
)

// TestDashboardShellGuards permits public static code only; private bootstrap/API reads remain guarded.
func TestDashboardShellGuards(t *testing.T) {
	s := fixtureAPI(t, stoppedRunner)
	for _, path := range []string{"/", "/navigation.js", "/dashboard.js", "/dashboard.css", "/diagnostics.js", "/compare.js", "/workspace.js"} {
		r := request(s, "GET", path, "shell", "")
		r.Header.Del("Authorization")
		r.Header.Del("X-Request-ID")
		w := httptest.NewRecorder()
		s.handle(w, r)
		if w.Code != 200 || strings.Contains(w.Body.String(), s.Token()) || strings.Contains(w.Body.String(), s.plan.ServerAddress) || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-inline") {
			t.Fatal("public shell/credential/CSP boundary failed", path)
		}
		for _, attack := range []string{"host", "origin", "peer", "cross_site"} {
			r := request(s, "GET", path, "attack", "")
			switch attack {
			case "host":
				r.Host = "localhost:9999"
			case "origin":
				r.Header.Set("Origin", "null")
			case "peer":
				r.RemoteAddr = "192.0.2.1:1234"
			case "cross_site":
				r.Header.Set("Sec-Fetch-Site", "cross-site")
			}
			invoke(t, s, r, 403)
		}
		invoke(t, s, request(s, "POST", path, "method", ""), 400)
		invoke(t, s, request(s, "GET", path+"?token=bad", "query", ""), 400)
	}
	r := request(s, "GET", "/api/v1/setup", "setup", "")
	r.Header.Del("Authorization")
	invoke(t, s, r, 401)
	for _, path := range []string{"/web/index.html", "/dashboard_test.cjs", "/api/v1/setup/extra", "/api/v1/setup?token=bad", "/%64ashboard.js"} {
		r := request(s, "GET", path, "path", "")
		w := httptest.NewRecorder()
		s.handle(w, r)
		if w.Code == 200 {
			t.Fatal("unexpected asset/bootstrap alias", path)
		}
	}
}

// TestDashboardSetupSnapshot distinguishes transient/saved revisions and exports only public enrollment.
func TestDashboardSetupSnapshot(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	read := func(saved bool) { // Read the protected setup response and verify normalized exact profile/revision state.
		e := invoke(t, s, request(s, "GET", "/api/v1/setup", "setup", ""), 200)
		b, _ := json.Marshal(e.Result)
		var out struct {
			Profile control.SavedProfile `json:"selected_profile"`
			Saved   bool                 `json:"saved"`
			Client  *control.Enrollment  `json:"client_enrollment"`
		}
		if json.Unmarshal(b, &out) != nil || out.Saved != saved || out.Profile.Plan.ID != s.plan.ID || out.Profile.Revision != s.Revision() || strings.Contains(string(b), "key_ref") || strings.Contains(string(b), s.store.Root) {
			t.Fatal("setup snapshot state/secrecy failed")
		}
		if saved && (out.Client == nil || out.Client.ClientID != "client") {
			t.Fatal("public enrollment missing")
		}
	}
	read(false)
	if _, err := s.store.SaveProfile(s.plan.ID, "", s.plan); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.InitClient("client", ""); err != nil {
		t.Fatal(err)
	}
	read(true)
	s.savedMu.Lock()
	invoke(t, s, request(s, "GET", "/api/v1/setup", "busy", ""), 503)
	s.savedMu.Unlock()
	for _, attack := range []string{"host", "origin", "peer"} {
		r := request(s, "GET", "/api/v1/setup", "attack", "")
		switch attack {
		case "host":
			r.Host = "evil"
		case "origin":
			r.Header.Set("Origin", "null")
		case "peer":
			r.RemoteAddr = "192.0.2.1:1234"
		}
		invoke(t, s, r, 403)
	}
}

// TestDashboardAdapterValidation rejects unsupported options during both unsaved validation and explicit save.
func TestDashboardAdapterValidation(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	for _, carrier := range []string{"foreign", "udp", "tcp"} {
		p := s.plan
		p.Adapter = carrier
		p.Options = map[string]json.RawMessage{"unsupported": json.RawMessage("true")}
		for _, op := range []struct{ method, path string }{{"POST", "/api/v1/profiles/validate"}, {"PUT", "/api/v1/profiles/" + p.ID}} {
			savedRequest(t, s, op.method, op.path, "invalid", map[string]any{"profile_revision": "", "profile": p}, 422)
		}
	}
	if ids, _ := s.store.SavedIDs("profiles"); len(ids) != 0 {
		t.Fatal("unsupported adapter profile persisted")
	}
}

// TestDashboardJavaScriptWorkflow runs the actual bundled script against real loopback API/storage controls.
// The DOM harness checks control logic; rendered browser QA is separate from this synthetic lifecycle fixture.
func TestDashboardJavaScriptWorkflow(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable for dashboard script fixture")
	}
	var starts atomic.Int32
	s := fixtureSavedAPI(t, func(ctx context.Context, _ profile.Plan) (control.RunResult, error) { // Hold a synthetic run until explicit cancellation; no network measurement is claimed.
		starts.Add(1)
		if feed := measure.ProgressSink(ctx); feed != nil {
			feed.Publish(measure.Progress{AsOf: 9007199254740993, Cutoff: 9007199254740992, Elapsed: 500, Total: 1000, Stage: "measurement", Phase: "fixture", AccountingValid: true, Main: measure.LiveCounts{Planned: 9007199254740993, DeadlineMisses: 1}, Warmup: measure.LiveCounts{}, PeerReceived: 2})
		}
		<-ctx.Done()
		return control.RunResult{Status: "partial"}, nil
	})
	sshFixture := testutil.StartSSH(t, control.DashboardServer)
	connector := control.NewSSHDashboard(s.store)
	if err := connector.SetHelperBinary(testutil.DashboardPayload()); err != nil {
		t.Fatal(err)
	}
	s.EnableSSH(connector)
	s.store.Adapters = adapter.Default()
	if err := extension.Register(s.store.Adapters); err != nil {
		t.Fatal(err)
	}
	importDiagnosticFixture(t, s)
	importComparisonFixture(t, s)
	pub, err := control.GenerateKey(t.TempDir() + "/host")
	if err != nil {
		t.Fatal(err)
	}
	bundle, _ := json.Marshal(control.Bundle{Version: 1, ServerID: "fixture", ControlAddress: "127.0.0.1", ControlPort: 22220, HostPublicKey: pub.PublicKey, HostFingerprint: pub.Fingerprint, Contracts: []int{1}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { // Serve real guards/HTTP on the owned listener for the Node browser-logic harness.
		done <- s.Serve(ctx, 2*time.Second)
	}()
	t.Cleanup(func() { // Stop and join the isolated API even if the script assertion fails.
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	commandCtx, stop := context.WithTimeout(context.Background(), 25*time.Second)
	defer stop()
	command := exec.CommandContext(commandCtx, node, "web/dashboard_test.cjs")
	command.Env = append(os.Environ(), "DASHBOARD_TEST_URL="+s.URL(), "DASHBOARD_TEST_SSH_PORT="+strconv.Itoa(sshFixture.Port), "DASHBOARD_TEST_BUNDLE="+string(bundle))
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("dashboard script fixture: %v\n%s", err, out)
	}
	if starts.Load() != 1 {
		t.Fatal("lost acknowledgment retry started duplicate work", starts.Load())
	}
}
