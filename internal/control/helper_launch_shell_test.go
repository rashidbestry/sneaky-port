package control

import (
	"context"
	"encoding/json"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sneaky-port/internal/testutil"
	"strings"
	"testing"
	"time"
)

// TestHelperLauncherLinuxShell executes actual POSIX lookup/permission/quoting rather than emulating exec.
func TestHelperLauncherLinuxShell(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native Linux shell fixture")
	}
	root := t.TempDir()
	target := filepath.Join(root, "server ' ; touch INJECTED ; #")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n[ \"$1\" = dashboard-server ] || exit 77\nprintf '%s\\n' '{\"invoked\":true}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ path, want string }{{target, "invoked"}, {filepath.Join(root, "missing"), "missing_executable"}} {
		command, err := helperLaunchCommand(test.path)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		process := exec.CommandContext(ctx, "/bin/sh", "-c", command)
		process.Dir = root
		output, _ := process.CombinedOutput()
		cancel()
		if !strings.Contains(string(output), test.want) {
			t.Fatal("native launcher failed", string(output))
		}
	}
	if _, err := os.Stat(filepath.Join(root, "INJECTED")); !os.IsNotExist(err) {
		t.Fatal("executable path was interpreted as shell code")
	}
	if err := os.Chmod(target, 0600); err != nil {
		t.Fatal(err)
	}
	command, _ := helperLaunchCommand(target)
	output, _ := exec.Command("/bin/sh", "-c", command).CombinedOutput()
	if !strings.Contains(string(output), "not_executable") {
		t.Fatal("native executable permission check failed")
	}
}

// TestHelperLauncherLinuxBinary starts the actual Linux CLI from a supplied isolated artifact and joins EOF cleanup.
func TestHelperLauncherLinuxBinary(t *testing.T) {
	binary := os.Getenv("DASHBOARD_NATIVE_BINARY")
	if runtime.GOOS != "linux" || binary == "" {
		t.Skip("explicit native Linux CLI artifact required")
	}
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := store.InitClient("shell-fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	command, err := helperLaunchCommand(binary)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	stdin, err := process.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process.Stderr = io.Discard
	if err = process.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	defer func() { /* Failed assertions still stop and join this private helper process. */
		stdin.Close()
		if !joined {
			cancel()
			process.Wait()
		}
	}()
	if err = json.NewEncoder(stdin).Encode(DashboardServerRequest{1, "127.0.0.1", enrollment}); err != nil {
		t.Fatal(err)
	}
	var reply DashboardServerReply
	if err = json.NewDecoder(stdout).Decode(&reply); err != nil || reply.Version != 1 || reply.Bundle.Validate() != nil {
		t.Fatal("actual CLI helper startup failed", err)
	}
	stdin.Close()
	err = process.Wait()
	joined = true
	if err != nil {
		t.Fatal("actual CLI helper EOF cleanup failed", err)
	}
}

// TestSSHActualLinuxCopiedHelper runs real SSH/exec/shell/helper/direct UDP from the supplied Linux file.
func TestSSHActualLinuxCopiedHelper(t *testing.T) { runActualLinuxHelper(t, false) }

// runActualLinuxHelper verifies native exec/upload, exact direct measurement and cleanup in an isolated temp root.
func runActualLinuxHelper(t *testing.T, upload bool) {
	t.Helper()
	binary := os.Getenv("DASHBOARD_NATIVE_BINARY")
	if runtime.GOOS != "linux" || binary == "" {
		t.Skip("explicit native Linux copied executable required")
	}
	root := t.TempDir()
	f := testutil.StartSSHCommand(t, func(ctx context.Context, command string, ch ssh.Channel) error { /* Use the actual remote shell/CLI subprocess while normal SSH channels stay fixture-owned. */
		process := exec.CommandContext(ctx, "/bin/sh", "-c", command)
		process.Env = append(os.Environ(), "TMPDIR="+root)
		process.Stdin = ch
		process.Stdout = ch
		process.Stderr = ch.Stderr()
		return process.Run()
	})
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := NewSSHDashboard(store)
	t.Cleanup(d.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	q := SSHConnectRequest{Address: f.IP, Port: f.Port, User: "sneak", Password: "fixture-password", Executable: binary}
	if upload {
		payload, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		if err = d.SetHelperBinary(payload); err != nil {
			t.Fatal(err)
		}
		q.Executable = ""
	}
	status, err := d.Connect(ctx, q)
	if err != nil || status.State != "connected" {
		t.Fatal("actual copied SSH helper failed", err)
	}
	p := testutil.Plan()
	p.PairingRef = status.PairingRef
	p.Phases[0].Duration = 50_000_000
	p.Phases[0].Client.RateNumerator = 60
	p.Phases[0].Server.RateNumerator = 60
	packet, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	p.DestinationPort = packet.LocalAddr().(*net.UDPAddr).Port
	packet.Close()
	result, err := d.Run(ctx, p)
	if err != nil || result.Local == nil || !result.Local.Complete || result.Remote == nil || result.Remote.Status != "verified" {
		t.Fatal("actual copied helper did not complete negotiated direct measurement/report transfer", err)
	}
	d.Close()
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("native helper or upload storage was not cleaned", err)
	}
}
