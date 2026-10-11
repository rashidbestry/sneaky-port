package control

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestPersistentStoreGuards checks reopen, explicit host replacement, atomic failed updates and private-key preservation.
func TestPersistentStoreGuards(t *testing.T) {
	r := newRig(t, "udp")
	reopened, err := OpenStore(r.server.Root)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := reopened.Authorizations()
	if err != nil || len(auth.Clients) != 1 || auth.Revision != 1 {
		t.Fatalf("persistent allowlist: %+v %v", auth, err)
	}
	if err = r.client.ImportPairing("fixture", r.bundle, false); err == nil {
		t.Fatal("host trust replaced without explicit flag")
	}
	bad := r.bundle
	bad.HostFingerprint = "bad"
	if err = r.client.ImportPairing("fixture", bad, true); err == nil {
		t.Fatal("invalid host bundle persisted")
	}
	loaded, err := r.client.Pairing("fixture")
	if err != nil || loaded.HostFingerprint != r.bundle.HostFingerprint {
		t.Fatal("failed pairing update changed trust")
	}
	path := filepath.Join(r.server.Root, "authorization.json")
	before, _ := os.ReadFile(path)
	duplicate := auth.Clients[0]
	duplicate.ClientID = "another"
	if err = r.server.Enroll(duplicate); err == nil {
		t.Fatal("same key enrolled with two IDs")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed update damaged authorization")
	}
	lock, err := r.server.lock()
	if err != nil {
		t.Fatal(err)
	}
	if err = r.server.Disable("client"); err == nil {
		t.Fatal("concurrent writer lock ignored")
	}
	r.server.unlock(lock)
	if err = r.server.Disable("client"); err != nil {
		t.Fatal(err)
	}
	auth, err = r.server.Authorizations()
	if err != nil || auth.Clients[0].Enabled || auth.Revision != 2 {
		t.Fatal("revocation not persisted")
	}
	// A failed initialization must never delete a preexisting generated-key destination.
	for _, role := range []string{"client", "server"} {
		store, err := OpenStore(filepath.Join(t.TempDir(), role))
		if err != nil {
			t.Fatal(err)
		}
		key := filepath.Join(store.Root, "keys", role+"-key")
		if _, err = GenerateKey(key); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(key)
		if role == "client" {
			_, err = store.InitClient("client", "")
		} else {
			serverID, _ := r.server.Server()
			_, err = store.InitServer("server", "127.0.0.1", 32001, serverID.Policy, "")
		}
		if err == nil {
			t.Fatal("existing key destination overwritten")
		}
		after, err := os.ReadFile(key)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("failed initialization deleted/modified existing key")
		}
	}
}

// TestEncryptedKeyReference keeps caller-owned key bytes untouched and requires the exact passphrase in memory.
func TestEncryptedKeyReference(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pass := []byte("fixture-passphrase")
	block, err := ssh.MarshalPrivateKeyWithPassphrase(private, "fixture", pass)
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(t.TempDir(), "encrypted-key")
	before := pem.EncodeToMemory(block)
	if err = os.WriteFile(key, before, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(t.TempDir(), "client"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.InitClient("client", key); err == nil {
		t.Fatal("encrypted key accepted without passphrase")
	}
	if _, err = store.InitClient("client", key, pass); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(key)
	if !bytes.Equal(before, after) {
		t.Fatal("referenced encrypted key changed")
	}
	id, _ := store.Client()
	if id.KeyRef != key {
		t.Fatal("key reference copied/substituted")
	}
}

// TestStrictPublicImports rejects ambiguous, missing, null and deep fields while accepting a public UTF-8 BOM export.
func TestStrictPublicImports(t *testing.T) {
	valid := Enrollment{ClientID: "client", PublicKey: "public", Enabled: true}
	b, _ := json.Marshal(valid)
	var decoded Enrollment
	if err := DecodeObject(append([]byte{0xef, 0xbb, 0xbf}, b...), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{`{"client_id":"c","public_key":"p"}`, `{"client_id":"c","public_key":"p","enabled":null}`, `{"client_id":"c","public_key":"p","enabled":true,"Enabled":false}`, `{"client_id":"c","client_id":"d","public_key":"p","enabled":true}`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)} {
		if err := DecodeObject([]byte(input), &decoded); err == nil {
			t.Fatalf("ambiguous public import accepted: %s", input)
		}
	}
	expiry := time.Now()
	valid.ExpiresAt = &expiry
	b, _ = json.Marshal(valid)
	if err := DecodeObject(b, &decoded); err != nil || decoded.ExpiresAt == nil {
		t.Fatalf("explicit expiry rejected: %v", err)
	}
}
