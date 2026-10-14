package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
	"sneaky-port/internal/worker"
)

// finiteRig gives real SSH/carrier fixtures explicit schedulable windows, not product defaults.
func finiteRig(t *testing.T, kind string) *testRig {
	t.Helper()
	r := newRig(t, kind)
	r.plan.Warmup = 30e6
	r.plan.Phases[0].Duration = 100e6
	r.plan.Phases[0].Client.RateNumerator = 50
	r.plan.Phases[0].Server.RateNumerator = 25
	r.plan.Grace = 100e6
	r.plan.Deadline = 50e6
	r.plan.Limits.MaxLag = 30e6
	r.plan.Limits.HeartbeatInterval = 20e6
	// Success/lifecycle fixtures allow a burst of raw boundary records; saturation has dedicated tests.
	r.plan.Limits.QueueMessages = 128
	return r
}

// TestFiniteAuthenticatedRuns exercises real finite start/grace/stop, both families and repeated lease reuse.
func TestFiniteAuthenticatedRuns(t *testing.T) {
	for _, kind := range []string{"udp", "tcp", "udp6", "tcp6"} {
		t.Run(kind, func(t *testing.T) { // Both authentic carriers use the same finite coordinator/accounting.
			r := finiteRig(t, kind)
			for i := 0; i < 2; i++ {
				result, err := Run(context.Background(), r.client, r.signer, r.plan)
				if err != nil {
					t.Fatalf("run: %v; status=%s local=%+v server=%s", err, result.Status, result.Local, result.ServerStatus)
				}
				if result.Status != "completed" || result.ServerStatus != "completed" || result.Local == nil || !result.Local.Complete || !result.Readiness.MeasurementStarted || result.Local.Generation < 130e6 || result.Local.Observation != 230e6 || result.Local.Summary.Planned != 5 || result.Local.Warmup.Planned != 2 {
					t.Fatalf("finite lifecycle/accounting mismatch: %+v", result)
				}
				assertFiniteTimeline(t, r.plan, result)
				if result.Local.Summary.Received != nil || result.Local.Summary.TransportPacketLoss != nil || !strings.HasPrefix(result.Local.Summary.ReceiverStatus, "unknown") {
					t.Fatal("lifecycle acknowledgment invented reconciled delivery")
				}
				if result.Remote == nil || result.Remote.Status != "verified" || result.Reconciliation == nil || result.Reconciliation.Status != "reconciled" || result.Reconciliation.Client.Summary.TransportPacketLoss != nil {
					t.Fatal("authenticated remote transfer/reconciliation missing or packet loss invented")
				}
				// A delayed host may skip every outgoing frame, or only write during warmup; marking follows actual I/O.
				var attempts int
				for _, raw := range result.Evidence.Records {
					var record engine.Record
					if err := json.Unmarshal(raw, &record); err != nil {
						t.Fatal("invalid finite event:", err)
					}
					if record.Kind == "attempt" {
						attempts++
					}
				}
				if len(result.Local.Markings) > 2 || attempts > 0 && len(result.Local.Markings) == 0 || attempts == 0 && len(result.Local.Markings) != 0 {
					t.Fatal("physical phase API evidence missing from public snapshot")
				}
				for _, mark := range result.Local.Markings {
					if mark.Marking.Observed != nil || mark.Phase != 0 || mark.Lifecycle < 1 || mark.Lifecycle > 2 {
						t.Fatal("invalid physical phase API evidence")
					}
				}
				public, _ := json.Marshal(result)
				for _, secret := range []string{"client_key", "server_key", "challenge", "PRIVATE KEY"} {
					if bytes.Contains(public, []byte(secret)) {
						t.Fatal("secret in CLI snapshot")
					}
				}
				if _, err := os.Stat(filepath.Join(r.client.Root, "active-run.lock")); !os.IsNotExist(err) {
					t.Fatal("client reservation leaked")
				}
				assertPersistedRun(t, r, result, true)
			}
		})
	}
}

