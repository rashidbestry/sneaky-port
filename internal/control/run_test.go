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
				if result.Status != "completed" || result.ServerStatus != "completed" || result.Local == nil || !result.Local.Complete || !result.Readiness.MeasurementStarted || result.Local.Generation != 130e6 || result.Local.Observation != 230e6 || result.Local.Summary.Planned != 5 || result.Local.Warmup.Planned != 2 {
					t.Fatalf("finite lifecycle/accounting mismatch: %+v", result)
				}
				if result.Local.Summary.Received != nil || result.Local.Summary.TransportPacketLoss != nil || !strings.HasPrefix(result.Local.Summary.ReceiverStatus, "unknown") {
					t.Fatal("lifecycle acknowledgment invented reconciled delivery")
				}
				if len(result.Local.Markings) != 2 || result.Local.Markings[0].Lifecycle != 1 || result.Local.Markings[1].Lifecycle != 2 || result.Local.Markings[0].Marking.Observed != nil {
					t.Fatal("physical phase API evidence missing from public snapshot")
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

// TestFiniteCancellationAndControlLoss preserves partial cutoffs and bounds cancellation in warmup/main/grace.
func TestFiniteCancellationAndControlLoss(t *testing.T) {
	for _, kind := range []string{"udp", "tcp"} {
		for _, stage := range []string{"warmup", "main", "grace", "control_loss", "revocation"} {
			t.Run(kind+"/"+stage, func(t *testing.T) { // Observe lifecycle via the run lock and allow generous timer variation in fixtures.
				r := finiteRig(t, kind)
				r.plan.Warmup = 250e6
				r.plan.Phases[0].Duration = 250e6
				r.plan.Grace = 250e6
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				delay := 100 * time.Millisecond
				if stage == "main" {
					delay = 350 * time.Millisecond
				} else if stage == "grace" {
					delay = 650 * time.Millisecond
				}
				stopTrigger := afterRunEvidence(t, r.client.Root, delay, func() { // Trigger cancellation/control loss only after this run's measurement actually starts.
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
				begin := time.Now()
				result, err := Run(ctx, r.client, r.signer, r.plan)
				if err == nil || result.Status != "partial" || result.Local == nil || result.Local.Complete || result.Evidence.Receiver.Complete || time.Since(begin) > 2*time.Second {
					t.Fatalf("cancel/loss result: %v status=%s local=%+v", err, result.Status, result.Local)
				}
				if stage != "grace" && result.Local.Generation >= 500e6 || result.Local.Observation > 750e6 {
					t.Fatal("interrupted run manufactured a full timeline")
				}
				assertPersistedRun(t, r, result, false)
				if stage != "control_loss" {
					if stage == "revocation" {
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
