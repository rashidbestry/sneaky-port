package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
)

// TestServerInitBuiltInPolicy initializes both address families without a policy file.
// Built-in service ceilings must not replace or rewrite the client's per-run workload.
func TestServerInitBuiltInPolicy(t *testing.T) {
	for _, tc := range []struct{ name, advertise, listen, bind string }{
		{"ipv4", "127.0.0.1:22220", "0.0.0.0:22220", "0.0.0.0"},
		{"ipv6", "[::1]:22220", "[::]:22220", "::"},
	} {
		t.Run(tc.name, func(t *testing.T) { // Initialize isolated identities; no listener or privileged port is opened.
			root := filepath.Join(t.TempDir(), "server")
			var out bytes.Buffer
			if err := run(context.Background(), []string{"server-init", "-data-dir", root, "-server-id", "builtin-server", "-advertise", tc.advertise}, &out); err != nil {
				t.Fatal(err)
			}
			var bundle control.Bundle
			if err := json.Unmarshal(out.Bytes(), &bundle); err != nil || bundle.Validate() != nil {
				t.Fatalf("public initialization bundle: %v", err)
			}
			store, err := control.OpenStore(root)
			if err != nil {
				t.Fatal(err)
			}
			identity, err := store.Server()
			if err != nil || identity.Policy.ControlListen != tc.listen || identity.Policy.DataBindIP != tc.bind {
				t.Fatalf("built-in binding: %+v %v", identity.Policy, err)
			}
			plan := profile.Default()
			if tc.name == "ipv6" {
				plan.Family, plan.ServerAddress = "ipv6", "::1"
			}
			before, err := plan.Hash()
			if err != nil {
				t.Fatal(err)
			}
			if err = identity.Policy.Check(plan); err != nil {
				t.Fatal(err)
			}
			after, err := plan.Hash()
			if err != nil || before != after {
				t.Fatal("server defaults changed the client workload", err)
			}
			plan.Limits.QueueBytes = identity.Policy.MaxQueueBytes + 1
			if identity.Policy.Check(plan) == nil {
				t.Fatal("built-in resource ceiling was bypassed")
			}
		})
	}
}

// TestAutomaticServerIdentity preserves first-start trust, enrollments and bindings across restarts.
func TestAutomaticServerIdentity(t *testing.T) {
	store, err := control.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := loadOrInitServer(store, "127.0.0.1:22022", "")
	if err != nil || first.Bundle.ControlPort != 22022 || first.Bundle.ControlAddress != "127.0.0.1" {
		t.Fatal("automatic server initialization", err)
	}
	auth, err := store.Authorizations()
	if err != nil || len(auth.Clients) != 0 {
		t.Fatal("new server must await authenticated enrollment", err)
	}
	client, err := control.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := client.InitClient("owner-client", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Enroll(enrollment); err != nil {
		t.Fatal(err)
	}
	second, err := loadOrInitServer(store, "", "")
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("restart replaced server identity or policy", err)
	}
	if _, err = loadOrInitServer(store, "127.0.0.1:22023", ""); err == nil {
		t.Fatal("new startup flags silently replaced saved bindings")
	}
	auth, err = store.Authorizations()
	if err != nil || len(auth.Clients) != 1 || auth.Clients[0].ClientID != enrollment.ClientID {
		t.Fatal("startup changed existing client authorization", err)
	}
}

// TestServerInitExplicitPolicy preserves existing constrained deployments and rejects malformed overrides without falling back.
func TestServerInitExplicitPolicy(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "constrained", true: "malformed"}[malformed], func(t *testing.T) { // Override failures must not create a usable default identity.
			root := t.TempDir()
			policy := control.DefaultServerPolicy("0.0.0.0:22220", "0.0.0.0")
			policy.PortMin, policy.PortMax = 32000, 32010
			b, err := json.Marshal(policy)
			if err != nil {
				t.Fatal(err)
			}
			if malformed {
				b = []byte(`{"schema_version":1}`)
			}
			policyPath := filepath.Join(root, "override.json")
			if err = os.WriteFile(policyPath, b, 0600); err != nil {
				t.Fatal(err)
			}
			data := filepath.Join(root, "store")
			var out bytes.Buffer
			err = run(context.Background(), []string{"server-init", "-data-dir", data, "-server-id", "override-server", "-advertise", "127.0.0.1:22220", "-policy", policyPath}, &out)
			if malformed {
				if err == nil {
					t.Fatal("invalid explicit policy fell back to defaults")
				}
				if _, err = os.Stat(filepath.Join(data, "server.json")); !os.IsNotExist(err) {
					t.Fatal("invalid policy initialized an identity", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			store, err := control.OpenStore(data)
			if err != nil {
				t.Fatal(err)
			}
			identity, err := store.Server()
			if err != nil || identity.Policy.PortMin != 32000 || identity.Policy.PortMax != 32010 {
				t.Fatal("explicit port restrictions changed", err)
			}
		})
	}
}
