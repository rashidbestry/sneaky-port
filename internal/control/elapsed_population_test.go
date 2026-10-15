package control

import (
	"testing"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
)

// TestElapsedSamplePopulations covers valid empty/zero populations and strict fixture guards.
// Synthetic production-ledger timestamps prove accounting contracts, not native clock or socket accuracy.
func TestElapsedSamplePopulations(t *testing.T) {
	for _, tc := range []struct {
		name, mode     string
		positive, zero profile.Decimal
		invalid        bool
	}{
		{"skipped", "skipped", 0, 0, false},
		{"unresolved", "unresolved", 0, 0, false},
		{"zero", "zero", 0, 2, false},
		{"positive", "positive", 2, 0, false},
		{"mixed", "mixed", 1, 1, false},
		{"warmup_only", "warmup_only", 1, 0, false},
		{"cutoff_equal", "cutoff_equal", 0, 0, false},
		{"cutoff_after", "cutoff_after", 0, 0, false},
		{"late", "late", 2, 0, false},
		{"duplicate", "duplicate", 0, 2, false},
		{"backwards", "zero", 0, 0, true},
		{"wrong_count", "zero", 0, 0, true},
		{"wrong_deadline_class", "zero", 0, 0, true},
		{"wrong_phase", "zero", 0, 0, true},
		{"invented_quantile", "skipped", 0, 0, true},
		{"wrong_extent", "positive", 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) { // Exercise independent lifecycle/phase scopes, including a quiet phase with no samples.
			ledger, err := measure.NewLedger(8)
			if err != nil {
				t.Fatal(err)
			}
			for index := 0; index < 2; index++ {
				id := measure.Identity{Lifecycle: uint8(index + 1), Phase: uint32(index), Sequence: profile.Decimal(index)}
				at := uint64(index*40 + 10)
				if err := ledger.Schedule(id, at, at+20); err != nil {
					t.Fatal(err)
				}
				if tc.mode == "skipped" {
					if err := ledger.Skip(id, "skipped_scheduler"); err != nil {
						t.Fatal(err)
					}
					continue
				}
				for _, err := range []error{ledger.Attempt(id, at), ledger.Written(id, at, 124, true)} {
					if err != nil {
						t.Fatal(err)
					}
				}
				if tc.mode == "unresolved" || tc.mode == "warmup_only" && index == 1 {
					continue
				}
				receipt := at
				switch tc.mode {
				case "positive", "warmup_only":
					receipt += 3
				case "mixed":
					receipt += uint64(index * 3)
				case "late":
					receipt += 21
				case "cutoff_equal":
					receipt = 100
				case "cutoff_after":
					receipt = 101
				}
				if err := ledger.ReceiveReceipt(id, at, receipt); err != nil {
					t.Fatal(err)
				}
				if tc.mode == "duplicate" {
					if err := ledger.ReceiveReceipt(id, at, receipt+1); err != nil {
						t.Fatal(err)
					}
				}
			}
			result := engine.Result{ObservationCutoff: 100, Entries: ledger.Snapshot(), Summary: ledger.Summarize(100, nil), Warmup: ledger.SummarizePhase(100, nil, 1, nil), Phases: ledger.SummarizePhases(100, nil, 2)}
			switch tc.name {
			case "backwards":
				at := profile.Decimal(49)
				result.Entries[1].Receipt = &at
			case "wrong_count":
				result.Summary.RTTSamples++
			case "wrong_deadline_class":
				result.Summary.OnTime--
				result.Summary.Late++
			case "wrong_phase":
				result.Phases[0] = result.Summary
			case "invented_quantile":
				value := "0"
				result.Summary.RTTP95 = &value
			case "wrong_extent":
				value := "4"
				result.Summary.RTTMax = &value
			}
			positive, zero, err := elapsedSampleAccounting(result)
			if (err != nil) != tc.invalid {
				t.Fatalf("sample guard error=%v want_invalid=%t", err, tc.invalid)
			}
			if err == nil && (positive != tc.positive || zero != tc.zero) {
				t.Fatalf("eligible population positive=%d zero=%d want=%d/%d", positive, zero, tc.positive, tc.zero)
			}
		})
	}
}
