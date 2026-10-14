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
	"sneaky-port/internal/measure"
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

// extensionEndpointState reports bounded endpoint status and half-open receiver populations.
// Raw observations and their original independently timed cutoff remain authoritative.
func extensionEndpointState(r *control.ReplayedReport) string {
	if r == nil {
		return "missing"
	}
	outside := 0
	for _, at := range r.Evidence.Receiver.Arrivals {
		if at >= r.Evidence.Receiver.Cutoff {
			outside++
		}
	}
	return fmt.Sprintf("status=%s verification=%s adapter=%s complete=%t generation=%d observation=%d receiver_arrivals=%d receiver_outside=%d", r.Status, r.Verification, r.Manifest.Plan.Adapter, r.Evidence.Complete, r.Evidence.GenerationCutoff, r.Evidence.ObservationCutoff, len(r.Evidence.Receiver.Arrivals), outside)
}

// extensionDirectionAccounting checks raw identities and exact delivery at independent local cutoffs.
// Only a matched cross-window attempt/receipt can justify unknown delivery in a completed live fixture.
func extensionDirectionAccounting(origin, peer *control.ReplayedReport, derived *control.LocalRun) (bool, error) {
	if derived == nil {
		return false, fmt.Errorf("missing directional reconciliation")
	}
	if derived.Summary.TransportPacketLoss != nil {
		return false, fmt.Errorf("directional accounting invented transport packet loss")
	}
	entries := make(map[measure.Identity]measure.Entry, len(origin.Evidence.Entries))
	for _, entry := range origin.Evidence.Entries {
		entries[entry.Identity] = entry
	}
	arrivals := make(map[measure.Identity]engine.Record, len(peer.Evidence.Receiver.Arrivals))
	for _, raw := range peer.Evidence.Records {
		var record engine.Record
		if err := json.Unmarshal(raw, &record); err != nil {
			return false, err
		}
		if record.Kind == "arrival" && record.FrameKind == probe.Data {
			arrivals[record.Identity] = record
		}
	}
	matched := func(id measure.Identity, entry measure.Entry, at uint64) bool { // Require an unchanged complete-frame capture before using either endpoint's cutoff.
		arrival, ok := arrivals[id]
		return ok && entry.Attempt != nil && arrival.At == profile.Decimal(at) && arrival.Attempt == *entry.Attempt && arrival.Epoch == origin.Evidence.Epoch && arrival.Direction == origin.Evidence.Direction
	}
	witness := false
	for id, at := range peer.Evidence.Receiver.Arrivals {
		if at >= uint64(peer.Evidence.ObservationCutoff) {
			continue
		}
		entry, ok := entries[id]
		if !ok || !matched(id, entry, at) {
			return false, fmt.Errorf("eligible receiver identity does not match original attempt: %+v", id)
		}
		if *entry.Attempt >= origin.Evidence.ObservationCutoff {
			witness = true
		}
	}
	for _, entry := range origin.Evidence.Entries {
		if entry.Receipt == nil || *entry.Receipt >= origin.Evidence.ObservationCutoff {
			continue
		}
		at, ok := peer.Evidence.Receiver.Arrivals[entry.Identity]
		if !ok || !matched(entry.Identity, entry, at) {
			return false, fmt.Errorf("in-window receipt lacks matching retained receiver capture: %+v", entry.Identity)
		}
		if at >= uint64(peer.Evidence.ObservationCutoff) {
			witness = true
		}
	}
	if witness {
		if derived.Summary.Received != nil || derived.Summary.MissingAtReceiver != nil {
			return true, fmt.Errorf("cross-window identities became known directional delivery")
		}
		return true, nil
	}
	var received, missing profile.Decimal
	for _, entry := range origin.Evidence.Entries {
		if entry.Identity.Lifecycle != 2 || entry.Scheduled >= origin.Evidence.ObservationCutoff || entry.Attempt == nil || *entry.Attempt >= origin.Evidence.ObservationCutoff || entry.State == "skipped_local" {
			continue
		}
		if at, ok := peer.Evidence.Receiver.Arrivals[entry.Identity]; ok && at < uint64(peer.Evidence.ObservationCutoff) {
			received++
		} else if entry.State == "written" && entry.WriteCompleted != nil && *entry.WriteCompleted < origin.Evidence.ObservationCutoff {
			missing++
		}
	}
	if derived.Summary.Received == nil || *derived.Summary.Received != received || derived.Summary.MissingAtReceiver == nil || *derived.Summary.MissingAtReceiver != missing {
		return false, fmt.Errorf("compatible direction lost exact delivery: received=%d missing=%d derived=%+v", received, missing, derived.Summary)
	}
	return false, nil
}

