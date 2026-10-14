package control

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestWindowsStoreReaderAllowsAtomicRevocation holds the actual reader handle across revocation.
// The old reader keeps its complete snapshot, fresh authorization sees the new revision, and ACLs match.
func TestWindowsStoreReaderAllowsAtomicRevocation(t *testing.T) {
	r := newRig(t, "udp")
	path := filepath.Join(r.server.Root, "authorization.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeACL, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	held, err := openStoreObject(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := r.server.Disable("client"); err != nil {
		t.Fatal("reader blocked atomic revocation", err)
	}
	oldBytes, err := io.ReadAll(held)
	if err != nil || !bytes.Equal(before, oldBytes) {
		t.Fatal("held reader lost its complete old snapshot", err)
	}
	auth, err := r.server.Authorizations()
	if err != nil || len(auth.Clients) != 1 || auth.Revision != 2 || auth.Clients[0].Enabled {
		t.Fatal("fresh reader did not see atomic revocation", auth, err)
	}
	if _, err := r.server.CheckClient(r.signer.PublicKey(), time.Now()); err == nil {
		t.Fatal("fresh authorization accepted the revoked key")
	}
	afterACL, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || beforeACL.String() != afterACL.String() {
		t.Fatal("atomic replacement changed the restricted document ACL", err)
	}
}

// TestWindowsStoreReaderUnicodeLongPath preserves Go's existing Unicode/extended-path file support.
// Only public fixture bytes are created; no application identity or user permission is changed.
func TestWindowsStoreReaderUnicodeLongPath(t *testing.T) {
	for _, long := range []bool{false, true} {
		t.Run(map[bool]string{false: "unicode", true: "extended"}[long], func(t *testing.T) { // Native extended paths must preserve the same guarded document contents.
			base := t.TempDir()
			if long {
				base = filepath.Join(base, strings.Repeat("r", 120), strings.Repeat("s", 120))
			}
			path := filepath.Join(base, "İstanbul-日本", "authorization.json")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			contents := []byte(`{"schema_version":1,"revision":"2","clients":[]}`)
			if err := os.WriteFile(path, contents, 0600); err != nil {
				t.Fatal(err)
			}
			var auth Authorization
			if err := readObject(path, &auth); err != nil || auth.Version != 1 || auth.Revision != 2 {
				t.Fatal("Unicode/extended public document rejected", auth, err)
			}
		})
	}
}

// TestWindowsAtomicRevocationPreservesReadOnlyDocument proves POSIX replacement does not bypass attributes.
// A failed publication retains complete old authorization bytes and its existing revision.
func TestWindowsAtomicRevocationPreservesReadOnlyDocument(t *testing.T) {
	r := newRig(t, "udp")
	path := filepath.Join(r.server.Root, "authorization.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attributes, err := windows.GetFileAttributes(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(name, attributes|windows.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
	defer windows.SetFileAttributes(name, attributes)
	if err := r.server.Disable("client"); err == nil {
		t.Fatal("read-only authorization was replaced")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed replacement damaged old authorization", err)
	}
}
