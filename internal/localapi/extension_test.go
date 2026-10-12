package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/control"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
	"sneaky-port/internal/testutil/extension"
)

// extensionAPI creates isolated real HTTP/SSH/framed-TCP endpoints with startup-only registration.
// IPv4/IPv6 loopback and API-only marking do not establish a new production carrier or native support.
func extensionAPI(t *testing.T, ip, family, network string) (*Server, *http.Client) {
	t.Helper()
	serverStore, err := control.OpenStore(filepath.Join(t.TempDir(), "server"))
	if err != nil {
		t.Fatal(err)
	}
	clientStore, err := control.OpenStore(filepath.Join(t.TempDir(), "client"))
	if err != nil {
		t.Fatal(err)
	}
	for _, store := range []*control.Store{serverStore, clientStore} {
		store.Adapters = adapter.Default()
		if err := extension.Register(store.Adapters); err != nil {
			t.Fatal(err)
		}
	}
	ln, err := net.Listen(network, net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Release the fixture listener after joined SSH shutdown.
		ln.Close()
	})
	policy := control.ServerPolicy{Version: 1, ControlListen: ln.Addr().String(), DataBindIP: ip, PortMin: 1, PortMax: 65535, SetupTimeout: 2e9, ShutdownTimeout: 2e9, AuthPoll: 20e6, MaxConnections: 8, MaxAuthTries: 3, MaxControlRequests: 100, MaxBindAttempts: 16, MaxQueueBytes: 1 << 20, MaxLedgerMessages: 1000, MaxEventBytes: 1 << 20, MaxFileBytes: 2 << 20}
	bundle, err := serverStore.InitServer("server", ip, ln.Addr().(*net.TCPAddr).Port, policy, "")
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
	hostID, err := serverStore.Server()
	if err != nil {
		t.Fatal(err)
	}
	host, err := control.LoadKey(hostID.KeyRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	sshServer, err := control.NewServer(serverStore, host)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { // Serve authenticated control on the owned loopback listener only.
		done <- sshServer.Serve(ctx, ln)
	}()
	t.Cleanup(func() { // Bound SSH shutdown after API/run cleanup has joined.
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("extension SSH cleanup timeout")
		}
	})
	identity, err := clientStore.Client()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := control.LoadKey(identity.KeyRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	port, err := net.Listen(network, net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Fatal(err)
	}
	p := testutil.Plan()
	p.Adapter, p.Family, p.ServerAddress = extension.ID, family, ip
	p.Options = map[string]json.RawMessage{"enabled": json.RawMessage("true")}
	p.DestinationPort = port.Addr().(*net.TCPAddr).Port
	port.Close()
	p.Warmup, p.Phases[0].Duration, p.Grace, p.Deadline = 40e6, 160e6, 100e6, 80e6
	p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 50, 25
	p.Limits.QueueMessages, p.Limits.MaxLag, p.Limits.HeartbeatInterval = 128, 50e6, 20e6
	s, err := ListenWithStore(net.JoinHostPort(ip, "0"), p, clientStore, func(ctx context.Context, plan profile.Plan) (control.RunResult, error) { // Execute the unchanged authenticated coordinator with the store registry.
		return control.Run(ctx, clientStore, signer, plan)
	})
	if err != nil {
		t.Fatal(err)
	}
	apiDone := make(chan error, 1)
	go func() { // Serve the real protected HTTP boundary independently of finite traffic.
		apiDone <- s.Serve(ctx, 2*time.Second)
	}()
	t.Cleanup(func() { // Cancel/join HTTP and any run before releasing control/data listeners.
		closeCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := s.Close(closeCtx); err != nil {
			t.Error(err)
		}
		if err := <-apiDone; err != nil {
			t.Error(err)
		}
	})
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil}}
	t.Cleanup(client.CloseIdleConnections)
	return s, client
}

// extensionHTTP sends actual authenticated origin-form HTTP, retaining raw exports for exact checks.
func extensionHTTP(t *testing.T, s *Server, c *http.Client, method, path, id string, body any, code int) []byte {
	t.Helper()
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r, err := http.NewRequest(method, s.URL()+path, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+s.Token())
	r.Header.Set("Origin", s.URL())
	r.Header.Set("X-Request-ID", id)
	r.Header.Set("Content-Type", "application/json")
	response, err := c.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	out, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != code {
		t.Fatalf("%s %s: %d want %d: %s %v", method, path, response.StatusCode, code, out, err)
	}
	if bytes.Contains(out, []byte(s.Token())) {
		t.Fatal("token leaked")
	}
	return out
}

