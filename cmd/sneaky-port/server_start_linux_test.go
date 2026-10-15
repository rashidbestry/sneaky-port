//go:build linux

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
	"sneaky-port/internal/profile"
)

// startAutomaticServer runs the real server command against only the test's private store and selected loopback port.
func startAutomaticServer(t *testing.T, root, listen string) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { // Join the actual command's listener and every connection when the test cancels it.
		done <- run(ctx, []string{"server", "-data-dir", root, "-listen", listen}, &bytes.Buffer{})
	}()
	stopped := false
	stop := func() { // Stop only this command and require its owned listener/connections to join.
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error("automatic server shutdown", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("automatic server did not join")
		}
	}
	t.Cleanup(stop)
	deadline := time.Now().Add(10 * time.Second)
	for {
		connection, err := net.DialTimeout("tcp4", listen, 100*time.Millisecond)
		if err == nil {
			connection.Close()
			return stop
		}
		select {
		case err := <-done:
			stopped = true
			cancel()
			t.Fatal("automatic server stopped before listening", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("automatic server listener unavailable", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestAutomaticServerClientPlans requires trusted enrollment and executes client-selected UDP/TCP plans across a restart.
// No server policy JSON is supplied; raw/replayed results stay authoritative even if scheduling yields skips or empty RTT populations.
func TestAutomaticServerClientPlans(t *testing.T) {
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listen := probe.Addr().String()
	probe.Close()
	root := filepath.Join(t.TempDir(), "server")
	stop := startAutomaticServer(t, root, listen)
	server, err := control.OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := server.Server()
	if err != nil {
		t.Fatal(err)
	}
	clientRoot := filepath.Join(t.TempDir(), "client")
	client, err := control.OpenStore(clientRoot)
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := client.InitClient("auto-client", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = client.ImportPairing("lab-server", identity.Bundle, false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "examples", "readiness-udp.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := profile.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	data, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p.DestinationPort = data.LocalAddr().(*net.UDPAddr).Port
	data.Close()
	p.Phases[0].Duration, p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 600e6, 20, 20
	p.Calibration.Cadence = 200e6
	planPath := filepath.Join(t.TempDir(), "plan.json")
	b, err = json.Marshal(p)
	if err != nil || os.WriteFile(planPath, b, 0600) != nil {
		t.Fatal("client plan fixture", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	if err = run(ctx, []string{"client-ready", "-data-dir", clientRoot, "-profile", planPath, "-hold-ns", "0"}, &bytes.Buffer{}); err == nil {
		t.Fatal("new unpaired client was automatically trusted")
	}
	if err = server.Enroll(enrollment); err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []string{"udp", "tcp"} {
		p.Adapter, p.Options = carrier, map[string]json.RawMessage{}
		if carrier == "tcp" {
			p.Options["tcp_no_delay"] = json.RawMessage("true")
		}
		b, err = json.Marshal(p)
		if err != nil || os.WriteFile(planPath, b, 0600) != nil {
			t.Fatal("client plan fixture", err)
		}
		var out bytes.Buffer
		if err = run(ctx, []string{"client-run", "-data-dir", clientRoot, "-profile", planPath}, &out); err != nil {
			t.Fatalf("client-selected %s plan: %v", carrier, err)
		}
		var result control.RunResult
		if err = json.Unmarshal(out.Bytes(), &result); err != nil || result.Status != "completed" || result.Remote == nil || result.Remote.Status != "verified" {
			t.Fatalf("client-selected %s result: %+v %v", carrier, result, err)
		}
		replay, err := control.ReplayReport(result.ReportPath, 64<<20)
		if err != nil || replay.Verification != "matched" || replay.Manifest.Plan.Adapter != carrier {
			t.Fatal("client plan/replay mismatch", err)
		}
		stop()
		stop = startAutomaticServer(t, root, listen)
		restarted, err := server.Server()
		if err != nil || restarted.Bundle.HostFingerprint != identity.Bundle.HostFingerprint {
			t.Fatal("server restart changed host identity", err)
		}
	}
}
