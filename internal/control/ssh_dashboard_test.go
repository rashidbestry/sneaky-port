package control

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sneaky-port/internal/testutil"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSSHDashboardWorkflow covers real password SSH/helper/control with direct UDP/TCP and exact plans.
func TestSSHDashboardWorkflow(t *testing.T) {
	f := testutil.StartSSH(t, DashboardServer)
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := NewSSHDashboard(store)
	t.Cleanup(d.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	q := SSHConnectRequest{Version: 1, Address: f.IP, Port: f.Port, User: "sneak", Password: "fixture-password"}
	status, err := d.Connect(ctx, q)
	if err != nil || status.State != "trust_required" || status.Fingerprint != f.Fingerprint || f.Authentications.Load() != 0 {
		t.Fatal("host trust must precede password delivery", err, status.State)
	}
	q.Trust = "SHA256:wrong"
	status, err = d.Connect(ctx, q)
	if err != nil || status.State != "trust_required" || f.Authentications.Load() != 0 {
		t.Fatal("wrong fingerprint accepted")
	}
	q.Trust = f.Fingerprint
	status, err = d.Connect(ctx, q)
	if err != nil || status.State != "connected" {
		t.Fatal("SSH helper connection failed", err)
	}
	for _, protocol := range []string{"udp", "tcp"} {
		p := testutil.Plan()
		p.Adapter = protocol
		if protocol == "tcp" {
			p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
		}
		if protocol == "tcp" {
			listener, err := net.Listen("tcp4", "0.0.0.0:0")
			if err != nil {
				t.Fatal(err)
			}
			p.DestinationPort = listener.Addr().(*net.TCPAddr).Port
			listener.Close()
		} else {
			listener, err := net.ListenPacket("udp4", "0.0.0.0:0")
			if err != nil {
				t.Fatal(err)
			}
			p.DestinationPort = listener.LocalAddr().(*net.UDPAddr).Port
			listener.Close()
		}
		p.PairingRef = status.PairingRef
		p.ServerAddress = f.IP
		p.Phases[0].Client.ContentBytes = 41
		p.Phases[0].Server.ContentBytes = 53
		p.Phases[0].Duration = 50_000_000
		p.Phases[0].Client.RateNumerator = 60
		p.Phases[0].Server.RateNumerator = 60
		p.Deadline = 20_000_000
		p.Grace = 20_000_000
		result, err := d.Run(ctx, p)
		if err != nil || result.Local == nil || !result.Local.Complete || !result.Readiness.MeasurementStarted {
			t.Fatal("negotiated finite run failed", protocol, err)
		}
		hash, _ := p.Hash()
		if result.Readiness.PlanHash != fmtHash(hash[:]) {
			t.Fatal("server accepted another plan")
		}
		if result.Reconciliation == nil || result.Remote == nil || result.Remote.Status != "verified" {
			t.Fatal("authenticated helper report transfer missing", protocol)
		}
	}
	unsupported := testutil.Plan()
	unsupported.PairingRef = status.PairingRef
	unsupported.Options = map[string]json.RawMessage{"unsupported": json.RawMessage("true")}
	if result, err := d.Run(ctx, unsupported); err == nil || result.Readiness.MeasurementStarted || result.Local != nil {
		t.Fatal("unsupported tweaks started measurement")
	}
	overLimit := testutil.Plan()
	overLimit.PairingRef = status.PairingRef
	overLimit.Limits.SetupTimeout = 31_000_000_000
	if result, err := d.Run(ctx, overLimit); err == nil || result.Readiness.MeasurementStarted || result.Local != nil {
		t.Fatal("server policy rejection started measurement")
	}
	// Public trust/key/report stores must never contain the Linux account password.
	err = filepath.WalkDir(store.Root, func(path string, entry os.DirEntry, err error) error { /* Audit only this isolated application store's bounded fixture files. */
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		bytes, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(bytes), q.Password) {
			t.Error("password persisted")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	if d.Status().State != "disconnected" {
		t.Fatal("helper did not disconnect")
	}
	q.Trust = ""
	status, err = d.Connect(ctx, q)
	if err != nil || status.State != "connected" {
		t.Fatal("pinned reconnect failed", err)
	}
}

// fmtHash keeps the fixture's exact canonical digest comparison independent of the SSH handshake.
func fmtHash(bytes []byte) string {
	const digits = "0123456789abcdef"
	text := make([]byte, len(bytes)*2)
	for i, b := range bytes {
		text[2*i] = digits[b>>4]
		text[2*i+1] = digits[b&15]
	}
	return string(text)
}

// TestSSHDashboardAuthenticationFailures keeps first trust, wrong passwords and missing helpers explicit.
func TestSSHDashboardAuthenticationFailures(t *testing.T) {
	f := testutil.StartSSH(t, DashboardServer)
	missing := testutil.StartSSH(t, nil)
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := NewSSHDashboard(store)
	t.Cleanup(d.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	q := SSHConnectRequest{Version: 1, Address: f.IP, Port: f.Port, User: "sneak", Password: "wrong-password", Trust: f.Fingerprint}
	if _, err = d.Connect(ctx, q); err == nil || strings.Contains(err.Error(), q.Password) || d.Status().State != "disconnected" {
		t.Fatal("wrong password produced a connection or secret-bearing error")
	}
	q.Port = missing.Port
	q.Password = "fixture-password"
	q.Trust = missing.Fingerprint
	if _, err = d.Connect(ctx, q); err == nil || d.Status().State != "disconnected" {
		t.Fatal("missing helper silently substituted")
	}
}

// TestSSHDashboardChangedHostRequiresReplacement refuses changed host keys before credential delivery.
func TestSSHDashboardChangedHostRequiresReplacement(t *testing.T) {
	first := testutil.StartSSH(t, DashboardServer)
	second := testutil.StartSSH(t, DashboardServer)
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := NewSSHDashboard(store)
	t.Cleanup(d.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	q := SSHConnectRequest{Version: 1, Address: first.IP, Port: first.Port, User: "sneak", Password: "fixture-password", Trust: first.Fingerprint}
	if _, err = d.Connect(ctx, q); err != nil {
		t.Fatal(err)
	}
	d.Close()
	firstAddress := net.JoinHostPort(first.IP, strconv.Itoa(first.Port))
	secondAddress := net.JoinHostPort(second.IP, strconv.Itoa(second.Port))
	path, err := d.hostPath(firstAddress, false)
	if err != nil {
		t.Fatal(err)
	}
	var previous sshHostPin
	if err = readObject(path, &previous); err != nil {
		t.Fatal(err)
	}
	path, err = d.hostPath(secondAddress, true)
	if err != nil {
		t.Fatal(err)
	}
	previous.Address = secondAddress
	if err = d.store.writeAtomic(path, previous); err != nil {
		t.Fatal(err)
	}
	q.Port = second.Port
	q.Trust = second.Fingerprint
	status, err := d.Connect(ctx, q)
	if err != nil || status.State != "trust_required" || status.Previous != first.Fingerprint || second.Authentications.Load() != 0 {
		t.Fatal("changed host got credentials without replacement")
	}
	q.Replace = true
	status, err = d.Connect(ctx, q)
	if err != nil || status.State != "connected" {
		t.Fatal("explicit host replacement failed", err)
	}
}

// TestSSHDashboardCancellation closes a live control tunnel and joins all helper/finite ownership.
func TestSSHDashboardCancellation(t *testing.T) {
	f := testutil.StartSSH(t, DashboardServer)
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := NewSSHDashboard(store)
	t.Cleanup(d.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	status, err := d.Connect(ctx, SSHConnectRequest{Version: 1, Address: f.IP, Port: f.Port, User: "sneak", Password: "fixture-password", Trust: f.Fingerprint})
	if err != nil {
		t.Fatal(err)
	}
	p := testutil.Plan()
	p.PairingRef = status.PairingRef
	p.Phases[0].Duration = 1_000_000_000
	p.Phases[0].Client.RateNumerator = 60
	p.Phases[0].Server.RateNumerator = 60
	packet, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p.DestinationPort = packet.LocalAddr().(*net.UDPAddr).Port
	packet.Close()
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { /* The real finite runner must unwind tunnel/calibration/data/report ownership on cancellation. */
		_, err := d.Run(runCtx, p)
		done <- err
	}()
	time.AfterFunc(50*time.Millisecond, stop)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled run reported success")
		}
	case <-ctx.Done():
		t.Fatal("cancelled SSH run failed to join")
	}
	d.Close()
	if _, err = os.Stat(filepath.Join(store.Root, "active-run.lock")); !os.IsNotExist(err) {
		t.Fatal("run reservation leaked")
	}
}