// assertFiniteTimeline checks raw stop evidence and the exact half-open reached population, including late wakes.
// Generation is an observed stop timestamp; observation metrics remain capped at the frozen planned grace end.
func assertFiniteTimeline(t *testing.T, p profile.Plan, result RunResult) {
	t.Helper()
	if result.Local == nil {
		t.Fatal("finite timeline has no local evidence")
	}
	var generation, observation profile.Decimal
	var generations, observations int
	for _, raw := range result.Evidence.Records {
		var record engine.Record
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal("invalid finite event:", err)
		}
		switch record.Kind {
		case "generation_end":
			generations++
			generation = record.At
		case "observation_end":
			observations++
			observation = record.At
		}
	}
	plannedGeneration := uint64(p.Warmup) + p.Duration()
	plannedObservation := plannedGeneration + uint64(p.Grace)
	if generations != 1 || observations != 1 || generation > observation || result.Local.Generation != generation || result.Evidence.GenerationCutoff != generation || uint64(result.Local.Observation) != min(uint64(observation), plannedObservation) || result.Evidence.ObservationCutoff != result.Local.Observation || result.Evidence.Receiver.Cutoff != uint64(result.Local.Observation) {
		t.Fatalf("finite raw cutoff mismatch: generation=%d observation=%d local_generation=%d local_observation=%d boundaries=%d/%d", generation, observation, result.Local.Generation, result.Local.Observation, generations, observations)
	}
	if result.Local.Complete != result.Evidence.Complete || result.Local.Complete != result.Evidence.Receiver.Complete || result.Local.Complete && (uint64(generation) < plannedGeneration || uint64(observation) < plannedObservation) {
		t.Fatal("finite completeness contradicts retained local cutoffs")
	}
	windows, err := worker.PlanWindows(p, 1)
	if err != nil {
		t.Fatal("invalid finite fixture plan:", err)
	}
	want := make(map[measure.Identity]uint64)
	var warmup, main uint64
	for _, window := range windows {
		var duration uint64
		if uint64(generation) > window.Offset {
			duration = min(uint64(generation)-window.Offset, window.Local.Duration)
		}
		count, err := window.Local.Traffic.Pattern().Count(duration)
		if err != nil {
			t.Fatal("invalid finite reached population:", err)
		}
		if window.Local.Lifecycle == 1 {
			warmup += count
		} else {
			main += count
		}
		for i := uint64(0); i < count; i++ {
			offset, err := window.Local.Traffic.Pattern().Offset(i, window.Local.Duration)
			if err != nil {
				t.Fatal("invalid finite slot offset:", err)
			}
			id := measure.Identity{Lifecycle: window.Local.Lifecycle, Phase: window.Local.Phase, Sequence: profile.Decimal(window.Local.FirstSequence + i)}
			want[id] = window.Offset + offset
		}
	}
	if result.Local.Warmup.Planned != profile.Decimal(warmup) || result.Local.Summary.Planned != profile.Decimal(main) || len(result.Evidence.Entries) != len(want) {
		t.Fatalf("finite reached population mismatch: warmup=%d/%d main=%d/%d entries=%d/%d", result.Local.Warmup.Planned, warmup, result.Local.Summary.Planned, main, len(result.Evidence.Entries), len(want))
	}
	for _, entry := range result.Evidence.Entries {
		scheduled, ok := want[entry.Identity]
		if !ok || uint64(entry.Scheduled) != scheduled || scheduled >= uint64(generation) {
			t.Fatal("finite identity omitted, duplicated or beyond half-open generation cutoff:", entry.Identity)
		}
		delete(want, entry.Identity)
	}
	if len(want) != 0 {
		t.Fatal("finite reached identities missing from local ledger")
	}
}