// extensionAccountingWarning independently names the one proven directional reconciliation warning.
// It follows only validated raw captures; ambiguous write completion does not imply missing delivery.
func extensionAccountingWarning(origin, peer *control.ReplayedReport, role string, witness bool) string {
	if witness {
		for _, entry := range origin.Evidence.Entries {
			at, ok := peer.Evidence.Receiver.Arrivals[entry.Identity]
			if ok && at < uint64(peer.Evidence.ObservationCutoff) && entry.Attempt != nil && *entry.Attempt >= origin.Evidence.ObservationCutoff {
				return role + ": receiver contains unmatched or outside-cutoff identity"
			}
		}
		return role + ": origin receipt contradicts complete receiver ledger"
	}
	for _, entry := range origin.Evidence.Entries {
		if entry.Attempt != nil && (entry.State != "written" || entry.WriteCompleted == nil || *entry.WriteCompleted >= origin.Evidence.ObservationCutoff) {
			return role + ": failed/pending writes are excluded from missing-data claims; observed arrivals remain distinct"
		}
	}
	return ""
}

// extensionRemoteAccounting requires successful registered replay and precise per-direction accounting.
// Hosted scheduling may produce a proven independent-cutoff incompatibility; arbitrary unknowns still fail.
func extensionRemoteAccounting(t *testing.T, r *control.ReplayedReport, run int) {
	t.Helper()
	peer, reconciliation := r.ServerReport, r.Reconciliation
	if r.Status != "completed" || r.Verification != "matched" || !r.Evidence.Complete || !r.Evidence.Receiver.Complete || r.Evidence.Receiver.Cutoff != uint64(r.Evidence.ObservationCutoff) || peer == nil || reconciliation == nil || r.TransferReceipt == nil || r.TransferReceipt.Status != "verified" || peer.Status != "completed" || peer.Verification != "matched" || !peer.Evidence.Complete || !peer.Evidence.Receiver.Complete || peer.Evidence.Receiver.Cutoff != uint64(peer.Evidence.ObservationCutoff) || peer.Manifest.Plan.Adapter != extension.ID || !reflect.DeepEqual(r.Manifest.Plan, peer.Manifest.Plan) || !reflect.DeepEqual(r.Manifest.Settings, peer.Manifest.Settings) || r.Manifest.RunID != peer.Manifest.RunID || r.Manifest.SessionID != peer.Manifest.SessionID || r.Manifest.PlanHash != peer.Manifest.PlanHash || r.Evidence.Direction != 1 || peer.Evidence.Direction != 2 || r.Evidence.Epoch != 1 || peer.Evidence.Epoch != 1 || r.Evidence.PeerEpoch != 1 || peer.Evidence.PeerEpoch != 1 {
		t.Fatalf("extension remote identity/accounting missing: run=%d client=[%s] peer=[%s] transfer=%+v reconciliation=%+v", run, extensionEndpointState(r), extensionEndpointState(peer), r.TransferReceipt, reconciliation)
	}
	clientWitness, clientErr := extensionDirectionAccounting(r, peer, reconciliation.Client)
	serverWitness, serverErr := extensionDirectionAccounting(peer, r, reconciliation.Server)
	if clientErr != nil || serverErr != nil {
		t.Fatalf("extension remote accounting differs: run=%d client_error=%v server_error=%v client=[%s] peer=[%s] reconciliation=%+v", run, clientErr, serverErr, extensionEndpointState(r), extensionEndpointState(peer), reconciliation)
	}
	warnings := []string{}
	for _, warning := range []string{extensionAccountingWarning(r, peer, "client", clientWitness), extensionAccountingWarning(peer, r, "server", serverWitness)} {
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	if !reflect.DeepEqual(reconciliation.Warnings, warnings) {
		t.Fatalf("extension reconciliation warning differs: actual=%v want=%v", reconciliation.Warnings, warnings)
	}
	if clientWitness || serverWitness {
		if reconciliation.Status != "unknown" || reconciliation.Reason != "unmatched receiver identities; directional delivery remains unknown where incompatible" {
			t.Fatalf("independent cutoff incompatibility lost: %+v", reconciliation)
		}
		t.Logf("run=%d exact independent-cutoff witness client=%t server=%t; complete registered endpoint records retained with unknown affected delivery", run, clientWitness, serverWitness)
	} else if reconciliation.Status != "reconciled" {
		t.Fatalf("compatible extension did not reconcile: %+v", reconciliation)
	}
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
				if i < 2 {
					extensionRemoteAccounting(t, r, i)
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
