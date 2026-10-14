package control

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"sneaky-port/internal/elapsed"
)

// TestProductionElapsedEpochProvenance exercises authentic finite data/calibration and saved replay.
// Native Windows QPC positivity is observational evidence, never calibrated oscillator accuracy.
func TestProductionElapsedEpochProvenance(t *testing.T) {
	for _, kind := range []string{"udp", "tcp"} {
		t.Run(kind, func(t *testing.T) { // Each authentic endpoint gets one finite-start domain and separate report provenance.
			r := finiteRig(t, kind)
			r.plan.Phases[0].Duration = 500e6
			r.plan.Calibration.Cadence = 50e6
			result, err := Run(context.Background(), r.client, r.signer, r.plan)
			if err != nil {
				t.Fatal(err)
			}
			client, err := ReplayReport(result.ReportPath, 8<<20)
			if err != nil {
				t.Fatal(err)
			}
			server, err := ReplayReport(filepath.Join(r.server.Root, "reports", result.Readiness.RunID+"-server"), 8<<20)
			if err != nil {
				t.Fatal(err)
			}
			for _, report := range []*ReplayedReport{client, server} {
				metadata := report.Manifest.ClockDetails
				if metadata == nil || metadata.Source == "" || metadata.Calibrated || metadata.IndependentlyVerified {
					t.Fatal("clock provenance missing or overclaimed", metadata)
				}
				if runtime.GOOS == "windows" && (metadata.Source != "Windows QueryPerformanceCounter" || metadata.FrequencyHz == "" || metadata.TickDenominator != metadata.FrequencyHz) {
					t.Fatal("Windows finite run used another clock", metadata)
				}
			}
			positive, zero := 0, 0
			for _, entry := range client.Evidence.Entries {
				if entry.Attempt != nil && entry.Receipt != nil && *entry.Receipt < client.Evidence.ObservationCutoff {
					if *entry.Receipt < *entry.Attempt {
						t.Fatal("elapsed receipt preceded attempt")
					}
					if *entry.Receipt > *entry.Attempt {
						positive++
					} else {
						zero++
					}
				}
			}
			if positive == 0 {
				t.Fatal("no observed positive completed interval")
			}
			if client.Reconciliation.OneWay.Calibrated != 0 {
				t.Fatal("source metadata invented calibration")
			}
			for _, format := range []string{"json", "jsonl"} {
				var output bytes.Buffer
				if err := ExportReport(&output, client, format); err != nil || !bytes.Contains(output.Bytes(), []byte("clock_details")) {
					t.Fatal("source provenance lost in export", format, err)
				}
			}
			t.Logf("completed elapsed samples positive=%d zero=%d; source=%+v; no independent accuracy bound", positive, zero, client.Manifest.ClockDetails)
			assertPersistedRun(t, r, result, true)
			if root := os.Getenv("SNEAKY_PORT_ELAPSED_EVIDENCE"); root != "" {
				for _, path := range []string{result.ReportPath, filepath.Join(r.server.Root, "reports", result.Readiness.RunID+"-server")} {
					// Retain only public report files; authentication keys/stores remain outside this subtree.
					if err := os.CopyFS(filepath.Join(root, filepath.Base(path)), os.DirFS(path)); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

// TestComparisonClockSourceIdentity refuses to match different tick-frequency provenance.
// Old reports retain their explicit missing details rather than acquiring new source claims.
func TestComparisonClockSourceIdentity(t *testing.T) {
	a := EvidenceManifest{Clock: "same source label", ClockDetails: &elapsed.Metadata{Source: "Windows QueryPerformanceCounter", FrequencyHz: "10000000"}}
	b := a
	details := *a.ClockDetails
	details.FrequencyHz = "3000000"
	b.ClockDetails = &details
	if reflect.DeepEqual(comparisonPlatform(a), comparisonPlatform(b)) {
		t.Fatal("different source frequency treated as compatible")
	}
	b.ClockDetails = nil
	if reflect.DeepEqual(comparisonPlatform(a), comparisonPlatform(b)) {
		t.Fatal("missing source metadata treated as matching known source")
	}
}
