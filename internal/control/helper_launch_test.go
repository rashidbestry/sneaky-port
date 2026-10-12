package control

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/testutil"
)

// TestHelperPathValidation rejects relative/control text while preserving quoted absolute file names.
func TestHelperPathValidation(t *testing.T) {
	for _, path := range []string{"./tool", "~/tool", "/tmp/tool\ncommand", "/tmp/tool\x00", strings.Repeat("/", 1025)} {
		if _, err := helperLaunchCommand(path); err == nil {
			t.Fatal("invalid executable path accepted")
		}
	}
	if _, err := helperLaunchCommand("/tmp/copied folder/tool'with quote"); err != nil {
		t.Fatal("valid quoted file path rejected", err)
	}
}

// TestHelperStartupDiagnostics bounds private stderr and exposes only fixed actionable classifications.
func TestHelperStartupDiagnostics(t *testing.T) {
	private := "do-not-reflect-private-data"
	for _, fault := range []struct{ stderr, want string }{{"unimplemented or unknown command: " + private, "latest Linux"}, {"exec format error: " + private, "architecture"}, {"permission denied: " + private, "permission"}, {"no space left on device: " + private, "disk space"}, {private, "startup logs"}} {
		err := helperStartupFailure(fault.stderr, io.EOF)
		if !strings.Contains(err.Error(), fault.want) || strings.Contains(err.Error(), private) {
			t.Fatal("unsafe or unhelpful startup classification")
		}
	}
	d := &helperDiagnostics{}
	bytes := []byte(strings.Repeat("x", 10000))
	if n, err := d.Write(bytes); n != len(bytes) || err != nil || len(d.Text()) != 8192 {
		t.Fatal("stderr retention/backpressure bound failed")
	}
}

// TestSSHCopiedExecutableRecovery retains authenticated trust through failed launch and reuses an explicit path.
func TestSSHCopiedExecutableRecovery(t *testing.T) {
	const path = "/home/fixture/copied folder/server'build"
	wanted, _ := helperLaunchCommand(path)
	f := testutil.StartSSHCommand(t, func(ctx context.Context, command string, ch ssh.Channel) error { /* Default lookup models a copied executable outside PATH; the exact quoted path uses the real helper. */
		if command != wanted {
			io.WriteString(ch, "{\"schema_version\":1,\"startup_error\":\"missing_executable\"}\n")
			return nil
		}
		return DashboardServer(ctx, ch, ch)
	})
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := NewSSHDashboard(store)
	if err = d.SetHelperBinary(testutil.DashboardPayload()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	q := SSHConnectRequest{Address: f.IP, Port: f.Port, User: "sneak", Password: "fixture-password", Trust: f.Fingerprint}
	if status, err := d.Connect(ctx, q); err == nil || !strings.Contains(err.Error(), "Server executable") || status.State == "connected" {
		t.Fatal("missing PATH executable was not explained", err)
	}
	q.Trust = ""
	if status, err := d.Connect(ctx, q); err == nil || status.State == "trust_required" || f.Authentications.Load() != 2 {
		t.Fatal("accepted SSH trust was forgotten after helper failure", err, status.State)
	}
	q.Executable = path
	status, err := d.Connect(ctx, q)
	if err != nil || status.State != "connected" || status.Executable != path {
		t.Fatal("copied executable recovery failed", err)
	}
}

// TestSSHStartupFailureClasses keeps copied-permission and old-build failures separate from host trust.
func TestSSHStartupFailureClasses(t *testing.T) {
	for _, test := range []struct{ name, body, stderr, want string }{{"permission", "{\"schema_version\":1,\"startup_error\":\"not_executable\"}\n", "", "not executable"}, {"old-build", "", "unimplemented or unknown command \"dashboard-server\"", "latest Linux"}, {"malformed", "{\"schema_version\":1}\n", "", "incompatible startup"}, {"banner", "Welcome to a shell\n", "", "unexpected text"}} {
		t.Run(test.name, func(t *testing.T) { /* Return real SSH stdout/stderr failure channels; no raw remote diagnostics may escape. */
			f := testutil.StartSSHCommand(t, func(_ context.Context, _ string, ch ssh.Channel) error { /* Model bounded remote launcher/runtime failures without an executable substitute. */
				io.WriteString(ch, test.body)
				io.WriteString(ch.Stderr(), test.stderr)
				return errors.New("fixture stopped")
			})
			store, err := OpenStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			d := NewSSHDashboard(store)
			if err = d.SetHelperBinary(testutil.DashboardPayload()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(d.Close)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err = d.Connect(ctx, SSHConnectRequest{Address: f.IP, Port: f.Port, User: "sneak", Password: "fixture-password", Trust: f.Fingerprint, Executable: "/fixture/helper"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatal("incorrect failure stage", err)
			}
		})
	}
}
