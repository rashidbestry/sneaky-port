package control

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"sneaky-port/internal/testutil"
)

// TestSavedProfileCAS verifies explicit creation, cross-process conflict/failure preservation and shared validation.
func TestSavedProfileCAS(t *testing.T) {
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := testutil.Plan()
	old, err := s.SaveProfile(p.ID, "", p)
	if err != nil {
		t.Fatal(err)
	}
	p.SpikeThreshold++
	if _, err = s.SaveProfile(p.ID, "", p); !errors.Is(err, ErrRevision) {
		t.Fatal("creation replaced profile", err)
	}
	if _, err = s.SaveProfile(p.ID, "stale", p); !errors.Is(err, ErrRevision) {
		t.Fatal("stale edit replaced profile", err)
	}
	var wg sync.WaitGroup
	var successes atomic.Int32
	for range 8 {
		wg.Add(1)
		go func() { // Separate Store values contend through the shared exclusive store lock.
			defer wg.Done()
			other := &Store{Root: s.Root}
			if _, err := other.SaveProfile(p.ID, old.Revision, p); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("concurrent revision admitted multiple edits", successes.Load())
	}
	current, err := s.Profile(p.ID)
	if err != nil || current.Revision == old.Revision || current.Plan.SpikeThreshold != p.SpikeThreshold {
		t.Fatal("saved edit lost", err)
	}
	p.DestinationPort = 0
	if _, err = s.SaveProfile(p.ID, current.Revision, p); err == nil {
		t.Fatal("invalid profile saved")
	}
	unchanged, _ := s.Profile(p.ID)
	if !reflect.DeepEqual(unchanged, current) {
		t.Fatal("failed edit changed saved state")
	}
	for _, id := range []string{"../fixture", "a/b", "a\\b", "", ".."} {
		if _, err := s.Profile(id); err == nil {
			t.Fatal("unsafe ID accepted")
		}
	}
}

// importFixtureFiles copies only fixed raw files from a synthetic partial report with precise integer evidence.
func importFixtureFiles(t *testing.T) map[string][]byte {
	t.Helper()
	dir := partialReplayFixture(t)
	files := map[string][]byte{}
	for _, name := range reportFiles {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = b
	}
	return files
}

// TestReadOnlyReportImport replays before publication and rejects tampering, unknown files and replacement.
func TestReadOnlyReportImport(t *testing.T) {
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := importFixtureFiles(t)
	r, err := s.ImportReport("imported", files)
	if err != nil || r.Verification != "matched" || r.Status != "partial" {
		t.Fatal("import failed", err)
	}
	before, err := s.SavedReport("imported")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportReport("imported", files); !errors.Is(err, ErrRevision) {
		t.Fatal("import replaced evidence", err)
	}
	files["events.jsonl"] = append(bytes.Clone(files["events.jsonl"]), []byte("{}\n")...)
	if _, err = s.ImportReport("bad", files); err == nil {
		t.Fatal("checksum tampering accepted")
	}
	if _, err = os.Lstat(filepath.Join(s.Root, "reports", "bad")); !os.IsNotExist(err) {
		t.Fatal("invalid import published")
	}
	files["../client.json"] = []byte("secret")
	if _, err = s.ImportReport("unsafe", files); err == nil {
		t.Fatal("path import accepted")
	}
	delete(files, "../client.json")
	files["remote-server/manifest.json"] = []byte("{}")
	if _, err = s.ImportReport("orphan", files); err == nil {
		t.Fatal("orphan remote accepted")
	}
	after, _ := s.SavedReport("imported")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed imports changed valid report")
	}
	entries, _ := os.ReadDir(filepath.Join(s.Root, "reports"))
	if len(entries) != 1 || entries[0].Name() != "imported" {
		t.Fatal("staging evidence leaked")
	}
}

// TestSavedCatalogBounds rejects excessive catalogs without allocating or reading all saved documents.
func TestSavedCatalogBounds(t *testing.T) {
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir, err := s.ownedDir("profiles", true)
	if err != nil {
		t.Fatal(err)
	}
	for i := range SavedItemsMax + 1 {
		if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("profile_%03d.json", i)), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.SavedIDs("profiles"); err == nil {
		t.Fatal("unbounded catalog accepted")
	}
	if ids, err := s.SavedIDs("reports"); err != nil || len(ids) != 0 {
		t.Fatal("missing catalog not empty", err)
	}
}

// TestSavedSymlinkNamespace rejects a substituted namespace before reading or writing user-owned data.
func TestSavedSymlinkNamespace(t *testing.T) {
	other, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err = os.Symlink(outside, filepath.Join(other.Root, "profiles")); err != nil {
		t.Skip("symlink creation unavailable on this host")
	}
	if _, err = other.Profile("fixture"); err == nil {
		t.Fatal("symlink namespace read")
	}
	if _, err = other.SaveProfile("fixture", "", testutil.Plan()); err == nil {
		t.Fatal("symlink namespace written")
	}
}
