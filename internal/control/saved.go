package control

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sneaky-port/internal/profile"
)

const SavedItemsMax = 256
const APIReportBytes = 8 << 20

var ErrRevision = errors.New("saved revision conflict")

// SavedProfile carries normalized settings and their canonical revision, never local key paths.
type SavedProfile struct {
	Plan     profile.Plan `json:"profile"`
	Revision string       `json:"profile_revision"`
}

// ownedDir checks each application namespace before use; imports never select directories or paths.
func (s *Store) ownedDir(namespace string, create bool) (string, error) {
	for _, dir := range []string{s.Root, filepath.Join(s.Root, namespace)} {
		if create {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return "", err
			}
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("owned directory required")
		}
		if create {
			if err = restrictDirectory(dir); err != nil {
				return "", err
			}
		}
	}
	return filepath.Join(s.Root, namespace), nil
}

// SavedIDs bounds directory enumeration before sorting; unexpected objects are excluded, never followed.
// The cap limits catalog inspection, not automatic deletion or retention of measurement evidence.
func (s *Store) SavedIDs(namespace string) ([]string, error) {
	if namespace != "profiles" && namespace != "pairings" && namespace != "reports" {
		return nil, errors.New("invalid namespace")
	}
	dir, err := s.ownedDir(namespace, false)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(SavedItemsMax + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > SavedItemsMax {
		return nil, errors.New("saved catalog exceeds inspection cap")
	}
	ids := []string{}
	for _, e := range entries {
		name := e.Name()
		if namespace == "reports" {
			if !e.IsDir() {
				continue
			}
		} else {
			if !e.Type().IsRegular() || !strings.HasSuffix(name, ".json") {
				continue
			}
			name = strings.TrimSuffix(name, ".json")
		}
		if profile.SafeID(name) {
			ids = append(ids, name)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// Profile reads a bounded saved JSON through the same normalized profile validator as the CLI.
func (s *Store) Profile(id string) (SavedProfile, error) {
	var out SavedProfile
	if !profile.SafeID(id) {
		return out, errors.New("invalid profile ID")
	}
	dir, err := s.ownedDir("profiles", false)
	if err != nil {
		return out, err
	}
	budget := uint64(profile.DocumentMax)
	b, err := reportFile(dir, id+".json", &budget)
	if err != nil {
		return out, err
	}
	p, err := profile.Decode(bytes.NewReader(b))
	if err != nil || p.ID != id || s.AdapterRegistry().Validate(p) != nil {
		return out, errors.New("invalid saved profile")
	}
	h, err := p.Hash()
	return SavedProfile{p, hex.EncodeToString(h[:])}, err
}

// SaveProfile compares the expected revision while holding the cross-process store lock.
// Empty revision means create only; failed validation/conflicts preserve the previous valid file.
func (s *Store) SaveProfile(id, expected string, p profile.Plan) (SavedProfile, error) {
	var out SavedProfile
	if !profile.SafeID(id) || p.ID != id {
		return out, errors.New("invalid profile identity")
	}
	if err := s.AdapterRegistry().Validate(p); err != nil {
		return out, err
	}
	lock, err := s.lock()
	if err != nil {
		return out, err
	}
	defer s.unlock(lock)
	dir, err := s.ownedDir("profiles", true)
	if err != nil {
		return out, err
	}
	old, err := s.Profile(id)
	if err == nil && (expected == "" || expected != old.Revision) || os.IsNotExist(err) && expected != "" {
		return out, ErrRevision
	}
	if err != nil && !os.IsNotExist(err) {
		return out, err
	}
	if os.IsNotExist(err) {
		ids, e := s.SavedIDs("profiles")
		if e != nil || len(ids) >= SavedItemsMax {
			return out, errors.New("profile catalog exhausted")
		}
	}
	if err = s.writeAtomic(filepath.Join(dir, id+".json"), p); err != nil {
		return out, err
	}
	return s.Profile(id)
}

// SavedReport resolves only safe report IDs and uses existing replay/checksum/accounting gates.
// Offline evidence is consistency-checked, never authenticated by import or cached summaries.
func (s *Store) SavedReport(id string) (*ReplayedReport, error) {
	if !profile.SafeID(id) {
		return nil, errors.New("invalid report ID")
	}
	dir, err := s.ownedDir("reports", false)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, id)
	if _, err = os.Lstat(path); err != nil {
		return nil, err
	}
	return ReplayReport(path, APIReportBytes, s.AdapterRegistry())
}

// ImportReport admits only fixed raw-evidence files into a private staging directory, then replays it.
// No existing report is replaced; failures remove only this operation's verified staging directory.
func (s *Store) ImportReport(id string, files map[string][]byte) (*ReplayedReport, error) {
	if !profile.SafeID(id) || len(files) < 2 || len(files) > 7 {
		return nil, errors.New("invalid import identity/files")
	}
	allowed := map[string]bool{"manifest.json": true, "events.jsonl": true, "summary.json": true, "server-transfer.json": true, "remote-server/manifest.json": true, "remote-server/events.jsonl": true, "remote-server/summary.json": true}
	total := 0
	for name, b := range files {
		if !allowed[name] {
			return nil, errors.New("unsupported report file")
		}
		total += len(b)
		if total > FrameMax {
			return nil, errors.New("import exceeds decoded file budget")
		}
	}
	if _, ok := files["manifest.json"]; !ok {
		return nil, errors.New("manifest required")
	}
	if _, ok := files["events.jsonl"]; !ok {
		return nil, errors.New("raw evidence required")
	}
	remote := 0
	for name := range files {
		if strings.HasPrefix(name, "remote-server/") {
			remote++
		}
	}
	if remote != 0 && (remote != 3 || files["server-transfer.json"] == nil) {
		return nil, errors.New("complete transferred file group required")
	}
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer s.unlock(lock)
	root, err := s.ownedDir("reports", true)
	if err != nil {
		return nil, err
	}
	ids, err := s.SavedIDs("reports")
	if err != nil || len(ids) >= SavedItemsMax {
		return nil, errors.New("report catalog exhausted")
	}
	target := filepath.Join(root, id)
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		return nil, ErrRevision
	}
	temp, err := os.MkdirTemp(root, ".import-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp) // MkdirTemp returns a verified child of the owned report root; no user path is removed.
	if err = restrictDirectory(temp); err != nil {
		return nil, err
	}
	for name, b := range files {
		dir := temp
		if strings.HasPrefix(name, "remote-server/") {
			dir = filepath.Join(temp, "remote-server")
			if err = os.MkdirAll(dir, 0700); err != nil {
				return nil, err
			}
			if err = restrictDirectory(dir); err != nil {
				return nil, err
			}
		}
		if err = publishEvidence(dir, filepath.Base(name), b, false); err != nil {
			return nil, err
		}
	}
	r, err := ReplayReport(temp, APIReportBytes, s.AdapterRegistry())
	if err != nil {
		return nil, err
	}
	if remote != 0 && r.ServerReport == nil {
		return nil, errors.New("remote evidence was not validated")
	}
	if _, ok := files["summary.json"]; ok && r.Manifest.SummaryFile != "summary.json" {
		return nil, errors.New("unreferenced summary file")
	}
	if _, ok := files["server-transfer.json"]; ok && r.TransferReceipt == nil {
		return nil, errors.New("unreferenced transfer receipt")
	}
	if err = os.Rename(temp, target); err != nil {
		return nil, err
	}
	return r, nil
}
