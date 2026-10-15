package control

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"sneaky-port/internal/elapsed"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
)

// TestProductionElapsedEpochProvenance exercises authentic finite data/calibration and saved replay.
// Actual eligible intervals may be empty or zero; source provenance never establishes oscillator accuracy.
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
				if metadata == nil || metadata.Source == "" || metadata.Quantization == "" || metadata.Calibrated || metadata.IndependentlyVerified {
					t.Fatal("clock provenance missing or overclaimed", metadata)
				}
				if runtime.GOOS == "windows" {
					frequency, err := strconv.ParseUint(metadata.FrequencyHz, 10, 64)
					if err != nil || frequency == 0 || metadata.Source != "Windows QueryPerformanceCounter" || metadata.TickNumeratorNS != "1000000000" || metadata.TickDenominator != metadata.FrequencyHz {
						t.Fatal("Windows finite run lost QPC frequency/tick provenance", metadata)
					}
				}
				positive, zero, err := elapsedSampleAccounting(report.Evidence)
				if err != nil {
					t.Fatalf("endpoint=%s elapsed accounting: %v", report.Manifest.Endpoint, err)
				}
				t.Logf("endpoint=%s eligible elapsed samples positive=%d zero=%d; main planned=%d attempted=%d skipped=%d samples=%d; warmup samples=%d cutoff=%d; source=%+v; no independent accuracy bound", report.Manifest.Endpoint, positive, zero, report.Evidence.Summary.Planned, report.Evidence.Summary.Attempted, report.Evidence.Summary.Skipped, report.Evidence.Summary.RTTSamples, report.Evidence.Warmup.RTTSamples, report.Evidence.ObservationCutoff, metadata)
			}
			if client.Reconciliation == nil || client.Reconciliation.OneWay == nil {
				t.Fatal("saved source evidence lost reconciliation/one-way state")
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

// elapsedSampleAccounting checks exact eligible RTT populations against every saved local scope.
// Empty and equal-timestamp observations remain honest; cutoff equality and duplicate receipts add no sample.
func elapsedSampleAccounting(r engine.Result) (positive, zero profile.Decimal, err error) {
	summaries := append([]measure.Summary{r.Warmup, r.Summary}, r.Phases...)
	var main, phases profile.Decimal
	for index, summary := range summaries {
		var pos, equal, onTime, late profile.Decimal
		var low, high uint64
		for _, entry := range r.Entries {
			lifecycle := uint8(2)
			if index == 0 {
				lifecycle = 1
			}
			if entry.Identity.Lifecycle != lifecycle || index >= 2 && entry.Identity.Phase != uint32(index-2) || entry.Scheduled >= r.ObservationCutoff || entry.Attempt == nil || entry.Receipt == nil || *entry.Receipt >= r.ObservationCutoff {
				continue
			}
			if *entry.Receipt < *entry.Attempt {
				return 0, 0, fmt.Errorf("scope=%d receipt precedes attempt: identity=%+v attempt=%d receipt=%d cutoff=%d", index, entry.Identity, *entry.Attempt, *entry.Receipt, r.ObservationCutoff)
			}
			interval := uint64(*entry.Receipt - *entry.Attempt)
			if pos+equal == 0 || interval < low {
				low = interval
			}
			if interval > high {
				high = interval
			}
			if interval == 0 {
				equal++
			} else {
				pos++
			}
			if *entry.Receipt <= entry.Deadline {
				onTime++
			} else {
				late++
			}
		}
		count := pos + equal
		if summary.RTTSamples != count || summary.OnTime != onTime || summary.Late != late {
			return 0, 0, fmt.Errorf("scope=%d eligible samples positive=%d zero=%d on_time=%d late=%d differ from summary samples=%d on_time=%d late=%d", index, pos, equal, onTime, late, summary.RTTSamples, summary.OnTime, summary.Late)
		}
		values := []*string{summary.RTTMin, summary.RTTMax, summary.RTTMedian, summary.RTTP95, summary.RTTP99}
		for _, value := range values {
			if count == 0 {
				if value != nil {
					return 0, 0, fmt.Errorf("scope=%d empty population invented RTT value %s", index, *value)
				}
				continue
			}
			if value == nil {
				return 0, 0, fmt.Errorf("scope=%d eligible samples lost RTT value", index)
			}
			v, parseErr := strconv.ParseUint(*value, 10, 64)
			if parseErr != nil || v < low || v > high {
				return 0, 0, fmt.Errorf("scope=%d RTT value %s outside observed %d..%d ns", index, *value, low, high)
			}
		}
		if count != 0 && (*summary.RTTMin != strconv.FormatUint(low, 10) || *summary.RTTMax != strconv.FormatUint(high, 10)) {
			return 0, 0, fmt.Errorf("scope=%d RTT extrema differ from actual intervals", index)
		}
		if index < 2 {
			positive, zero = positive+pos, zero+equal
		}
		if index == 1 {
			main = count
		} else if index >= 2 {
			phases += count
		}
	}
	if main != phases {
		return 0, 0, fmt.Errorf("main samples=%d differ from phase samples=%d", main, phases)
	}
	return positive, zero, nil
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
