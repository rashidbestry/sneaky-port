package validation

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
	"sneaky-port/internal/worker"
	"sneaky-port/internal/writer"
)

const deterministicFaultStart = uint64(1e6)

// accountingRelease is a fixture-owned complete authenticated frame at an explicit
// synthetic delivery time; it carries no native scheduling or network timing claim.
type accountingRelease struct {
	release
	origin int
	data   bool
}

// faultAccounting retains both independent origin populations and the component's
// raw transitions. Real sockets, worker receive guards and SSH transfer are separate tests.
type faultAccounting struct {
	local   [2]engine.Result
	summary [2]measure.Summary
}

// accountingPlan preserves the named twelve-slot workload without reading a host
// clock. Explicit timestamps exercise production schedule/accounting transitions.
func accountingPlan() profile.Plan {
	p := testutil.Plan()
	p.Phases[0].Duration, p.Grace, p.Deadline = 600e6, 300e6, 60e6
	p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 20, 20
	p.Limits.MaxLag, p.SpikeThreshold = 1e9, 100e6
	return p
}

// accountingApply submits each explicit component observation through the same
// collector as live runs; any rejected transition fails before population checks.
func accountingApply(t *testing.T, c *engine.Collector, event worker.Event) {
	t.Helper()
	if err := c.Apply(event); err != nil {
		t.Fatalf("deterministic %s transition: %v", event.Kind, err)
	}
}

