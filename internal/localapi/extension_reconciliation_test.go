package localapi

import (
	"encoding/json"
	"fmt"
	"testing"

	"sneaky-port/internal/control"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// TestExtensionAccountingCutoffWitness requires precise raw evidence before allowing live unknowns.
// Synthetic boundaries verify the fixture guard; production reconciliation is exercised in control tests.
func TestExtensionAccountingCutoffWitness(t *testing.T) {
	for _, direction := range []byte{1, 2} {
		for _, fault := range []string{"compatible", "write_cutoff", "attempt_equal", "attempt_after", "receipt_equal", "receipt_after", "unknown", "known_cutoff", "invented_loss", "missing_raw", "raw_time", "raw_attempt", "raw_epoch", "raw_direction", "foreign", "missing_receipt_capture"} {
			t.Run(fmt.Sprintf("%d/%s", direction, fault), func(t *testing.T) { // Reject malformed or missing captures rather than treating them as scheduling evidence.
				id := measure.Identity{Lifecycle: 2}
				entry := measure.Entry{Identity: id, State: "written", Scheduled: 0, Attempt: decimalPointer(10), WriteCompleted: decimalPointer(20)}
				origin := &control.ReplayedReport{Evidence: engine.Result{Direction: direction, Epoch: 1, ObservationCutoff: 100, Entries: []measure.Entry{entry}}}
				peer := &control.ReplayedReport{Evidence: engine.Result{ObservationCutoff: 90, Receiver: measure.ReceiverEvidence{Cutoff: 90, Arrivals: map[measure.Identity]uint64{id: 50}}}}
				derived := &control.LocalRun{Summary: measure.Summary{Received: decimalPointer(1), MissingAtReceiver: decimalPointer(0)}}
				record := engine.Record{Kind: "arrival", FrameKind: probe.Data, Identity: id, Direction: direction, Epoch: 1, Attempt: 10, At: 50}
				wantWitness, wantError := false, false
				wantWarning := ""
				switch fault {
				case "write_cutoff":
					origin.Evidence.Entries[0].WriteCompleted = decimalPointer(100)
					wantWarning = "origin: failed/pending writes are excluded from missing-data claims; observed arrivals remain distinct"
				case "attempt_equal", "attempt_after", "known_cutoff":
					at := profile.Decimal(100)
					if fault == "attempt_after" {
						at++
					}
					origin.Evidence.Entries[0].Attempt, record.Attempt = &at, at
					wantWitness = true
					wantWarning = "origin: receiver contains unmatched or outside-cutoff identity"
					if fault == "known_cutoff" {
						wantError = true
					} else {
						derived.Summary.Received, derived.Summary.MissingAtReceiver = nil, nil
					}
				case "receipt_equal", "receipt_after":
					at := profile.Decimal(90)
					if fault == "receipt_after" {
						at++
					}
					origin.Evidence.Entries[0].Receipt = decimalPointer(30)
					record.At, peer.Evidence.Receiver.Arrivals[id] = at, uint64(at)
					derived.Summary.Received, derived.Summary.MissingAtReceiver = nil, nil
					wantWitness = true
					wantWarning = "origin: origin receipt contradicts complete receiver ledger"
				case "unknown":
					derived.Summary.Received, derived.Summary.MissingAtReceiver = nil, nil
					wantError = true
				case "invented_loss":
					zero := "0"
					derived.Summary.TransportPacketLoss = &zero
					wantError = true
				case "missing_raw":
					wantError = true
				case "raw_time":
					record.At++
					wantError = true
				case "raw_attempt":
					record.Attempt++
					wantError = true
				case "raw_epoch":
					record.Epoch++
					wantError = true
				case "raw_direction":
					record.Direction++
					wantError = true
				case "foreign":
					delete(peer.Evidence.Receiver.Arrivals, id)
					peer.Evidence.Receiver.Arrivals[measure.Identity{Lifecycle: 2, Sequence: 99}] = 50
					wantError = true
				case "missing_receipt_capture":
					origin.Evidence.Entries[0].Receipt = decimalPointer(30)
					delete(peer.Evidence.Receiver.Arrivals, id)
					wantError = true
				}
				if fault != "missing_raw" {
					b, err := json.Marshal(record)
					if err != nil {
						t.Fatal(err)
					}
					peer.Evidence.Records = []json.RawMessage{b}
				}
				witness, err := extensionDirectionAccounting(origin, peer, derived)
				if (err != nil) != wantError || witness != wantWitness {
					t.Fatalf("exact cutoff witness: witness=%t want=%t error=%v want_error=%t", witness, wantWitness, err, wantError)
				}
				if err == nil && extensionAccountingWarning(origin, peer, "origin", witness) != wantWarning {
					t.Fatalf("exact warning: actual=%s want=%s", extensionAccountingWarning(origin, peer, "origin", witness), wantWarning)
				}
			})
		}
	}
}
