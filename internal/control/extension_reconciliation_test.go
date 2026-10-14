package control

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil/extension"
)

// TestRegisteredExtensionReconciliation retains an exact positive accounting gate without host timers.
// Synthetic paired collectors, restricted persistence and production replay do not establish a live carrier.
func TestRegisteredExtensionReconciliation(t *testing.T) {
	store, p, ready := evidenceFixture(t)
	registry := adapter.Default()
	if err := extension.Register(registry); err != nil {
		t.Fatal(err)
	}
	p.Adapter = extension.ID
	p.Options = map[string]json.RawMessage{"enabled": json.RawMessage("true")}
	p.Phases[0].Duration, p.Grace = 2_000_000, 2_000_000
	p.Deadline = 1_000_000
	hash, err := p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	ready.PlanHash = hex.EncodeToString(hash[:])
	collectors := make([]*engine.Collector, 2)
	for i := range collectors {
		collectors[i], err = engine.NewCollector(p, byte(i+1), 0, registry)
		if err != nil {
			t.Fatal(err)
		}
	}
	apply := func(index int, record engine.Record) { // Apply exact synthetic local observations through shared collector transitions.
		t.Helper()
		if err := collectors[index].ReplayRecord(record); err != nil {
			t.Fatalf("endpoint=%d record=%s identity=%+v: %v", index+1, record.Kind, record.Identity, err)
		}
	}
	for i := range collectors {
		for sequence := 0; sequence < 2; sequence++ {
			base := profile.Decimal(sequence * 1_000_000)
			r := engine.Record{Identity: measure.Identity{Lifecycle: 2, Sequence: profile.Decimal(sequence)}, FrameKind: probe.Data, Direction: byte(i + 1), Epoch: 1}
			r.Kind, r.At, r.Scheduled, r.Count, r.Admission = "admission", base+1, base, 1, "accepted"
			apply(i, r)
			r.Kind, r.At, r.Attempt = "attempt", base+10, base+10
			apply(i, r)
			r.Kind, r.At, r.Call, r.Bytes = "write", base+20, base+10, profile.Decimal(probe.Overhead+4+p.Phases[0].Client.ContentBytes)
			apply(i, r)
			r.Kind, r.At = "arrival", base+30
			apply(1-i, r)
			if sequence == 0 {
				// The other datum is delivered without an origin receipt; no delivery is inferred from that absence.
				r.Kind, r.FrameKind, r.At, r.PeerEpoch = "arrival", probe.Receipt, base+40, 1
				r.PeerArrival, r.PeerAttempt, r.Bytes = base+30, base+35, probe.Overhead+4
				apply(i, r)
			}
		}
	}
	reports := make([]*ReplayedReport, 2)
	for i, endpoint := range []string{"client", "server"} {
		result := collectors[i].ReplayFinish(2_000_000, 4_000_000, true)
		writer, err := newEvidence(store, p, ready, endpoint)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range result.Records {
			if err := writer.Append(raw); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Finalize(result); err != nil {
			t.Fatal(err)
		}
		if _, err := ReplayReport(writer.path, 4<<20); err == nil {
			t.Fatal("unregistered extension silently substituted during replay")
		}
		reports[i], err = ReplayReport(writer.path, 4<<20, registry)
		if err != nil || reports[i].Status != "completed" || reports[i].Verification != "matched" || reports[i].Manifest.Plan.Adapter != extension.ID || string(reports[i].Manifest.Plan.Options["enabled"]) != "true" {
			t.Fatalf("registered endpoint replay: endpoint=%s error=%v", endpoint, err)
		}
	}
	r := reconcileReports(reports[0], reports[1])
	if r.Status != "reconciled" || len(r.Warnings) != 0 {
		t.Fatalf("exact registered reconciliation: status=%s reason=%s warnings=%v", r.Status, r.Reason, r.Warnings)
	}
	for i, endpoint := range []*LocalRun{r.Client, r.Server} {
		s := endpoint.Summary
		if s.Planned != 2 || s.Written != 2 || s.OnTime != 1 || s.Unresolved != 1 || s.RTTSamples != 1 || s.Received == nil || *s.Received != 2 || s.MissingAtReceiver == nil || *s.MissingAtReceiver != 0 || s.TransportPacketLoss != nil || endpoint.Warmup.Planned != 0 {
			t.Fatalf("exact registered direction=%d summary=%+v", i+1, s)
		}
		if reports[i].Evidence.Summary.Received != nil || reports[i].Evidence.Summary.MissingAtReceiver != nil {
			t.Fatal("derived reconciliation rewrote endpoint-only delivery")
		}
	}
}