// accountingRun executes the existing seeded transform synchronously around actual
// probe Encode/Decode and production collectors. Fresh directional keys make prior-run
// bytes fail production authentication; the native fixture separately tests worker guards.
// Missed slots come from production Pattern.Due at an explicit delayed wake, never retry,
// a timing tolerance, or a change to the no-catch-up scheduling policy.
func accountingRun(t *testing.T, state *faultScenario, run byte, missed [2]uint64) faultAccounting {
	t.Helper()
	p := accountingPlan()
	digest, err := p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	auth := [2]probe.Auth{{Adapter: "udp", Plan: digest, Key: [32]byte{run, 1}}, {Adapter: "udp", Plan: digest, Key: [32]byte{run, 2}}}
	collectors := [2]*engine.Collector{}
	var pending []accountingRelease
	for origin := range collectors {
		collectors[origin], err = engine.NewCollector(p, byte(origin+1), deterministicFaultStart)
		if err != nil {
			t.Fatal(err)
		}
		windows, err := worker.PlanWindows(p, byte(origin+1))
		if err != nil || len(windows) != 1 {
			t.Fatalf("deterministic windows: %v %+v", err, windows)
		}
		window := windows[0]
		pattern := window.Local.Traffic.Pattern()
		wake, err := pattern.Offset(missed[origin], window.Local.Duration)
		if err != nil {
			t.Fatal(err)
		}
		end, send, err := pattern.Due(0, window.Local.Duration, wake, uint64(p.Limits.MaxLag))
		if err != nil || !send || end != missed[origin]+1 {
			t.Fatalf("deterministic delayed wake: end=%d send=%v err=%v want skipped=%d", end, send, err, missed[origin])
		}
		base := probe.Frame{Kind: probe.Data, Direction: byte(origin + 1), Lifecycle: 2, Run: [16]byte{run}, Session: [16]byte{run, 3}, OriginEpoch: uint32(origin + 1)}
		if skipped := end - 1; skipped > 0 {
			accountingApply(t, collectors[origin], worker.Event{Kind: "schedule_skip", Frame: base, At: deterministicFaultStart + wake, Scheduled: deterministicFaultStart, Count: skipped, Reason: "scheduler_lag"})
		}
		for seq := end - 1; seq < faultSlots; seq++ {
			offset, err := pattern.Offset(seq, window.Local.Duration)
			if err != nil {
				t.Fatal(err)
			}
			scheduled := deterministicFaultStart + offset
			frame := base
			frame.Sequence, frame.Content = seq, window.Local.Traffic.Payload()
			completion := make(chan writer.Result, 1)
			completion <- writer.Result{Attempted: true, Bytes: probe.Overhead + len(frame.Content)}
			accountingApply(t, collectors[origin], worker.Event{Kind: "admission", Frame: frame, At: scheduled, Scheduled: scheduled, Count: 1, Admission: writer.Admission{State: writer.Accepted, Done: completion}})
			frame.Attempt = scheduled + 1000
			accountingApply(t, collectors[origin], worker.Event{Kind: "attempt", Frame: frame, At: frame.Attempt})
			accountingApply(t, collectors[origin], worker.Event{Kind: "write", Frame: frame, At: frame.Attempt + 500, Call: frame.Attempt, Bytes: probe.Overhead + len(frame.Content)})
			encoded, err := auth[origin].Encode(frame, probe.UDPMax)
			if err != nil {
				t.Fatal(err)
			}
			role := "server"
			if origin == 1 {
				role = "client"
			}
			for _, output := range state.transform(encoded, role, scheduled+2000) {
				pending = append(pending, accountingRelease{output, origin, true})
			}
		}
	}
	seenData := [2]map[uint64]bool{{}, {}}
	seenReceipts := [2]map[uint64]bool{{}, {}}
	for len(pending) > 0 {
		sort.SliceStable(pending, func(i, j int) bool { // Synthetic release time establishes order independently of host pauses.
			return pending[i].due < pending[j].due
		})
		output := pending[0]
		pending = pending[1:]
		destination, key, role := output.origin, 1-output.origin, "client"
		if output.data {
			destination, key, role = 1-output.origin, output.origin, "server"
		}
		if output.origin == 1 {
			if role == "server" {
				role = "client"
			} else {
				role = "server"
			}
		}
		state.record(output.bytes, role, output.action, time.Duration(output.due-output.seen))
		frame, err := auth[key].Decode(output.bytes, probe.UDPMax)
		if err != nil {
			accountingApply(t, collectors[destination], worker.Event{Kind: "invalid", At: output.due, Bytes: len(output.bytes), Err: err})
			continue
		}
		seen := seenReceipts[output.origin]
		if output.data {
			seen = seenData[output.origin]
		}
		kind := "arrival"
		if seen[frame.Sequence] {
			kind = "duplicate"
		}
		seen[frame.Sequence] = true
		accountingApply(t, collectors[destination], worker.Event{Kind: kind, Frame: frame, At: output.due, Bytes: len(output.bytes)})
		if !output.data || kind == "duplicate" {
			continue
		}
		frame.Kind, frame.Content = probe.Receipt, nil
		frame.PeerEpoch, frame.Arrival, frame.ReceiptAttempt = uint32(destination+1), output.due, output.due+1000
		encoded, err := auth[destination].Encode(frame, probe.UDPMax)
		if err != nil {
			t.Fatal(err)
		}
		role = "client"
		if output.origin == 1 {
			role = "server"
		}
		for _, receipt := range state.transform(encoded, role, frame.ReceiptAttempt+1000) {
			pending = append(pending, accountingRelease{receipt, output.origin, false})
		}
	}
	var result faultAccounting
	for origin, collector := range collectors {
		generation := deterministicFaultStart + p.Duration()
		cutoff := generation + uint64(p.Grace)
		accountingApply(t, collector, worker.Event{Kind: "generation_end", At: generation})
		accountingApply(t, collector, worker.Event{Kind: "observation_end", At: cutoff})
		result.local[origin], err = collector.Finish(nil, cutoff)
		if err != nil || !result.local[origin].Complete {
			t.Fatalf("deterministic finish: %v complete=%v", err, result.local[origin].Complete)
		}
		accountingReplay(t, p, result.local[origin])
	}
	for origin := range result.local {
		local, remote := result.local[origin], result.local[1-origin]
		result.summary[origin] = measure.SummarizeSnapshot(local.Entries, uint64(local.ObservationCutoff), &remote.Receiver, 2, nil)
	}
	return result
}

