package control

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/testutil"
)

// TestHelperUploadValidation rejects missing/foreign payloads and keeps caller mutations out of uploads.
func TestHelperUploadValidation(t *testing.T) {
	for _, binary := range [][]byte{nil, []byte("#!/bin/sh\n"), make([]byte, 64), make([]byte, helperBinaryMax+1)} {
		if _, err := helperUploadCommand(binary); err == nil {
			t.Fatal("invalid bundled helper accepted")
		}
	}
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := NewSSHDashboard(store)
	binary := testutil.DashboardPayload()
	if err = d.SetHelperBinary(binary); err != nil {
		t.Fatal(err)
	}
	binary[18] = 183
	if d.binary[18] != 62 {
		t.Fatal("caller can mutate the upload")
	}
	if _, err = helperUploadCommand(binary); err == nil {
		t.Fatal("ARM helper accepted while deferred")
	}
}

// TestSSHUploadFaults retains automatic authenticated trust after rejected/checksum-failed upload attempts.
func TestSSHUploadFaults(t *testing.T) {
	for _, fault := range []string{"not_linux", "unsupported_architecture", "upload_tools_unavailable", "upload_storage_failed", "upload_checksum_failed"} {
		t.Run(fault, func(t *testing.T) { /* Fixed faults travel through real SSH without reflecting remote text. */
			f := testutil.StartSSHCommand(t, func(_ context.Context, _ string, ch ssh.Channel) error { /* Fail before helper enrollment with a declared launcher fault. */
				_, err := io.WriteString(ch, `{"schema_version":1,"startup_error":"`+fault+`"}`+"\n")
				return err
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
			q := SSHConnectRequest{Address: f.IP, Port: f.Port, User: "sneak", Password: "fixture-password"}
			for attempt := 0; attempt < 2; attempt++ {
				status, err := d.Connect(ctx, q)
				if err == nil || status.State == "trust_required" || d.Status().State != "disconnected" {
					t.Fatal("upload fault lost automatic trust or left a live helper", err)
				}
			}
			if f.Authentications.Load() != 2 {
				t.Fatal("automatic trust was not reusable after upload failure")
			}
		})
	}
}

// TestSSHUploadCancellation bounds a stalled transfer and exposes connecting until its owned admission ends.
func TestSSHUploadCancellation(t *testing.T) {
	ready := make(chan struct{})
	f := testutil.StartSSHCommand(t, func(ctx context.Context, _ string, ch ssh.Channel) error { /* Withhold reads past the SSH channel window until its parent connection closes. */
		io.WriteString(ch, "{\"schema_version\":1,\"upload_ready\":true}\n")
		close(ready)
		<-ctx.Done()
		return ctx.Err()
	})
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := NewSSHDashboard(store)
	payload := make([]byte, 6<<20)
	copy(payload, testutil.DashboardPayload())
	if err = d.SetHelperBinary(payload); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { /* Keep connection setup independent of the public progress observer. */
		_, err := d.Connect(ctx, SSHConnectRequest{Address: f.IP, Port: f.Port, User: "sneak", Password: "fixture-password"})
		done <- err
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("upload was not admitted")
	}
	if status := d.Status(); status.State != "connecting" || status.Stage != "uploading" {
		t.Fatal("live setup did not expose public upload progress")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil || d.Status().State != "disconnected" {
			t.Fatal("cancelled upload remained connected")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled upload did not join")
	}
}

// TestHelperUploadLinuxFailures runs real byte framing/checksums and confirms private cleanup after short/corrupt uploads.
func TestHelperUploadLinuxFailures(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native Linux shell fixture")
	}
	binary := testutil.DashboardPayload()
	command, err := helperUploadCommand(binary)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Clone(binary)
	corrupt[63] = 1
	for _, payload := range [][]byte{binary[:80], corrupt} {
		root := t.TempDir()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		process := exec.CommandContext(ctx, "/bin/sh", "-c", command)
		process.Env = append(os.Environ(), "TMPDIR="+root)
		process.Stdin = bytes.NewReader(payload)
		output, runErr := process.CombinedOutput()
		cancel()
		if runErr == nil || !strings.Contains(string(output), "upload_checksum_failed") {
			t.Fatal("short/corrupt upload started a helper", string(output), runErr)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatal("failed upload left private executable storage", err)
		}
	}
}

// TestSSHActualLinuxUploadedHelper uploads and runs the actual bundled Linux CLI, with no remote executable selection.
func TestSSHActualLinuxUploadedHelper(t *testing.T) { runActualLinuxHelper(t, true) }

// TestPackagedHelper matches the embedded bytes to the exact server artifact produced by the packaging command.
func TestPackagedHelper(t *testing.T) {
	artifact := os.Getenv("DASHBOARD_BUNDLE_ARTIFACT")
	if artifact == "" {
		t.Skip("explicit packaged artifact check")
	}
	want, err := os.ReadFile(filepath.Clean(artifact))
	if err != nil {
		t.Fatal(err)
	}
	got := bundledHelper()
	if validateHelperBinary(got) != nil || !bytes.Equal(got, want) {
		t.Fatal("packaged helper is not the matching built artifact")
	}
}