// extensionStatus extracts the common run handle/completeness independently of the fixture identity.
func extensionStatus(t *testing.T, b []byte) Status {
	t.Helper()
	var out struct {
		Result Status `json:"result"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.Result
}

// TestAdapterExtensionWorkflow crosses common config, HTTP, SSH, accounting, reports and cleanup.
// The fixture is TCP-backed with distinct HMAC identity; no TLS/QUIC/DNS support is implied.
func TestAdapterExtensionWorkflow(t *testing.T) {
	for _, family := range []struct{ ip, name, network string }{{"127.0.0.1", "ipv4", "tcp4"}, {"::1", "ipv6", "tcp6"}} {
		t.Run(family.name, func(t *testing.T) { // Run the same contract in both declared address families.
			s, c := extensionAPI(t, family.ip, family.name, family.network)
			descriptors := extensionHTTP(t, s, c, "GET", "/api/v1/adapters", "adapters", nil, 200)
			if !bytes.Contains(descriptors, []byte(extension.ID)) || !bytes.Contains(descriptors, []byte(`"framing_bytes":4`)) {
				t.Fatal("extension discovery missing")
			}
			p := s.plan
			for _, invalid := range []string{"option", "frame", "version", "unknown"} {
				b, _ := json.Marshal(p)
				bad, _ := profile.Decode(bytes.NewReader(b))
				switch invalid {
				case "option":
					bad.Options["enabled"] = json.RawMessage("false")
				case "frame":
					bad.Phases[0].Client.ContentBytes = extension.MaxFrame - probe.Overhead + 1
				case "version":
					bad.Contract = 2
				case "unknown":
					bad.Adapter = "unregistered"
				}
				extensionHTTP(t, s, c, "POST", "/api/v1/profiles/validate", invalid, map[string]any{"schema_version": 1, "request_id": invalid, "profile_revision": "", "profile": bad}, 422)
			}
			extensionHTTP(t, s, c, "PUT", "/api/v1/profiles/"+p.ID, "save", map[string]any{"schema_version": 1, "request_id": "save", "profile_revision": "", "profile": p}, 200)
			ids := []string{}
			for i := 0; i < 3; i++ {
				if i == 2 {
					// Extend only the cancellation fixture so persisted attempt detection cannot race natural completion.
					p.Phases[0].Duration = 1e9
					extensionHTTP(t, s, c, "PUT", "/api/v1/profiles/"+p.ID, "save-cancel", map[string]any{"schema_version": 1, "request_id": "save-cancel", "profile_revision": s.Revision(), "profile": p}, 200)
				}
				id := fmt.Sprintf("start-%d", i)
				body := mutation{1, id, p.ID, s.Revision()}
				state := extensionStatus(t, extensionHTTP(t, s, c, "POST", "/api/v1/runs", id, body, 200))
				if i == 2 {
					// Cancel after persisted attempts prove traffic started; allow a generous setup bound.
					end := time.Now().Add(5 * time.Second)
					observed := false
					for time.Now().Before(end) {
						dirs, _ := os.ReadDir(filepath.Join(s.store.Root, "reports"))
						for _, dir := range dirs {
							if !dir.IsDir() || dir.Name() == ids[0] || dir.Name() == ids[1] {
								continue
							}
							b, _ := os.ReadFile(filepath.Join(s.store.Root, "reports", dir.Name(), "events.jsonl"))
							if bytes.Contains(b, []byte(`"Kind":"attempt"`)) {
								observed = true
							}
						}
						if observed {
							break
						}
						time.Sleep(2 * time.Millisecond)
					}
					if !observed {
						t.Fatal("cancel fixture never reached data attempts")
					}
					extensionHTTP(t, s, c, "POST", "/api/v1/runs/"+state.ID+"/cancel", "cancel", mutation{Version: 1, RequestID: "cancel"}, 200)
				}
				end := time.Now().Add(6 * time.Second)
				for time.Now().Before(end) {
					state = extensionStatus(t, extensionHTTP(t, s, c, "GET", "/api/v1/runs/"+state.ID, "poll", nil, 200))
					if state.State != "executing" && state.State != "cancelling" {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if i == 2 {
					if state.State != "partial" || state.Complete == nil || *state.Complete {
						t.Fatalf("cancel lost partial accounting: %+v", state)
					}
				} else if state.State != "completed" || state.Complete == nil || !*state.Complete {
					t.Fatalf("extension run failed: %+v", state)
				}
				retry := extensionStatus(t, extensionHTTP(t, s, c, "POST", "/api/v1/runs", id, body, 200))
				if retry.ID != state.ID {
					t.Fatal("retry duplicated traffic")
				}
				reportID := state.MeasurementID + "-client"
				path := filepath.Join(s.store.Root, "reports", reportID)
				r, err := s.store.SavedReport(reportID)
				if err != nil || r.Verification != "matched" || r.Manifest.Plan.Adapter != extension.ID || string(r.Manifest.Plan.Options["enabled"]) != "true" {
					t.Fatal("extension replay lost identity/options", err)
				}
				if _, err := control.ReplayReport(path, 4<<20); err == nil {
					t.Fatal("unregistered report silently substituted")
				}
				if i < 2 && (r.Reconciliation == nil || r.Reconciliation.Status != "reconciled" || r.ServerReport == nil || r.ServerReport.Manifest.Plan.Adapter != extension.ID || r.Reconciliation.Client.Summary.Received == nil) {
					t.Fatal("extension remote accounting missing")
				}
				writes := 0
				for _, raw := range r.Evidence.Records {
					var record engine.Record
					if err := json.Unmarshal(raw, &record); err != nil {
						t.Fatal(err)
					}
					if record.Kind == "write" && record.Error == "" {
						want := probe.Overhead + 4
						if record.FrameKind == probe.Data {
							want += p.Phases[0].Client.ContentBytes
						}
						if record.Bytes != profile.Decimal(want) {
							t.Fatalf("wrong registered framing count: %d want %d", record.Bytes, want)
						}
						writes++
					}
				}
				if writes == 0 {
					t.Fatal("extension never wrote a complete frame")
				}
				extensionHTTP(t, s, c, "GET", "/api/v1/reports/"+reportID, "report", nil, 200)
				extensionHTTP(t, s, c, "GET", "/api/v1/reports/"+reportID+"/diagnostics?endpoint=local&scope=main", "diagnostic", nil, 200)
				for _, format := range []string{"json", "jsonl", "csv"} {
					out := extensionHTTP(t, s, c, "GET", "/api/v1/reports/"+reportID+"/export?format="+format, "export", nil, 200)
					var want bytes.Buffer
					if err := control.ExportReport(&want, r, format); err != nil || !bytes.Equal(out, want.Bytes()) {
						t.Fatal("common export differs", format, err)
					}
				}
				ids = append(ids, reportID)
				if _, err := os.Stat(filepath.Join(s.store.Root, "active-run.lock")); !os.IsNotExist(err) {
					t.Fatal("reservation leaked")
				}
			}
			files := map[string][]byte{}
			for _, name := range []string{"manifest.json", "events.jsonl", "summary.json", "server-transfer.json", "remote-server/manifest.json", "remote-server/events.jsonl", "remote-server/summary.json"} {
				b, err := os.ReadFile(filepath.Join(s.store.Root, "reports", ids[0], filepath.FromSlash(name)))
				if err != nil {
					t.Fatal(err)
				}
				files[name] = b
			}
			imported, err := s.store.ImportReport("extension-import", files)
			original, originalErr := s.store.SavedReport(ids[0])
			if err != nil || originalErr != nil || !reflect.DeepEqual(imported.Evidence, original.Evidence) {
				t.Fatal("extension import/replay differs", err, originalErr)
			}
			extensionHTTP(t, s, c, "POST", "/api/v1/reports/compare", "compare", map[string]any{"schema_version": 1, "request_id": "compare", "pairs": []control.TrialPair{{A: ids[0], B: ids[1]}}, "axes": []string{}, "endpoint": "local", "scope": "main"}, 200)
			port, err := net.Listen(family.network, net.JoinHostPort(family.ip, fmt.Sprint(p.DestinationPort)))
			if err != nil {
				t.Fatal("extension listener leaked", err)
			}
			port.Close()
			if strings.Contains(imported.Integrity, "authenticated") {
				t.Fatal("offline import invented authentication")
			}
		})
	}
}