// accountingReplay reconstructs every raw record through production replay and
// compares complete ledgers/receiver maps/analysis, rather than cached summaries alone.
func accountingReplay(t *testing.T, p profile.Plan, original engine.Result) {
	t.Helper()
	replayer, err := engine.NewCollector(p, original.Direction, deterministicFaultStart)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range original.Records {
		var record engine.Record
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		if err := replayer.ReplayRecord(record); err != nil {
			t.Fatal(err)
		}
	}
	if replayed := replayer.ReplayFinish(original.GenerationCutoff, original.ObservationCutoff, original.Complete); !reflect.DeepEqual(replayed, original) {
		t.Fatalf("deterministic exact replay changed direction %d", original.Direction)
	}
}

// accountingPopulation requires exact planned/local/outcome denominators without
// deriving expected values from the summary under test or interpreting IP packet loss.
func accountingPopulation(t *testing.T, summary measure.Summary, skipped, received, missing, samples, unresolved, late, duplicates profile.Decimal) {
	t.Helper()
	written := profile.Decimal(faultSlots) - skipped
	if summary.Planned != faultSlots || summary.Skipped != skipped || summary.LocalMisses != skipped || summary.Accepted != written || summary.Attempted != written || summary.Written != written || summary.WriteFailed != 0 || summary.RTTSamples != samples || summary.Unresolved != unresolved || summary.OnTime != samples-late || summary.Late != late || summary.DuplicateReceipts != duplicates || summary.Received == nil || *summary.Received != received || summary.MissingAtReceiver == nil || *summary.MissingAtReceiver != missing || summary.TransportPacketLoss != nil {
		t.Fatalf("deterministic exact population mismatch: %+v", summary)
	}
}

// TestDeterministicFaultLossAccounting preserves the fixed four-data/two-receipt
// seed and every exact identity even when a hosted native scheduler would miss slots.
func TestDeterministicFaultLossAccounting(t *testing.T) {
	state := newScenario("loss", nil)
	wantData := map[uint64]bool{0: true, 2: true, 3: true, 8: true}
	wantReceipt := map[uint64]bool{1: true, 4: true}
	if !reflect.DeepEqual(state.dataDrops, wantData) || !reflect.DeepEqual(state.receiptDrops, wantReceipt) {
		t.Fatal("named seed changed its independently specified drop identities")
	}
	result := accountingRun(t, state, 1, [2]uint64{})
	accountingPopulation(t, result.summary[0], 0, 8, 4, 6, 6, 0, 0)
	accountingPopulation(t, result.summary[1], 0, 12, 0, 12, 0, 0, 0)
	for _, entry := range result.local[0].Entries {
		seq := uint64(entry.Identity.Sequence)
		_, arrived := result.local[1].Receiver.Arrivals[entry.Identity]
		if arrived != !wantData[seq] || (entry.Receipt != nil) != !(wantData[seq] || wantReceipt[seq]) || entry.State != "written" || entry.Attempt == nil {
			t.Fatalf("seeded loss identity changed: %+v arrived=%v", entry, arrived)
		}
	}
	dataDrops, receiptDrops := map[uint64]bool{}, map[uint64]bool{}
	for _, truth := range state.truth {
		if truth.Action != "drop" {
			continue
		}
		if truth.Role == "server" && truth.Kind == probe.Data {
			dataDrops[uint64(truth.Identity.Sequence)] = true
		} else if truth.Role == "client" && truth.Kind == probe.Receipt {
			receiptDrops[uint64(truth.Identity.Sequence)] = true
		} else {
			t.Fatalf("unexpected directional drop: %+v", truth)
		}
	}
	if !reflect.DeepEqual(dataDrops, wantData) || !reflect.DeepEqual(receiptDrops, wantReceipt) {
		t.Fatal("exact drop truth changed")
	}
}

