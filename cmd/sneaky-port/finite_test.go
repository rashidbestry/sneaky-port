package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// TestFiniteCLI exercises command parsing, real authenticated finite execution and public partial output.
// This same-host fixture does not complete the Windows-to-Linux deployment gate.
func TestFiniteCLI(t *testing.T) {
	root := t.TempDir()
	serverStore, err := control.OpenStore(filepath.Join(root, "server"))
	if err != nil {
		t.Fatal(err)
	}
	clientStore, err := control.OpenStore(filepath.Join(root, "client"))
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Release setup resources after the server cleanup registered below has joined.
		ln.Close()
	})
	policy := control.ServerPolicy{Version: 1, ControlListen: ln.Addr().String(), DataBindIP: "127.0.0.1", PortMin: 1, PortMax: 65535, SetupTimeout: 2e9, ShutdownTimeout: 2e9, AuthPoll: 20e6, MaxConnections: 8, MaxAuthTries: 3, MaxControlRequests: 100, MaxBindAttempts: 16, MaxQueueBytes: 1 << 20, MaxLedgerMessages: 1000, MaxEventBytes: 1 << 20, MaxFileBytes: 2 << 20}
	bundle, err := serverStore.InitServer("server", "127.0.0.1", ln.Addr().(*net.TCPAddr).Port, policy, "")
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := clientStore.InitClient("client", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = serverStore.Enroll(enrollment); err != nil {
		t.Fatal(err)
	}
	if err = clientStore.ImportPairing("fixture", bundle, false); err != nil {
		t.Fatal(err)
	}
	id, err := serverStore.Server()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := control.LoadKey(id.KeyRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	server, err := control.NewServer(serverStore, signer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { // Serve only the isolated owned loopback endpoint.
		done <- server.Serve(ctx, ln)
	}()
	t.Cleanup(func() { // Join the fixture server after every CLI run has completed its bounded cleanup.
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("CLI fixture cleanup timeout")
		}
	})
	port, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := testutil.Plan()
	p.DestinationPort = port.LocalAddr().(*net.UDPAddr).Port
	port.Close()
	p.Phases[0].Duration, p.Grace, p.Deadline = 200e6, 100e6, 50e6
	p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 50, 25
	p.Limits.MaxLag = 30e6
	// This CLI success/cancellation fixture must not conflate storage saturation with lifecycle behavior.
	p.Limits.QueueMessages = 128
	path := filepath.Join(root, "plan.json")
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"client-run", "-data-dir", clientStore.Root, "-profile", path}
	var out bytes.Buffer
	if err = run(context.Background(), args, &out); err != nil {
		t.Fatal(err)
	}
	var result control.RunResult
	if err = json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || result.Local == nil || !result.Local.Complete {
		t.Fatal("CLI finite result missing")
	}
	for _, format := range []string{"json", "jsonl", "csv"} {
		out.Reset()
		if err = run(context.Background(), []string{"report-export", "-report", result.ReportPath, "-max-bytes", "4194304", "-format", format}, &out); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(out.Bytes(), []byte("completed")) || !bytes.Contains(out.Bytes(), []byte("unknown")) {
			t.Fatal("CLI export omitted local/remote status")
		}
	}
	out.Reset()
	if err = run(context.Background(), []string{"report-replay", "-report", result.ReportPath, "-max-bytes", "4194304"}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"saved_summary_verification": "matched"`)) {
		t.Fatal("CLI replay did not verify saved summary")
	}
	out.Reset()
	canceled, stop := context.WithCancel(context.Background())
	defer stop()
	triggerDone := make(chan struct{})
	go func() { // Cancel only after new raw records prove measurement began; race instrumentation can delay SSH/storage setup.
		defer close(triggerDone)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case <-canceled.Done():
				return
			case <-deadline.C:
				stop()
				return
			case <-ticker.C:
				dirs, _ := os.ReadDir(filepath.Join(clientStore.Root, "reports"))
				for _, dir := range dirs {
					if !dir.IsDir() || dir.Name() == filepath.Base(result.ReportPath) {
						continue
					}
					info, err := os.Stat(filepath.Join(clientStore.Root, "reports", dir.Name(), "events.jsonl"))
					if err == nil && info.Size() > 0 {
						stop()
						return
					}
				}
			}
		}
	}()
	if err = run(canceled, args, &out); err == nil {
		t.Fatal("interrupted CLI exit succeeded")
	}
	stop()
	<-triggerDone
	if err = json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "partial" || result.Local == nil || result.Local.Complete {
		t.Fatal("partial CLI evidence missing")
	}
	exerciseFiniteAPI(t, clientStore, path)
}

// exerciseFiniteAPI reuses the real SSH fixture to verify HTTP start/status/cancel and persisted evidence.
// These are same-host IPv4 UDP/SSH runs, distinct from deployment/native/live acceptance.
func exerciseFiniteAPI(t *testing.T, store *control.Store, path string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() { // Bootstrap through a pipe so token output never races an in-memory buffer or enters logs.
		err := run(ctx, []string{"client-api", "-data-dir", store.Root, "-profile", path}, writer)
		writer.CloseWithError(err)
		done <- err
	}()
	t.Cleanup(func() { // Stop the API and join all finite cleanup before the SSH fixture shuts down.
		cancel()
		reader.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("API CLI cleanup timeout")
		}
	})
	var bootstrap struct {
		Version   int    `json:"schema_version"`
		URL       string `json:"api_url"`
		Token     string `json:"session_token"`
		ProfileID string `json:"profile_id"`
		Revision  string `json:"profile_revision"`
	}
	if err := json.NewDecoder(reader).Decode(&bootstrap); err != nil {
		t.Fatal("API bootstrap failed", err)
	}
	if bootstrap.Version != 1 || len(bootstrap.Token) != 64 {
		t.Fatal("invalid API bootstrap")
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := profile.Decode(file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	p.ID = "api-saved"
	profileBody, _ := json.Marshal(map[string]any{"schema_version": 1, "request_id": "save", "profile_revision": "", "profile": p})
	saved := apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "PUT", "/api/v1/profiles/api-saved", "save", string(profileBody))
	bootstrap.ProfileID, bootstrap.Revision = p.ID, saved["profile_revision"].(string)
	apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "GET", "/api/v1/profiles/api-saved", "read-profile", "")
	apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "GET", "/api/v1/client-keys/client/public", "public", "")
	for _, interrupted := range []bool{false, true} {
		requestID := "complete"
		if interrupted {
			requestID = "cancelled"
		}
		body, _ := json.Marshal(map[string]any{"schema_version": 1, "request_id": requestID, "profile_id": bootstrap.ProfileID, "profile_revision": bootstrap.Revision})
		state := apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "POST", "/api/v1/runs", requestID, string(body))
		controlID, ok := state["control_run_id"].(string)
		if !ok || controlID == "" {
			t.Fatal("missing API run handle")
		}
		if interrupted {
			// Cancellation during setup is also bounded; data-phase cancellation is already covered by the CLI fixture above.
			apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "POST", "/api/v1/runs/"+controlID+"/cancel", "stop", `{"schema_version":1,"request_id":"stop"}`)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			state = apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "GET", "/api/v1/runs/"+controlID, "poll", "")
			if state["status"] != "executing" && state["status"] != "cancelling" {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if interrupted {
			if state["status"] != "failed" && state["status"] != "partial" {
				t.Fatal("cancel did not finalize", state["status"])
			}
		} else {
			if state["status"] != "completed" || state["local_complete"] != true || state["measurement_started"] != true {
				t.Fatal("API finite run missing", state)
			}
			id, ok := state["measurement_run_id"].(string)
			if !ok || len(id) != 32 {
				t.Fatal("measurement identity absent")
			}
			report, err := control.ReplayReport(filepath.Join(store.Root, "reports", id+"-client"), 4<<20)
			if err != nil || report.Verification != "matched" {
				t.Fatal("API run evidence failed offline replay", err)
			}
			exerciseReportAPI(t, client, bootstrap.URL, bootstrap.Token, store, id+"-client", report)
		}
		// A retry after terminal publication must return the original start handle without a new run.
		retry := apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "POST", "/api/v1/runs", requestID, string(body))
		if retry["control_run_id"] != controlID {
			t.Fatal("terminal start retry duplicated run")
		}
	}
}

// exerciseReportAPI verifies real HTTP replay/export and fixed-file import with transferred server evidence.
// Imported files provide offline consistency, not fresh authentication or production clock bounds.
func exerciseReportAPI(t *testing.T, client *http.Client, base, token string, store *control.Store, id string, original *control.ReplayedReport) {
	t.Helper()
	apiCLIRequest(t, client, base, token, "GET", "/api/v1/reports?limit=1", "reports", "")
	got := apiCLIRequest(t, client, base, token, "GET", "/api/v1/reports/"+id, "report", "")
	if got["saved_summary_verification"] != "matched" {
		t.Fatal("HTTP replay verification lost")
	}
	for _, format := range []string{"json", "jsonl", "csv"} {
		r, err := http.NewRequest("GET", base+"/api/v1/reports/"+id+"/export?format="+format, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Request-ID", "export")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal("report export HTTP failed", err)
		}
		b, err := io.ReadAll(response.Body)
		response.Body.Close()
		var expected bytes.Buffer
		if err != nil || control.ExportReport(&expected, original, format) != nil || response.StatusCode != 200 || !bytes.Equal(b, expected.Bytes()) {
			t.Fatal("report export diverged", format, err)
		}
	}
	files := map[string][]byte{}
	for _, name := range []string{"manifest.json", "events.jsonl", "summary.json", "server-transfer.json", "remote-server/manifest.json", "remote-server/events.jsonl", "remote-server/summary.json"} {
		b, err := os.ReadFile(filepath.Join(store.Root, "reports", id, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = b
	}
	body, _ := json.Marshal(map[string]any{"schema_version": 1, "request_id": "import-report", "report_id": "imported", "files": files})
	imported := apiCLIRequest(t, client, base, token, "POST", "/api/v1/reports/import", "import-report", string(body))
	wanted, _ := json.Marshal(original)
	actual, _ := json.Marshal(imported)
	var expected map[string]any
	json.Unmarshal(wanted, &expected)
	canonical, _ := json.Marshal(expected)
	if !bytes.Equal(actual, canonical) {
		t.Fatal("import changed recomputed report")
	}
	apiCLIRequest(t, client, base, token, "GET", "/api/v1/reports/imported", "read-import", "")
}

// apiCLIRequest exercises actual authenticated HTTP while keeping bootstrap credentials out of diagnostics.
func apiCLIRequest(t *testing.T, client *http.Client, base, token, method, path, id, body string) map[string]any {
	t.Helper()
	r, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Request-ID", id)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", base)
	response, err := client.Do(r)
	if err != nil {
		t.Fatal("API HTTP failed", err)
	}
	defer response.Body.Close()
	var envelope struct {
		Version int            `json:"schema_version"`
		Result  map[string]any `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil || response.StatusCode != 200 || envelope.Version != 1 {
		t.Fatal("API request failed", response.StatusCode, err)
	}
	return envelope.Result
}

// TestFreshClientAPI permits explicit key initialization from a new protected API without creating it at launch.
func TestFreshClientAPI(t *testing.T) {
	root := t.TempDir()
	p := testutil.Plan()
	b, _ := json.Marshal(p)
	path := filepath.Join(root, "profile.json")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	storeRoot := filepath.Join(root, "client")
	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() { // Bootstrap is read before sending protected requests; keep token output out of diagnostics.
		err := run(ctx, []string{"client-api", "-data-dir", storeRoot, "-profile", path}, writer)
		writer.CloseWithError(err)
		done <- err
	}()
	t.Cleanup(func() { // Join API storage/HTTP ownership before the temporary fixture directory is removed.
		cancel()
		reader.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("fresh API cleanup timeout")
		}
	})
	var bootstrap struct {
		URL   string `json:"api_url"`
		Token string `json:"session_token"`
	}
	if err := json.NewDecoder(reader).Decode(&bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(storeRoot, "client.json")); !os.IsNotExist(err) {
		t.Fatal("launch implicitly generated key")
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "POST", "/api/v1/client-keys", "create", `{"schema_version":1,"request_id":"create","client_id":"fresh"}`)
	apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "GET", "/api/v1/client-keys/fresh/public", "public", "")
	store, err := control.OpenStore(storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Client()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = control.LoadKey(id.KeyRef, nil); err != nil {
		t.Fatal("generated key unusable", err)
	}
}