// TestFiniteEarlyCutoffPopulation exercises actual accounting at deterministic early boundaries without host timers.
// These fixtures prove exact interrupted populations; authenticated host tests separately own real I/O and cleanup.
func TestFiniteEarlyCutoffPopulation(t *testing.T) {
	for _, stage := range []struct {
		name                    string
		generation, observation uint64
		warmup, main            uint64
	}{{"warmup", 100e6, 125e6, 5, 0}, {"main", 350e6, 375e6, 13, 5}, {"grace", 500e6, 650e6, 13, 13}} {
		t.Run(stage.name, func(t *testing.T) { // Exact equality excludes the slot at the interruption boundary.
			p := testutil.Plan()
			p.Warmup, p.Phases[0].Duration, p.Grace = 250e6, 250e6, 250e6
			p.Phases[0].Client.RateNumerator = 50
			p.Phases[0].Server.RateNumerator = 25
			collector, err := engine.NewCollector(p, 1, 0)
			if err != nil {
				t.Fatal(err)
			}
			apply := func(event worker.Event) { // Exercise the production accounting transition with explicit synthetic evidence.
				if err := collector.Apply(event); err != nil {
					t.Fatal("early cutoff fixture event:", err)
				}
			}
			apply(worker.Event{Kind: "schedule_skip", Frame: probe.Frame{Kind: probe.Data, Direction: 1, Lifecycle: 1}, At: stage.generation, Scheduled: 0, Count: stage.warmup, Reason: "stopped_scheduler"})
			if stage.main > 0 {
				apply(worker.Event{Kind: "schedule_skip", Frame: probe.Frame{Kind: probe.Data, Direction: 1, Lifecycle: 2, Sequence: 13}, At: stage.generation, Scheduled: 250e6, Count: stage.main, Reason: "stopped_scheduler"})
			}
			apply(worker.Event{Kind: "generation_end", At: stage.generation})
			apply(worker.Event{Kind: "observation_end", At: stage.observation})
			evidence, err := collector.Finish(nil, stage.observation)
			if err != nil || evidence.Complete || evidence.Receiver.Complete || uint64(evidence.GenerationCutoff) != stage.generation || uint64(evidence.ObservationCutoff) != stage.observation || uint64(evidence.Warmup.Planned) != stage.warmup || uint64(evidence.Summary.Planned) != stage.main || evidence.Warmup.Skipped != evidence.Warmup.Planned || evidence.Summary.Skipped != evidence.Summary.Planned || evidence.Warmup.Attempted != 0 || evidence.Summary.Attempted != 0 {
				t.Fatalf("early cutoff promoted or changed population: %v evidence=%+v", err, evidence)
			}
			assertFiniteTimeline(t, p, RunResult{Local: localSnapshot(evidence), Evidence: evidence})
		})
	}
}