// TestDeterministicFaultDelayAccounting requires every exact jitter/spike duration,
// two independent spike episodes and strict late classifications using synthetic times.
func TestDeterministicFaultDelayAccounting(t *testing.T) {
	result := accountingRun(t, newScenario("delay", nil), 2, [2]uint64{})
	accountingPopulation(t, result.summary[0], 0, 12, 0, 12, 0, 2, 0)
	accountingPopulation(t, result.summary[1], 0, 12, 0, 12, 0, 0, 0)
	for _, entry := range result.local[0].Entries {
		seq := uint64(entry.Identity.Sequence)
		hold := uint64(9+seq%3*2) * uint64(time.Millisecond)
		if seq == 3 || seq == 8 {
			hold = uint64(140 * time.Millisecond)
		}
		if entry.Attempt == nil || entry.Receipt == nil || uint64(*entry.Receipt-*entry.Attempt) != hold+3000 {
			t.Fatalf("exact selected release floor changed: %+v expected_rtt_ns=%d", entry, hold+3000)
		}
	}
	spikes := result.local[0].Analysis.Spikes
	if len(spikes) != 2 || spikes[0].First.Sequence != 3 || spikes[0].Last.Sequence != 3 || spikes[0].Count != 1 || spikes[1].First.Sequence != 8 || spikes[1].Last.Sequence != 8 || spikes[1].Count != 1 {
		t.Fatalf("exact known spike episodes changed: %+v", spikes)
	}
}

// TestDeterministicFaultDuplicateReorderAccounting requires all named faults on two
// runs. Fresh production Auth keys reject authentic previous-run bytes; native worker
// identity guards and SSH/report transfer remain covered by the real-socket fixture.
func TestDeterministicFaultDuplicateReorderAccounting(t *testing.T) {
	state := newScenario("duplicate", nil)
	for run := 0; run < 2; run++ {
		result := accountingRun(t, state, byte(3+run), [2]uint64{})
		accountingPopulation(t, result.summary[0], 0, 12, 0, 12, 0, 1, 1)
		accountingPopulation(t, result.summary[1], 0, 12, 0, 12, 0, 0, 0)
		arrivals := records(t, result.local[1], "arrival", probe.Data)
		positions := map[profile.Decimal]int{}
		for index, record := range arrivals {
			positions[record.Identity.Sequence] = index
		}
		dataDuplicates := records(t, result.local[1], "duplicate", probe.Data)
		receiptDuplicates := records(t, result.local[0], "duplicate", probe.Receipt)
		if len(arrivals) != faultSlots || positions[2] >= positions[1] || len(dataDuplicates) != 1 || dataDuplicates[0].Identity.Sequence != 4 || len(receiptDuplicates) != 1 || receiptDuplicates[0].Identity.Sequence != 5 || len(records(t, result.local[1], "invalid", 0)) != 2+run {
			t.Fatalf("named duplicate/reorder/authentication populations changed in run %d", run+1)
		}
		state.stale = bytes.Clone(state.previous)
		state.truth = nil
	}
}

// TestDeterministicFaultSchedulerSkips fixes explicit delayed-wake examples matching
// the hosted nine/one local skips, while the independent direction stays twelve.
// Production Pattern.Due supplies the skipped range; those slots create no attempts,
// receipts, receiver missing population or application packet-loss claim.
func TestDeterministicFaultSchedulerSkips(t *testing.T) {
	for _, missed := range [][2]uint64{{9, 0}, {0, 1}} {
		result := accountingRun(t, newScenario("none", nil), 5, missed)
		for origin, skipped := range missed {
			assertControlledPopulation(t, result.local[origin], result.local[1-origin], result.summary[origin])
			written := profile.Decimal(faultSlots - skipped)
			accountingPopulation(t, result.summary[origin], profile.Decimal(skipped), written, 0, written, 0, 0, 0)
			for _, entry := range result.local[origin].Entries {
				isSkipped := uint64(entry.Identity.Sequence) < skipped
				if isSkipped != (entry.State == "skipped_local") || isSkipped != (entry.Attempt == nil) || isSkipped != (entry.Receipt == nil) || isSkipped && entry.Reason != "scheduler_lag" {
					t.Fatalf("explicit scheduler skip identity changed: origin=%d entry=%+v", origin+1, entry)
				}
				_, arrived := result.local[1-origin].Receiver.Arrivals[entry.Identity]
				if arrived == isSkipped {
					t.Fatalf("local skip became peer arrival: origin=%d entry=%+v", origin+1, entry)
				}
			}
		}
	}
}
