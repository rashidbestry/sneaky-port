package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
)

// TestValidateDefaultProfile proves an omitted flag requires neither example files nor a local readiness.yaml.
func TestValidateDefaultProfile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("readiness.yaml", []byte("not a valid profile"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"validate"}, &out); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Plan profile.Plan `json:"plan"`
		Hash string       `json:"plan_hash"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want, err := profile.Default().Hash()
	if err != nil {
		t.Fatal(err)
	}
	actual, err := got.Plan.Hash()
	if err != nil || actual != want || got.Hash != hex.EncodeToString(want[:]) {
		t.Fatal("default validation did not expose the exact compiled starter", err)
	}
}

// TestProfileOverrideErrors keeps explicit missing/malformed files strict across every profile-taking command.
func TestProfileOverrideErrors(t *testing.T) {
	for _, command := range []string{"validate", "client-api", "client-ready", "client-run"} {
		t.Run(command, func(t *testing.T) { // A file failure must precede serving, authentication or measurement.
			root := t.TempDir()
			args := []string{command}
			if command != "validate" {
				args = append(args, "-data-dir", filepath.Join(root, "client"))
			}
			if command == "client-ready" {
				args = append(args, "-hold-ns", "0")
			}
			path := filepath.Join(root, "explicit.json")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var out bytes.Buffer
			err := run(ctx, append(args, "-profile", path), &out)
			var missing *os.PathError
			if !errors.As(err, &missing) || missing.Path != path || !os.IsNotExist(err) {
				t.Fatal("missing explicit profile fell back to defaults", err)
			}
			if err = os.WriteFile(path, []byte(`{"schema_version":1}`), 0600); err != nil {
				t.Fatal(err)
			}
			err = run(ctx, append(args, "-profile", path), &out)
			if err == nil || !strings.Contains(err.Error(), "profile.profile_id is required") {
				t.Fatal("malformed explicit profile bypassed strict validation", err)
			}
		})
	}
}

// TestDefaultCLIRequiresIdentity accepts omitted profiles without bypassing the existing trusted-client requirement.
func TestDefaultCLIRequiresIdentity(t *testing.T) {
	for _, command := range []string{"client-ready", "client-run"} {
		t.Run(command, func(t *testing.T) { // The valid default reaches identity lookup without opening data sockets.
			root := filepath.Join(t.TempDir(), "client")
			args := []string{command, "-data-dir", root}
			if command == "client-ready" {
				args = append(args, "-hold-ns", "0")
			}
			err := run(context.Background(), args, &bytes.Buffer{})
			var missing *os.PathError
			if !errors.As(err, &missing) || missing.Path != filepath.Join(root, "client.json") || !os.IsNotExist(err) {
				t.Fatal("default profile did not retain normal identity checks", err)
			}
		})
	}
}

// TestClientAPIDefaultProfile starts the real API with built-ins, saved edits or an explicit override and joins shutdown.
// It sends protected setup reads only; no SSH login, network measurement or automatic profile/key save occurs.
func TestClientAPIDefaultProfile(t *testing.T) {
	for _, mode := range []string{"default", "saved", "explicit", "implicit-directory", "zero-arguments", "leading-flags"} {
		t.Run(mode, func(t *testing.T) { // Each API owns an isolated store and finite loopback listener.
			if runtime.GOOS != "windows" && (mode == "zero-arguments" || mode == "leading-flags") {
				t.Skip("Linux no-command startup retains its server role")
			}
			root := t.TempDir()
			config := filepath.Join(root, "config")
			t.Setenv("APPDATA", config)
			t.Setenv("XDG_CONFIG_HOME", config)
			implicit := mode == "implicit-directory" || mode == "zero-arguments" || mode == "leading-flags"
			storeRoot := filepath.Join(root, "client")
			if implicit {
				storeRoot = filepath.Join(config, "sneaky-port", "client")
			}
			store, err := control.OpenStore(storeRoot)
			if err != nil {
				t.Fatal(err)
			}
			want := profile.Default()
			args := []string{"client-api", "-data-dir", store.Root}
			switch mode {
			case "implicit-directory":
				args = []string{"client-api"}
			case "zero-arguments":
				args = nil
			case "leading-flags":
				args = []string{"-listen", "127.0.0.1:0"}
			}
			if mode == "saved" {
				want.Phases[0].Duration = 3e9
				if _, err = store.SaveProfile(want.ID, "", want); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "explicit" {
				want.ID, want.DestinationPort = "explicit-start", 31000
				b, marshalErr := json.Marshal(want)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				path := filepath.Join(root, "profile.json")
				if err = os.WriteFile(path, b, 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "-profile", path)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			reader, writer := io.Pipe()
			done := make(chan error, 1)
			go func() { // Deliver bootstrap through a pipe, never sharing or logging its bearer buffer.
				err := run(ctx, args, writer)
				writer.CloseWithError(err)
				done <- err
			}()
			t.Cleanup(func() { // Join the owned API before removing its private temporary store.
				cancel()
				reader.Close()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(5 * time.Second):
					t.Error("default API shutdown did not join")
				}
			})
			var bootstrap struct {
				URL       string `json:"api_url"`
				Token     string `json:"session_token"`
				ProfileID string `json:"profile_id"`
				Revision  string `json:"profile_revision"`
			}
			if err = json.NewDecoder(reader).Decode(&bootstrap); err != nil {
				t.Fatal("default API bootstrap failed", err)
			}
			wantHash, err := want.Hash()
			if err != nil || bootstrap.ProfileID != want.ID || bootstrap.Revision != hex.EncodeToString(wantHash[:]) {
				t.Fatal("API did not select the intended startup plan", err)
			}
			client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
			defer client.CloseIdleConnections()
			setup := apiCLIRequest(t, client, bootstrap.URL, bootstrap.Token, "GET", "/api/v1/setup", "default-setup", "")
			selection := setup["selected_profile"].(map[string]any)
			if selection["profile_revision"] != bootstrap.Revision {
				t.Fatal("protected setup disagrees with the selected startup revision")
			}
			b, err := json.Marshal(selection["profile"])
			if err != nil {
				t.Fatal(err)
			}
			selected, err := profile.Decode(bytes.NewReader(b))
			if err != nil {
				t.Fatal("protected startup settings are invalid", err)
			}
			selectedHash, err := selected.Hash()
			if err != nil || selectedHash != wantHash {
				t.Fatal("protected startup settings differ from the intended plan", err)
			}
			if _, err = store.Client(); !os.IsNotExist(err) {
				t.Fatal("startup unexpectedly initialized a private client identity", err)
			}
			if mode != "saved" {
				if _, err = store.Profile(want.ID); !os.IsNotExist(err) {
					t.Fatal("startup unexpectedly saved its transient profile", err)
				}
			}
			if !implicit {
				if _, err = os.Stat(filepath.Join(config, "sneaky-port", "client")); !os.IsNotExist(err) {
					t.Fatal("explicit store override touched the default directory", err)
				}
			}
		})
	}
}
