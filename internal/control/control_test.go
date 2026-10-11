package control

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestControlFrames rejects malformed/ambiguous frames before exposing application bodies.
func TestControlFrames(t *testing.T) {
	e := Envelope{Version: 1, Type: "negotiate", RequestID: "request", Body: json.RawMessage(`{"supported_contract_versions":[1]}`)}
	var b bytes.Buffer
	if err := Write(&b, e); err != nil {
		t.Fatal(err)
	}
	original := append([]byte(nil), b.Bytes()...)
	got, err := Read(&b)
	if err != nil || got.Type != e.Type {
		t.Fatalf("control roundtrip %v", err)
	}
	for n := 0; n < len(original); n++ {
		if _, err := Read(bytes.NewReader(original[:n])); err == nil {
			t.Fatalf("accepted partial frame %d", n)
		}
	}
	for _, payload := range []string{`{"schema_version":1,"schema_version":1,"type":"x","request_id":"r","body":{}}`, `{"schema_version":1,"type":"x","request_id":"r","body":{"a":1,"a":2}}`, `{"schema_version":1,"type":"x","request_id":"r","body":{}} {}`, `{"schema_version":2,"type":"x","request_id":"r","body":{}}`} {
		frame := make([]byte, len(payload)+4)
		binary.BigEndian.PutUint32(frame, uint32(len(payload)))
		copy(frame[4:], payload)
		if _, err := Read(bytes.NewReader(frame)); err == nil {
			t.Fatal("ambiguous/unsupported control frame accepted")
		}
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], FrameMax+1)
	if _, err := Read(bytes.NewReader(prefix[:])); err == nil {
		t.Fatal("oversized control allocation accepted")
	}
}

// TestOwnedIdentity checks generated-key load, pinned trust, expiry and no overwrite.
func TestOwnedIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client-key")
	public, err := GenerateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := LoadKey(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateKey(path); err == nil {
		t.Fatal("existing private key overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("private key mutated")
	}
	callback, err := PinnedHost(public.PublicKey, public.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if err = callback("fixture", &net.TCPAddr{}, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}
	other, err := GenerateKey(filepath.Join(dir, "other-key"))
	if err != nil {
		t.Fatal(err)
	}
	otherPublic, _, _, _, _ := ssh.ParseAuthorizedKey([]byte(other.PublicKey))
	if err = callback("fixture", &net.TCPAddr{}, otherPublic); err == nil {
		t.Fatal("wrong host identity accepted")
	}
	if _, err = PinnedHost(public.PublicKey, "wrong"); err == nil {
		t.Fatal("wrong bundle fingerprint accepted")
	}
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	expiry := now.Add(time.Hour)
	records := []Enrollment{{ClientID: "fixture", PublicKey: public.PublicKey, Enabled: true, ExpiresAt: &expiry}}
	if id, err := Authorize(records, signer.PublicKey(), now); err != nil || id != "fixture" {
		t.Fatal("enrolled key rejected")
	}
	if _, err := Authorize(records, signer.PublicKey(), expiry); err == nil {
		t.Fatal("expired key accepted")
	}
	records[0].Enabled = false
	if _, err := Authorize(records, signer.PublicKey(), now); err == nil {
		t.Fatal("revoked key accepted")
	}
}
