package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sneaky-port/internal/control"
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
	timer := time.AfterFunc(100*time.Millisecond, stop)
	defer timer.Stop()
	if err = run(canceled, args, &out); err == nil {
		t.Fatal("interrupted CLI exit succeeded")
	}
	if err = json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "partial" || result.Local == nil || result.Local.Complete {
		t.Fatal("partial CLI evidence missing")
	}
}
