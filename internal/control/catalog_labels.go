package control

import (
	"errors"
	"os"
	"path/filepath"
	"sneaky-port/internal/profile"
	"strings"
	"time"
)

// ReportLabel contains unverified display metadata only; loading diagnostics still performs full replay gates.
type ReportLabel struct {
	ID       string `json:"report_id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol,omitempty"`
	Created  string `json:"created,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	State    string `json:"metadata_state"`
}

// ReportLabel reads only a bounded manifest under the caller's aggregate catalog budget, never raw event files.
// Labels cannot establish report validity, completeness, remote delivery or calibrated timing.
func (s *Store) ReportLabel(id string, budget *uint64) (ReportLabel, error) {
	out := ReportLabel{ID: id, Name: "Saved test", State: "unverified"}
	if !profile.SafeID(id) {
		return out, errors.New("invalid report identity")
	}
	root, err := s.ownedDir("reports", false)
	if err != nil {
		return out, err
	}
	dir := filepath.Join(root, id)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return out, errors.New("catalog report directory unavailable")
	}
	remaining := min(*budget, uint64(FrameMax))
	before := remaining
	data, err := reportFile(dir, "manifest.json", &remaining)
	*budget -= before - remaining
	if err != nil {
		return out, err
	}
	var manifest EvidenceManifest
	if decodeReportObject(data, &manifest) != nil || manifest.Version != 1 || s.AdapterRegistry().Validate(manifest.Plan) != nil || (manifest.Endpoint != "client" && manifest.Endpoint != "server") {
		return out, errors.New("catalog metadata unavailable")
	}
	if _, err = time.Parse(time.RFC3339Nano, manifest.Created); err != nil {
		return out, errors.New("catalog metadata unavailable")
	}
	out.Name = strings.ReplaceAll(strings.ReplaceAll(manifest.Plan.ID, "-", " "), "_", " ")
	out.Protocol, out.Created, out.Endpoint = manifest.Plan.Adapter, manifest.Created, manifest.Endpoint
	return out, nil
}