// afterRunProgress uses the first selected-clock snapshot to give the host timer a stage dispatch hint.
// Timer domains and scheduling do not prove execution time; the joined latest snapshot can be stale.
func afterRunProgress(t *testing.T, feed *measure.ProgressFeed, target time.Duration, action func()) func() *measure.Progress {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var fired *measure.Progress
	go func() { // Observe only this run-owned feed; progress does not change authoritative accounting or scheduling.
		defer close(done)
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			if progress := feed.Snapshot(); progress != nil {
				remaining := max(target-time.Duration(progress.Elapsed), 0)
				timer := time.NewTimer(remaining)
				defer timer.Stop()
				select {
				case <-ctx.Done():
				case <-timer.C:
					fired = feed.Snapshot()
					action()
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return func() *measure.Progress { // Stop and join the fixture trigger before reading its result or releasing stores.
		cancel()
		<-done
		return fired
	}
}

// TestFiniteCancellationAndControlLoss verifies authentic interruption or late processing against retained cutoffs.
// Requested stages are fixture targets; host scheduling does not guarantee actions execute within those stages.
func TestFiniteCancellationAndControlLoss(t *testing.T) {
	for _, kind := range []string{"udp", "tcp"} {
		for _, stage := range []string{"warmup", "main", "grace", "control_loss", "revocation"} {
			t.Run(kind+"/"+stage, func(t *testing.T) { // Target run-local elapsed time; persisted events can first appear well into a phase.
				r := finiteRig(t, kind)
				r.plan.Warmup = 250e6
				r.plan.Phases[0].Duration = 250e6
				r.plan.Grace = 250e6
				feed := &measure.ProgressFeed{}
				watchdog, stopWatchdog := context.WithTimeout(context.Background(), 30*time.Second)
				defer stopWatchdog()
				ctx, cancel := context.WithCancel(measure.WithProgress(watchdog, feed))
				defer cancel()
				delay := 100 * time.Millisecond
				if stage == "main" {
					delay = 350 * time.Millisecond
				} else if stage == "grace" {
					delay = 650 * time.Millisecond
				}
				stopTrigger := afterRunProgress(t, feed, delay, func() { // Target cancellation/control loss within this run's measurement epoch.
					if stage == "control_loss" {
						r.stop()
					} else if stage == "revocation" {
						if err := r.server.Disable("client"); err != nil {
							t.Error(err)
							cancel()
						}
					} else {
						cancel()
					}
				})
				defer stopTrigger()
				result, err := Run(ctx, r.client, r.signer, r.plan)
				trigger := stopTrigger()
				if watchdog.Err() != nil {
					t.Fatal("finite fixture exceeded its setup/run/cleanup liveness watchdog:", watchdog.Err())
				}
				if result.Local == nil || !result.Readiness.MeasurementStarted || err != nil && result.Status != "partial" || err == nil && (result.Status != "completed" || !result.Local.Complete) || trigger == nil && err != nil {
					t.Fatalf("cancel/loss result: %v status=%s local=%+v", err, result.Status, result.Local)
				}
				assertFiniteTimeline(t, r.plan, result)
				if trigger == nil {
					t.Logf("target=%s action not dispatched before completion; verified full cutoff and population", stage)
				} else {
					t.Logf("target=%s latest feed stage=%s elapsed=%d (may be stale); retained generation=%d observation=%d local_complete=%t status=%s", stage, trigger.Stage, trigger.Elapsed, result.Local.Generation, result.Local.Observation, result.Local.Complete, result.Status)
				}
				assertPersistedRun(t, r, result, result.Local.Complete)
				if stage != "control_loss" || trigger == nil {
					if stage == "revocation" && trigger != nil {
						client, err := r.client.Client()
						if err != nil {
							t.Fatal(err)
						}
						if err = r.server.Enroll(client.Enrollment); err != nil {
							t.Fatal(err)
						}
					}
					assertReadyAfterDrain(t, r)
				}
			})
		}
	}
}

// TestFiniteReservationAndBudget rejects concurrent client ownership and insufficient liveness capacity before I/O.
func TestFiniteReservationAndBudget(t *testing.T) {
	r := finiteRig(t, "udp")
	path := filepath.Join(r.client.Root, "active-run.lock")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if result, err := Run(context.Background(), r.client, r.signer, r.plan); err == nil || result.Readiness.MeasurementStarted {
		t.Fatal("client reservation ignored")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("another owner's reservation was removed")
	}
	os.Remove(path)
	r.plan.Limits.ControlRequests = 8
	if _, err := Run(context.Background(), r.client, r.signer, r.plan); err == nil {
		t.Fatal("heartbeat demand accepted beyond budget")
	}
}

// TestFiniteSetupCancellation releases the local reservation without claiming a started measurement.
func TestFiniteSetupCancellation(t *testing.T) {
	r := finiteRig(t, "udp")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Run(ctx, r.client, r.signer, r.plan)
	if err == nil || result.Readiness.MeasurementStarted || result.Local != nil || result.Status != "not_started" || result.Reason == "" {
		t.Fatal("setup cancellation fabricated a run:", err, result)
	}
	if _, err = Ready(context.Background(), r.client, r.signer, r.plan, 0); err != nil {
		t.Fatal("setup reservation leaked:", err)
	}
}

// TestRunHeartbeatSurfacesServerFailure exposes an early real TCP EOF as partial lifecycle, not inferred delivery.
func TestRunHeartbeatSurfacesServerFailure(t *testing.T) {
	r := finiteRig(t, "tcp")
	r.plan.Phases[0].Duration = 500e6
	c := r.sshClient(t, r.signer)
	ch, reqs, err := c.OpenChannel(ChannelType, nil)
	if err != nil {
		t.Fatal(err)
	}
	go ssh.DiscardRequests(reqs)
	offer := prepareFixture(t, r, ch, "")
	data := finishFixtureBind(t, r, ch, offer)
	var body offerBody
	DecodeObject(offer.Body, &body)
	Write(ch, message("start", "start-1", offer.RunID, offer.SessionID, hashBody{body.Hash}))
	response, err := Read(ch)
	if err != nil || response.Type != "started" {
		t.Fatal("start failed:", err)
	}
	data.Close()
	end := time.Now().Add(time.Second)
	for i := 0; ; i++ {
		Write(ch, message("heartbeat", fmt.Sprintf("beat-%d", i), offer.RunID, offer.SessionID, hashBody{body.Hash}))
		response, err = Read(ch)
		var ack finishAck
		if err != nil || response.Type != "run-alive" || DecodeObject(response.Body, &ack) != nil || ack.Hash != body.Hash {
			t.Fatal("invalid run heartbeat:", err)
		}
		if ack.Status == "partial" {
			break
		}
		if time.Now().After(end) {
			t.Fatal("server EOF hidden behind live control")
		}
		timer := time.NewTimer(10 * time.Millisecond)
		<-timer.C
	}
	Write(ch, message("stop", "stop-1", offer.RunID, offer.SessionID, hashBody{body.Hash}))
	response, err = Read(ch)
	var finished finishAck
	if err != nil || response.Type != "finished" || DecodeObject(response.Body, &finished) != nil || finished.Status != "partial" {
		t.Fatal("partial server termination missing:", err)
	}
	// This manual fixture declines report transfer; SSH close ends bounded ownership.
	c.Close()
	assertReadyAfterDrain(t, r)
}

// TestStartedLeaseAndStartIdentity checks missing heartbeats and invalid/repeated frozen-plan starts with real SSH.
func TestStartedLeaseAndStartIdentity(t *testing.T) {
	for _, cause := range []string{"lease", "wrong_hash", "second_start"} {
		t.Run(cause, func(t *testing.T) { // Every rejection/expired started lease must release the real listener and active slot.
			r := finiteRig(t, "tcp")
			r.plan.Phases[0].Duration = 300e6
			r.plan.Limits.Lease = 100e6
			c := r.sshClient(t, r.signer)
			ch, reqs, err := c.OpenChannel(ChannelType, nil)
			if err != nil {
				t.Fatal(err)
			}
			go ssh.DiscardRequests(reqs)
			offer := prepareFixture(t, r, ch, "")
			finishFixtureBind(t, r, ch, offer)
			var body offerBody
			DecodeObject(offer.Body, &body)
			hash := body.Hash
			if cause == "wrong_hash" {
				hash = "wrong"
			}
			Write(ch, message("start", "start-1", offer.RunID, offer.SessionID, hashBody{hash}))
			response, err := Read(ch)
			if err != nil || cause == "wrong_hash" && response.Type != "error" || cause != "wrong_hash" && response.Type != "started" {
				t.Fatalf("start response: %s %v", response.Type, err)
			}
			if cause == "second_start" {
				Write(ch, message("start", "start-2", offer.RunID, offer.SessionID, hashBody{hash}))
				response, err = Read(ch)
				if err != nil || response.Type != "error" {
					t.Fatal("second start accepted")
				}
			}
			wait := make(chan error, 1)
			go func() { // Join the real lease-controlled connection within a finite fixture bound.
				wait <- c.Wait()
			}()
			select {
			case <-wait:
			case <-time.After(time.Second):
				t.Fatal("started lease cleanup exceeded bound")
			}
			assertReadyAfterDrain(t, r)
		})
	}
}

// assertReadyAfterDrain distinguishes immediate SSH EOF from the permitted bounded data drain.
func assertReadyAfterDrain(t *testing.T, r *testRig) {
	t.Helper()
	end := time.Now().Add(time.Second)
	for {
		_, err := Ready(context.Background(), r.client, r.signer, r.plan, 0)
		if err == nil {
			return
		}
		if time.Now().After(end) || !strings.Contains(err.Error(), "busy") {
			t.Fatal("started listener/reservation leaked:", err)
		}
		timer := time.NewTimer(10 * time.Millisecond)
		<-timer.C
	}
}
