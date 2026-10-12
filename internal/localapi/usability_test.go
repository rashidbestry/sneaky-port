package localapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sneaky-port/internal/control"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// TestDashboardRecoversActiveRun supplies the admitted public handle/progress on setup without a second start.
func TestDashboardRecoversActiveRun(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	feed := &measure.ProgressFeed{}
	feed.Publish(measure.Progress{Stage: "measurement", Elapsed: 123, Total: 456, Main: measure.LiveCounts{Planned: 7}})
	s.mu.Lock()
	s.active = "owned-run"
	s.runs[s.active] = &runState{status: Status{ID: s.active, ProfileID: s.plan.ID, State: "executing"}, progress: feed}
	s.mu.Unlock()
	w := httptest.NewRecorder()
	s.handle(w, request(s, "GET", "/api/v1/setup", "recover", ""))
	var response struct {
		Result struct {
			Active *Status `json:"active_run"`
		} `json:"result"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Result.Active == nil || response.Result.Active.ID != "owned-run" || response.Result.Active.Progress.Main.Planned != 7 {
		t.Fatal("active run recovery unavailable")
	}
	response.Result.Active.Progress.Main.Planned = 99
	if feed.Snapshot().Main.Planned != 7 {
		t.Fatal("setup mutated the accounting-owner snapshot")
	}
	s.mu.Lock()
	s.active = ""
	delete(s.runs, "owned-run")
	s.mu.Unlock()
}

// TestReportLabelCatalogGuards keeps friendly labels optional, protected and explicitly unverified/unavailable.
func TestReportLabelCatalogGuards(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	importDiagnosticFixture(t, s)
	r := request(s, "GET", "/api/v1/reports?labels=1", "labels", "")
	w := httptest.NewRecorder()
	s.handle(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "metadata_state") || strings.Contains(w.Body.String(), "events.jsonl") {
		t.Fatal("metadata catalog failed or exposed raw evidence")
	}
	for _, path := range []string{"/api/v1/profiles?labels=1", "/api/v1/reports?labels=0", "/api/v1/reports?labels=1&labels=1"} {
		invoke(t, s, request(s, "GET", path, "invalid-labels", ""), 400)
	}
	r = request(s, "GET", "/api/v1/reports?labels=1", "private", "")
	r.Header.Del("Authorization")
	invoke(t, s, r, 401)
}

// TestGuidedDashboardRealMeasurement drives the actual friendly UI over HTTP/SSH and real direct UDP/TCP.
// Fixtures emulate normal sshd channels; DOM logic differs from rendered-browser and live-path acceptance.
func TestGuidedDashboardRealMeasurement(t *testing.T) {
	runGuidedDashboardFixture(t, false)
}

// TestDashboardSixtySeconds verifies actual elapsed UDP/TCP runs; opt-in avoids slowing ordinary unit suites.
func TestDashboardSixtySeconds(t *testing.T) {
	if os.Getenv("SNEAKY_PORT_DASHBOARD_DURATION60") != "1" {
		t.Skip("set SNEAKY_PORT_DASHBOARD_DURATION60=1 for two real sixty-second runs")
	}
	runGuidedDashboardFixture(t, true)
}

// TestSixtySecondControlBudgetFailure preserves the reported default rejection before keys/sockets are opened.
func TestSixtySecondControlBudgetFailure(t *testing.T) {
	p := testutil.Plan()
	p.Phases[0].Duration, p.Grace = 60e9, 100e6
	p.Limits.HeartbeatInterval, p.Calibration.Cadence = 1e9, 1e9
	p.Limits.ControlRequests = 128
	p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 60, 60
	p.Limits.LedgerMessages, p.Limits.EventBytes, p.Limits.FileBytes = 7200, 32<<20, 64<<20
	_, err := control.Run(context.Background(), &control.Store{Root: t.TempDir()}, nil, p)
	if err == nil || !strings.Contains(err.Error(), "finite run exceeds control/calibration request budget") {
		t.Fatal("default sixty-second request-budget rejection not reproduced", err)
	}
}

// TestCurrentRunDiagnosticsGuards checks owned terminal identity, normal privacy/admission and the finite larger replay cap.
func TestCurrentRunDiagnosticsGuards(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	importDiagnosticFixture(t, s)
	report, err := s.store.SavedReport("diagnostic")
	if err != nil {
		t.Fatal(err)
	}
	status := Status{ID: "current", State: "partial", ReportID: "diagnostic", Revision: report.Manifest.PlanHash, MeasurementID: report.Manifest.RunID}
	s.runs[status.ID] = &runState{status: status}
	path := "/api/v1/runs/current/diagnostics?endpoint=local&scope=main"
	result := invoke(t, s, request(s, "GET", path, "current-read", ""), 200)
	encoded, err := json.Marshal(result.Result)
	if err != nil || !strings.Contains(string(encoded), `"report_id":"diagnostic"`) || !strings.Contains(string(encoded), `"complete":false`) {
		t.Fatal("current test projection lost owned report identity/partial evidence")
	}
	r := request(s, "GET", path, "current-unauthorized", "")
	r.Header.Del("Authorization")
	invoke(t, s, r, 401)
	r = request(s, "GET", path, "current-origin", "")
	r.Header.Set("Origin", "https://example.invalid")
	invoke(t, s, r, 403)
	for _, query := range []string{"limit=999999999", "report=diagnostic", "scope=main&scope=warmup"} {
		invoke(t, s, request(s, "GET", "/api/v1/runs/current/diagnostics?"+query, "current-query", ""), 400)
	}
	invoke(t, s, request(s, "GET", "/api/v1/runs/unknown/diagnostics", "current-unknown", ""), 404)
	invoke(t, s, request(s, "POST", path, "current-method", ""), 404)
	s.runs[status.ID].status.State = "executing"
	invoke(t, s, request(s, "GET", path, "current-active", ""), 409)
	s.runs[status.ID].status = status
	s.runs[status.ID].status.Revision = "different-frozen-plan"
	invoke(t, s, request(s, "GET", path, "current-mismatch", ""), 422)
	s.runs[status.ID].status = status
	s.savedMu.Lock()
	invoke(t, s, request(s, "GET", path, "current-busy", ""), 503)
	s.savedMu.Unlock()
	if control.APIReportBytes != 8<<20 || control.APIRunReportBytes != 64<<20 {
		t.Fatal("saved/current report caps changed")
	}
	raw, err := os.OpenFile(filepath.Join(s.store.Root, "reports", "diagnostic", "events.jsonl"), os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = raw.Truncate(control.APIRunReportBytes + 1)
	raw.Close()
	if err != nil {
		t.Fatal(err)
	}
	invoke(t, s, request(s, "GET", path, "current-cap", ""), 422)
}

// runGuidedDashboardFixture owns temporary HTTP/SSH/direct data endpoints for short or actual sixty-second trials.
func runGuidedDashboardFixture(t *testing.T, long bool) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable")
	}
	sshFixture := testutil.StartSSH(t, control.DashboardServer)
	var connector *control.SSHDashboard
	var starts atomic.Int32
	plans := make(chan profile.Plan, 2)
	var s *Server
	s = fixtureSavedAPI(t, func(ctx context.Context, plan profile.Plan) (control.RunResult, error) { /* Keep exact server negotiation/data/report lifecycle beneath the actual UI. */
		starts.Add(1)
		plans <- plan
		if long {
			t.Logf("Running %s for %d seconds with %d bounded control requests", plan.Adapter, plan.Duration()/1e9, plan.Limits.ControlRequests)
		}
		result, err := connector.Run(ctx, plan)
		if long && result.ReportPath != "" {
			_, savedErr := s.store.SavedReport(filepath.Base(result.ReportPath))
			t.Logf("%s report replay under original saved 8-MiB cap: %v", plan.Adapter, savedErr)
			t.Logf("%s lifecycle status=%s server=%s local_complete=%v runner_error=%v", plan.Adapter, result.Status, result.ServerStatus, result.Local != nil && result.Local.Complete, err)
			var remote control.EvidenceManifest
			if encoded, readErr := os.ReadFile(filepath.Join(result.ReportPath, "remote-server", "manifest.json")); readErr == nil && json.Unmarshal(encoded, &remote) == nil {
				t.Logf("server complete=%v storage=%s raw_bytes=%d events=%d event_limit=%d", remote.Complete, remote.Storage, remote.RawBytes, remote.Events, plan.Limits.EventBytes)
			}
		}
		return result, err
	})
	if long {
		// Match operator-example lifecycle/queue defaults rather than the intentionally short unit-fixture leases.
		s.plan.Limits.HeartbeatInterval, s.plan.Calibration.Cadence = 1e9, 1e9
		s.plan.Limits.Lease, s.plan.Calibration.Expiry = 10e9, 10e9
		s.plan.Limits.SetupTimeout, s.plan.Limits.WriteTimeout, s.plan.Limits.ShutdownTimeout = 5e9, 5e9, 5e9
		s.plan.Limits.QueueMessages, s.plan.Limits.ReceiptReserve, s.plan.Limits.Outstanding = 64, 8, 1024
		s.plan.Limits.MaxLag = 10e6
		s.plan.SpikeThreshold = 10e6
		s.plan.Limits.ControlRequests = 128
		hash, err := s.plan.Hash()
		if err != nil {
			t.Fatal(err)
		}
		s.revision = hex.EncodeToString(hash[:])
	}
	connector = control.NewSSHDashboard(s.store)
	if err = connector.SetHelperBinary(testutil.DashboardPayload()); err != nil {
		t.Fatal(err)
	}
	s.EnableSSH(connector)
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	udpPort := udp.LocalAddr().(*net.UDPAddr).Port
	udp.Close()
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tcpPort := tcp.Addr().(*net.TCPAddr).Port
	tcp.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { /* Serve only this isolated loopback API/store during the guided workflow. */
		done <- s.Serve(ctx, 5*time.Second)
	}()
	t.Cleanup(func() { /* Cancel and join only the owned API/helper, preserving all user services/data. */
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	budget := 40 * time.Second
	if long {
		budget = 180 * time.Second
	}
	commandCtx, stop := context.WithTimeout(context.Background(), budget)
	defer stop()
	command := exec.CommandContext(commandCtx, node, "web/dashboard_test.cjs")
	command.Env = append(os.Environ(), "DASHBOARD_TEST_URL="+s.URL(), "DASHBOARD_TEST_SSH_PORT="+strconv.Itoa(sshFixture.Port), "DASHBOARD_GUIDED_REAL=1", "DASHBOARD_UDP_PORT="+strconv.Itoa(udpPort), "DASHBOARD_TCP_PORT="+strconv.Itoa(tcpPort))
	if long {
		command.Env = append(command.Env, "DASHBOARD_TEST_SECONDS=60")
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("guided UI fixture: %v\n%s", err, output)
	}
	if starts.Load() != 2 {
		t.Fatal("guided retry duplicated a run", starts.Load())
	}
	for index := 0; index < 2; index++ {
		plan := <-plans
		if long {
			if plan.Duration() != 60e9 || plan.Limits.ControlRequests <= 128 || plan.Phases[0].Client.Mode != "steady" || uint64(plan.Phases[0].Client.RateNumerator) != 60 || uint64(plan.Phases[0].Server.RateNumerator) != 60 {
				t.Fatal("sixty-second settings were not sized/negotiated exactly")
			}
			continue
		}
		if len(plan.Phases) != 2 || plan.Phases[1].Deadline == nil || uint64(*plan.Phases[1].Deadline) != 12125000 {
			t.Fatal("inline phases/deadline did not reach the negotiated runner")
		}
		for _, phase := range plan.Phases {
			if uint64(phase.Duration) != 60000000 || phase.Client.Mode != "burst" || uint64(phase.Client.Active) != 20000000 || uint64(phase.Client.Gap) != 10000000 || uint64(phase.Client.RateNumerator) != 2997 || uint64(phase.Client.RateDenominator) != 50 || phase.Client.ContentBytes != 41 || phase.Server.Mode != "steady" || phase.Server.ContentBytes != 53 {
				t.Fatal("typed directional settings changed before exact endpoint agreement")
			}
		}
		if plan.Adapter == "tcp" && string(plan.Options["tcp_no_delay"]) != "true" {
			t.Fatal("explicit TCP checkbox was not negotiated")
		}
	}
}
