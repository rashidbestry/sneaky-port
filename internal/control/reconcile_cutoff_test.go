package control

import (
	"bytes"
	"encoding/json"
	"maps"
	"reflect"
	"testing"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// reconciliationArrival retains one exact peer observation independently of the origin cutoff.
// These snapshots are synthetic; authenticated persistence/replay remains covered by real runs.
func reconciliationArrival(t *testing.T, origin, peer *ReplayedReport, index int, at profile.Decimal) {
	t.Helper()
	e := origin.Evidence.Entries[index]
	peer.Evidence.Receiver.Arrivals[e.Identity] = uint64(at)
	b, err := json.Marshal(engine.Record{Kind: "arrival", FrameKind: probe.Data, Identity: e.Identity, Direction: origin.Evidence.Direction, Epoch: origin.Evidence.Epoch, Attempt: *e.Attempt, At: at})
	if err != nil {
		t.Fatal(err)
	}
	peer.Evidence.Records = append(peer.Evidence.Records, b)
}

// TestReconciliationRetainedCutoffArrivals excludes equality and later reads from both directions.
// Outside-window raw evidence is retained; no receipt or within-window arrival is fabricated.
func TestReconciliationRetainedCutoffArrivals(t *testing.T) {
	for _, direction := range []string{"client", "server"} {
		for _, boundary := range []string{"before", "equal", "after"} {
			t.Run(direction+"/"+boundary, func(t *testing.T) { // Apply the same exact half-open population contract to independent endpoint clocks.
				client, server := reconcileFixture()
				origin, peer := client, server
				if direction == "server" {
					server.Evidence.Entries = append([]measure.Entry(nil), client.Evidence.Entries...)
					for i := 0; i < 2; i++ {
						reconciliationArrival(t, server, client, i, profile.Decimal(50+i))
					}
					origin, peer = server, client
				}
				at := profile.Decimal(peer.Evidence.Receiver.Cutoff)
				wantReceived, wantMissing := profile.Decimal(2), profile.Decimal(1)
				if boundary == "before" {
					at--
					wantReceived, wantMissing = 3, 0
				} else if boundary == "after" {
					at++
				}
				reconciliationArrival(t, origin, peer, 2, at)
				beforeMaps := [2]map[measure.Identity]uint64{maps.Clone(client.Evidence.Receiver.Arrivals), maps.Clone(server.Evidence.Receiver.Arrivals)}
				snapshots := []engine.Result{client.Evidence, server.Evidence}
				for i := range snapshots {
					// Identity-keyed maps have no JSON representation; compare their copied contents separately.
					snapshots[i].Receiver.Arrivals = nil
				}
				before, err := json.Marshal(snapshots)
				if err != nil {
					t.Fatal(err)
				}
				r := reconcileReports(client, server)
				result := r.Client
				if direction == "server" {
					result = r.Server
				}
				if r.Status != "reconciled" || result.Summary.Received == nil || *result.Summary.Received != wantReceived || result.Summary.MissingAtReceiver == nil || *result.Summary.MissingAtReceiver != wantMissing || result.Summary.OnTime != 1 || result.Summary.Unresolved != 2 || result.Summary.TransportPacketLoss != nil || len(r.Warnings) != 0 {
					t.Fatalf("strict independent receiver population: status=%s reason=%s received=%v missing=%v summary=%+v warnings=%v", r.Status, r.Reason, result.Summary.Received, result.Summary.MissingAtReceiver, result.Summary, r.Warnings)
				}
				afterMaps := [2]map[measure.Identity]uint64{client.Evidence.Receiver.Arrivals, server.Evidence.Receiver.Arrivals}
				snapshots = []engine.Result{client.Evidence, server.Evidence}
				for i := range snapshots {
					snapshots[i].Receiver.Arrivals = nil
				}
				after, err := json.Marshal(snapshots)
				if err != nil || !bytes.Equal(before, after) || !reflect.DeepEqual(beforeMaps, afterMaps) {
					t.Fatal("reconciliation rewrote retained endpoint evidence", err)
				}
			})
		}
	}
}

// TestReconciliationCutoffContradictions keeps incompatible eligible identities and receipts unknown.
// Discarding outside-window observations must never hide an in-window delivery contradiction.
func TestReconciliationCutoffContradictions(t *testing.T) {
	for _, fault := range []string{"foreign", "unattempted", "attempt", "attempt_cutoff", "receipt_equal", "receipt_after"} {
		t.Run(fault, func(t *testing.T) { // Require exact identity/attempt consistency wherever receiver evidence is eligible.
			client, server := reconcileFixture()
			switch fault {
			case "foreign":
				server.Evidence.Receiver.Arrivals[measure.Identity{Lifecycle: 2, Sequence: 99}] = 60
			case "unattempted":
				reconciliationArrival(t, client, server, 2, 60)
				client.Evidence.Entries[2].Attempt = nil
			case "attempt":
				reconciliationArrival(t, client, server, 2, 60)
				var record engine.Record
				if err := json.Unmarshal(server.Evidence.Records[2], &record); err != nil {
					t.Fatal(err)
				}
				record.Attempt++
				b, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				server.Evidence.Records[2] = b
			case "attempt_cutoff":
				at := client.Evidence.ObservationCutoff
				client.Evidence.Entries[2].Attempt = &at
				reconciliationArrival(t, client, server, 2, 60)
			case "receipt_equal", "receipt_after":
				at := profile.Decimal(server.Evidence.Receiver.Cutoff)
				if fault == "receipt_after" {
					at++
				}
				reconciliationArrival(t, client, server, 2, at)
				receipt := profile.Decimal(30)
				client.Evidence.Entries[2].Receipt = &receipt
			}
			r := reconcileReports(client, server)
			if r.Status != "unknown" || r.Client.Summary.Received != nil || r.Client.Summary.MissingAtReceiver != nil || r.Client.Summary.TransportPacketLoss != nil || len(r.Warnings) == 0 {
				t.Fatalf("incompatible evidence became delivery: status=%s reason=%s client=%+v warnings=%v", r.Status, r.Reason, r.Client.Summary, r.Warnings)
			}
		})
	}
}
