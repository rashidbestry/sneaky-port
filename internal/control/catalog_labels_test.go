package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sneaky-port/internal/testutil"
)

// TestReportLabelBounds reads only bounded display metadata and never promotes it to valid measurement evidence.
func TestReportLabelBounds(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(store.Root, "reports", "named-trial")
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := EvidenceManifest{Version: 1, Plan: testutil.Plan(), Endpoint: "client", Created: "2026-10-12T17:20:00Z"}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	budget := uint64(len(data))
	label, err := store.ReportLabel("named-trial", &budget)
	if err != nil || budget != 0 || label.State != "unverified" || label.Protocol != "udp" || label.Created != manifest.Created {
		t.Fatal("bounded display metadata unavailable", err)
	}
	if _, err = store.SavedReport("named-trial"); err == nil {
		t.Fatal("manifest-only label was treated as valid report evidence")
	}
	for _, id := range []string{"named-trial", "../outside"} {
		budget = uint64(len(data) - 1)
		if _, err = store.ReportLabel(id, &budget); err == nil || budget != uint64(len(data)-1) {
			t.Fatal("insufficient budget or directory traversal accepted")
		}
	}
}
