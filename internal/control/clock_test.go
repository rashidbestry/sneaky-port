package control

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/profile"
)

// clockPeer owns isolated authenticated/pinned SSH sockets for deterministic channel fault injection.
// It never changes an installed SSH service, clock, privilege or network policy.
func clockPeer(t *testing.T, handler func(context.Context, ssh.Channel)) *ssh.Client {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { // Accept exactly this fixture connection and join every calibration handler at cleanup.
		defer close(done)
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		stop := context.AfterFunc(ctx, func() { // Interrupt handshake and all channel work at fixture teardown.
			raw.Close()
		})
		defer stop()
		config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) { // Authenticate only the freshly generated fixture key.
			if !bytes.Equal(k.Marshal(), signer.PublicKey().Marshal()) {
				return nil, errors.New("wrong key")
			}
			return &ssh.Permissions{}, nil
		}}
		config.AddHostKey(signer)
		conn, channels, requests, err := ssh.NewServerConn(raw, config)
		if err != nil {
			return
		}
		defer conn.Close()
		var workers sync.WaitGroup
		defer workers.Wait()
		go ssh.DiscardRequests(requests)
		for incoming := range channels {
			if incoming.ChannelType() != CalibrationChannelType {
				incoming.Reject(ssh.Prohibited, "fixture calibration only")
				continue
			}
			ch, reqs, err := incoming.Accept()
			if err != nil {
				return
			}
			go ssh.DiscardRequests(reqs)
			workers.Add(1)
			go func() { // Calibration fault handlers cannot block unrelated channel requests.
				defer workers.Done()
				defer ch.Close()
				handler(ctx, ch)
			}()
		}
	}()
	client, err := ssh.Dial("tcp", ln.Addr().String(), &ssh.ClientConfig{User: "fixture", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()), Timeout: 2 * time.Second})
	if err != nil {
		cancel()
		ln.Close()
		<-done
		t.Fatal(err)
	}
	t.Cleanup(func() { // Close only fixture-owned sockets and verify that channel handlers exit.
		client.Close()
		cancel()
		ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("clock fixture worker leaked")
		}
	})
	return client
}

// syntheticCapture supplies deliberately distinct host epochs/offsets and exact >2^53-safe raw values.
func syntheticCapture(id int, at profile.Decimal) ClockCapture {
	return ClockCapture{ID: fmt.Sprintf("clock-%d", id), ClientEpoch: 1, ServerEpoch: 1, Scheduled: at, T1: at, T2: at + 1000, T3: at + 1010, T4: at + 100, ClientCosts: clockCosts{at + 1, at + 2, at + 3}, ServerCosts: clockCosts{at + 1011, at + 1012, at + 1013}, Finished: at + 110, Sent: 100, Received: 200, Valid: true}
}

// TestClockWindowAndExpiry checks exact expiry edges, invalid retention and bounded recent populations.
func TestClockWindowAndExpiry(t *testing.T) {
	_, p, _ := evidenceFixture(t)
	p.Calibration.Samples, p.Calibration.Window = 2, 3
	p.Calibration.Cadence, p.Calibration.Expiry = 100, 300
	captures := []ClockCapture{syntheticCapture(1, 10), syntheticCapture(2, 110), syntheticCapture(3, 210), syntheticCapture(4, 310)}
	captures[2].ServerEpoch = 2
	captures[2].Valid = false
	captures[2].Reason = validateCapture(captures[2])
	s := clockSnapshot(p, captures, 410)
	if s.Attempts != 4 || s.Valid != 3 || s.Invalid != 1 || s.RecentValid != 2 || len(s.Recent) != 3 || s.Recent[0].ID != "clock-2" || s.Status != "samples_available" || s.Age == nil || *s.Age != 0 {
		t.Fatalf("window: %+v", s)
	}
	s = clockSnapshot(p, captures, 710)
	if s.Expired || s.RecentValid != 1 || s.Status != "unavailable" {
		t.Fatalf("exact expiry: %+v", s)
	}
	s = clockSnapshot(p, captures, 711)
	if !s.Expired || s.RecentValid != 0 || s.Reason != "clock samples expired" {
		t.Fatalf("expired: %+v", s)
	}
	bad := syntheticCapture(5, 9007199254740993)
	bad.ClientCosts.WriteStart = bad.T1 - 1
	if validateCapture(bad) == "" {
		t.Fatal("reversed send order accepted")
	}
	if validateCapture(syntheticCapture(6, 9007199254740993)) != "" {
		t.Fatal("distinct host offsets rejected")
	}
}

// TestClockFraming rejects oversized/truncated/ambiguous frames and retains complete-arrival boundaries.
func TestClockFraming(t *testing.T) {
	anchor := time.Now()
	var b bytes.Buffer
	cost, err := clockWrite(&b, message("clock", "clock-1", "run", "session", clockRequest{"hash", 1, 0}), anchor)
	if err != nil || cost.Encoded > cost.WriteStart || cost.WriteStart > cost.WriteEnd {
		t.Fatal("send boundaries", err)
	}
	encoded := bytes.Clone(b.Bytes())
	e, at, err := clockRead(&b, anchor)
	if err != nil || e.RequestID != "clock-1" || at < cost.WriteEnd {
		t.Fatal("arrival boundary", err)
	}
	for _, raw := range [][]byte{encoded[:3], encoded[:len(encoded)-1], {0, 0, 0, 0}, {0, 0, 0x20, 0}} {
		if _, at, err = clockRead(bytes.NewReader(raw), anchor); err == nil || at != 0 {
			t.Fatal("invalid frame accepted or invented complete arrival")
		}
	}
	if _, err = clockWrite(io.Discard, message("clock", "id", "run", "session", strings.Repeat("x", clockFrameMax)), anchor); err == nil {
		t.Fatal("oversized write accepted")
	}
	duplicate := []byte(`{"schema_version":1,"schema_version":1}`)
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(duplicate)))
	if _, _, err = clockRead(bytes.NewReader(append(prefix[:], duplicate...)), anchor); err == nil {
		t.Fatal("duplicate fields accepted")
	}
}

// TestLiveClockRuns verifies real authenticated IPv4/IPv6 UDP/TCP, replay and three export formats.
// Assertions cover evidence/populations, not a numerical host-clock accuracy claim.
func TestLiveClockRuns(t *testing.T) {
	for _, kind := range []string{"udp", "tcp", "udp6", "tcp6"} {
		t.Run(kind, func(t *testing.T) { // Each run has explicit time-separated samples and a bounded recent window.
			r := finiteRig(t, kind)
			r.plan.Phases[0].Duration = 300e6
			r.plan.Calibration.Cadence = 50e6
			r.plan.Calibration.Window = 3
			result, err := Run(context.Background(), r.client, r.signer, r.plan)
			if err != nil {
				t.Fatal(err)
			}
			if result.Clock == nil || result.Clock.Valid < 2 || result.Clock.Attempts > clockBudget(r.plan) || len(result.Clock.Recent) > 3 || result.Clock.Status == "calibrated" || result.Clock.Traffic.Sent == 0 || result.Clock.Traffic.Received == 0 {
				t.Fatalf("live captures: %+v", result.Clock)
			}
			for _, c := range result.Clock.Recent {
				if c.Valid && validateCapture(c) != "" || c.Scheduled%r.plan.Calibration.Cadence != 0 {
					t.Fatal("invalid raw boundaries/cadence", c)
				}
			}
			replay, err := ReplayReport(result.ReportPath, 8<<20)
			if err != nil {
				t.Fatal(err)
			}
			if replay.Clock == nil || !reflect.DeepEqual(replay.Clock, result.Clock) || replay.Verification != "matched" {
				t.Fatal("replay changed captures")
			}
			if result.Reconciliation == nil || result.Reconciliation.OneWay == nil || !reflect.DeepEqual(result.Reconciliation.OneWay, replay.Reconciliation.OneWay) || result.Reconciliation.OneWay.Calibrated != 0 || result.Reconciliation.OneWay.Estimated == 0 {
				t.Fatal("live one-way estimates/replay missing or unverified bounds promoted")
			}
			for _, format := range []string{"json", "jsonl", "csv"} {
				var b bytes.Buffer
				if err = ExportReport(&b, replay, format); err != nil || !bytes.Contains(b.Bytes(), []byte("calibration_id")) || !bytes.Contains(b.Bytes(), []byte("t1_ns")) || !bytes.Contains(b.Bytes(), []byte("lower_ns")) || !bytes.Contains(b.Bytes(), []byte("one_way")) {
					t.Fatal("clock export", format, err)
				}
			}
			server, err := ReplayReport(filepath.Join(r.server.Root, "reports", result.Readiness.RunID+"-server"), 8<<20)
			if err != nil || server.Clock != nil {
				t.Fatal("server invented client-origin capture set", err)
			}
			assertPersistedRun(t, r, result, true)
		})
	}
}

// TestClockFaultsAndSkippedCadence injects delayed, wrong-epoch, mismatched and stalled SSH responses.
// Calibration failures retain evidence and leave an independent authenticated SSH request usable.
func TestClockFaultsAndSkippedCadence(t *testing.T) {
	for _, kind := range []string{"delay", "epoch", "identity", "stall"} {
		t.Run(kind, func(t *testing.T) { // No network impairment settings are changed; only the fixture peer response is delayed.
			store, p, ready := evidenceFixture(t)
			p.Phases[0].Duration = 240e6
			p.Calibration.Cadence = 20e6
			p.Limits.WriteTimeout = 500e6
			p.Phases[0].Client.RateNumerator = 50
			p.Phases[0].Server.RateNumerator = 25
			hash, _ := p.Hash()
			ready.PlanHash = hex.EncodeToString(hash[:])
			anchors := make(chan time.Time, 1)
			client := clockPeer(t, func(ctx context.Context, ch ssh.Channel) { // Fault at the actual complete-request/response boundary.
				serverAnchor := (<-anchors).Add(-time.Second)
				for i := 0; i < clockBudget(p); i++ {
					e, t2, err := clockRead(ch, serverAnchor)
					if err != nil {
						return
					}
					var req clockRequest
					if DecodeObject(e.Body, &req) != nil {
						return
					}
					if kind == "stall" {
						<-ctx.Done()
						return
					}
					if kind == "delay" && i == 0 {
						time.Sleep(55 * time.Millisecond)
					}
					epoch := uint32(1)
					if kind == "epoch" && i == 0 {
						epoch = 2
					}
					id := e.RequestID
					if kind == "identity" {
						id = "foreign"
					}
					cost, err := clockWrite(ch, message("clock", id, e.RunID, e.SessionID, clockResponse{req.Hash, epoch, req.T1, t2, clockNow(serverAnchor)}), serverAnchor)
					if err != nil {
						return
					}
					if _, err = clockWrite(ch, message("clock-costs", id, e.RunID, e.SessionID, cost), serverAnchor); err != nil {
						return
					}
				}
			})
			w, err := newEvidence(store, p, ready, "client")
			if err != nil {
				t.Fatal(err)
			}
			// Fixture setup and fsync are outside the sampler's local measurement epoch.
			anchor := time.Now()
			anchors <- anchor
			s := startClock(context.Background(), client, anchor, p, ready, w)
			select {
			case <-s.done:
			case <-time.After(2 * time.Second):
				s.stop()
				t.Fatal("sampler did not finish")
			}
			snap := s.window.snapshot(clockNow(anchor))
			if kind == "delay" && (snap.Skipped < 1 || snap.Valid < 2) || kind == "epoch" && (snap.Invalid != 1 || snap.Valid < 2) || (kind == "identity" || kind == "stall") && (snap.Invalid != 1 || snap.Attempts != 1) {
				t.Fatalf("fault state: %+v", snap)
			}
			if _, _, err = client.OpenChannel("still-alive", nil); err == nil || !strings.Contains(err.Error(), "fixture calibration only") {
				t.Fatal("sampler closed shared SSH connection", err)
			}
			collector, err := engine.NewCollector(p, 1, 0)
			if err != nil {
				t.Fatal(err)
			}
			if err = w.Finalize(collector.ReplayFinish(0, 0, false)); err != nil {
				t.Fatal(err)
			}
			replay, err := ReplayReport(w.path, 4<<20)
			if err != nil {
				t.Fatal(err)
			}
			if replay.Clock == nil || replay.Clock.Attempts != snap.Attempts || !reflect.DeepEqual(replay.Clock.Recent, snap.Recent) {
				t.Fatal("invalid capture lost on replay")
			}
		})
	}
}

// TestClockRawBudget isolates calibration admission from data reserves and preserves invalid raw fields.
func TestClockRawBudget(t *testing.T) {
	store, p, ready := evidenceFixture(t)
	w, err := newEvidence(store, p, ready, "client")
	if err != nil {
		t.Fatal(err)
	}
	c := syntheticCapture(1, 9007199254740993)
	c.ServerEpoch = 7
	c.Valid = false
	c.Reason = validateCapture(c)
	if err = w.appendClock(c); err != nil {
		t.Fatal(err)
	}
	w.clockLimit = w.clockCharged
	if err = w.appendClock(c); err == nil {
		t.Fatal("clock byte cap ignored")
	}
	raw, _ := json.Marshal(engine.Record{Kind: "invalid", At: 2})
	if err = w.Append(raw); err != nil {
		t.Fatal("calibration consumed data reserve", err)
	}
	collector, err := engine.NewCollector(p, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = collector.ReplayRecord(engine.Record{Kind: "invalid", At: 2}); err != nil {
		t.Fatal(err)
	}
	if err = w.Finalize(collector.ReplayFinish(0, 3, false)); err != nil {
		t.Fatal(err)
	}
	r, err := ReplayReport(w.path, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if r.Clock == nil || r.Clock.Invalid != 1 || r.Clock.Recent[0].T1 != c.T1 || r.Clock.Recent[0].ServerEpoch != 7 {
		t.Fatal("invalid or large raw value changed")
	}
}

// TestClockChannelAndRequestGates verifies readiness/stop, single-channel and frozen identity gates over real SSH.
func TestClockChannelAndRequestGates(t *testing.T) {
	for _, fault := range []string{"run", "session", "hash", "epoch", "replay", "budget"} {
		t.Run(fault, func(t *testing.T) { // Channel faults must leave the independently validated lifecycle channel usable.
			r := finiteRig(t, "udp")
			client := r.sshClient(t, r.signer)
			defer client.Close()
			control, requests, err := client.OpenChannel(ChannelType, nil)
			if err != nil {
				t.Fatal(err)
			}
			go ssh.DiscardRequests(requests)
			offer := prepareFixture(t, r, control, "")
			finishFixtureBind(t, r, control, offer)
			if ch, _, err := client.OpenChannel(CalibrationChannelType, nil); err == nil {
				ch.Close()
				t.Fatal("calibration admitted before start")
			}
			hash, _ := r.plan.Hash()
			hashText := hex.EncodeToString(hash[:])
			if err = Write(control, message("start", "start", offer.RunID, offer.SessionID, hashBody{hashText})); err != nil {
				t.Fatal(err)
			}
			ack, err := Read(control)
			if err != nil || ack.Type != "started" {
				t.Fatal("start", err, string(ack.Body))
			}
			anchor := time.Now()
			clock, requests, err := client.OpenChannel(CalibrationChannelType, nil)
			if err != nil {
				t.Fatal(err)
			}
			go ssh.DiscardRequests(requests)
			if extra, _, err := client.OpenChannel(CalibrationChannelType, nil); err == nil {
				extra.Close()
				t.Fatal("duplicate calibration channel admitted")
			}
			if extra, _, err := client.OpenChannel(CalibrationChannelType, []byte("unexpected")); err == nil {
				extra.Close()
				t.Fatal("extra channel data admitted")
			}
			count := 1
			if fault == "replay" {
				count = 2
			}
			if fault == "budget" {
				count = clockBudget(r.plan) + 1
			}
			for i := 1; i <= count; i++ {
				run, session, id, requestedHash, epoch := offer.RunID, offer.SessionID, fmt.Sprintf("clock-%d", i), hashText, uint32(1)
				if fault == "run" {
					run = strings.Repeat("ef", 16)
				}
				if fault == "session" {
					session = strings.Repeat("ef", 16)
				}
				if fault == "hash" {
					requestedHash = strings.Repeat("00", 32)
				}
				if fault == "epoch" {
					epoch = 2
				}
				if fault == "replay" && i == 2 {
					id = "clock-1"
				}
				// Identity/budget fixtures use explicitly ordered synthetic client offsets, avoiding clock quantization ties.
				_, writeErr := clockWrite(clock, message("clock", id, run, session, clockRequest{requestedHash, epoch, profile.Decimal(i)}), anchor)
				_, _, readErr := clockRead(clock, anchor)
				wantValid := fault == "replay" && i == 1 || fault == "budget" && i < count
				if wantValid {
					if writeErr != nil || readErr != nil {
						t.Fatal("valid request rejected", i, "of", clockBudget(r.plan), writeErr, readErr)
					}
					if _, _, err = clockRead(clock, anchor); err != nil {
						t.Fatal("cost evidence", err)
					}
				} else if readErr == nil {
					t.Fatal("invalid/replayed/over-budget request accepted")
				}
			}
			clock.Close()
			if err = Write(control, message("heartbeat", "heartbeat", offer.RunID, offer.SessionID, hashBody{hashText})); err != nil {
				t.Fatal(err)
			}
			ack, err = Read(control)
			if err != nil || ack.Type != "run-alive" {
				t.Fatal("calibration fault cancelled lifecycle", err)
			}
			if err = Write(control, message("stop", "stop", offer.RunID, offer.SessionID, hashBody{hashText})); err != nil {
				t.Fatal(err)
			}
			ack, err = Read(control)
			if err != nil || ack.Type != "finished" {
				t.Fatal("stop", err)
			}
			if extra, _, err := client.OpenChannel(CalibrationChannelType, nil); err == nil {
				extra.Close()
				t.Fatal("post-stop calibration accepted")
			}
		})
	}
}

// clockReleaseCloser exposes a shared-transport close that releases otherwise uncancellable SSH IO.
type clockReleaseCloser struct {
	once    sync.Once
	release chan struct{}
}

// Close releases only this injected transport and is safe for concurrent lifecycle cleanup.
func (c *clockReleaseCloser) Close() error {
	c.once.Do(func() { // One close unblocks every simulated shared SSH write/accept/read.
		close(c.release)
	})
	return nil
}

// blockedClockChannel models the pinned SSH library's blocking channel-close/read boundaries.
type blockedClockChannel struct {
	release     <-chan struct{}
	readEntered chan struct{}
}

// Read exposes the wait before shared transport closure interrupts a complete-frame read.
func (c *blockedClockChannel) Read(_ []byte) (int, error) {
	close(c.readEntered)
	<-c.release
	return 0, io.EOF
}

// Write exposes the shared transport blockage rather than claiming accepted application bytes.
func (c *blockedClockChannel) Write(_ []byte) (int, error) { <-c.release; return 0, io.ErrClosedPipe }

// Close waits for transport closure, matching an SSH close behind a blocked transport write.
func (c *blockedClockChannel) Close() error { <-c.release; return nil }

// CloseWrite reuses the same injected SSH submission boundary.
func (c *blockedClockChannel) CloseWrite() error { return c.Close() }

// SendRequest rejects arbitrary fixture requests without adding another blocked worker.
func (c *blockedClockChannel) SendRequest(_ string, _ bool, _ []byte) (bool, error) {
	return false, io.EOF
}

// Stderr supplies the interface's extended stream without introducing a real side channel.
func (c *blockedClockChannel) Stderr() io.ReadWriter { return c }

// blockedClockIncoming places the injected blockage at acceptance or at accepted channel IO.
type blockedClockIncoming struct {
	blockAccept bool
	entered     chan struct{}
	transport   *clockReleaseCloser
	channel     *blockedClockChannel
}

// ChannelType advertises only the owned application calibration channel.
func (c *blockedClockIncoming) ChannelType() string { return CalibrationChannelType }

// ExtraData carries no unexpected channel-open metadata.
func (c *blockedClockIncoming) ExtraData() []byte { return nil }

// Accept signals the boundary before optionally waiting for shared transport teardown.
func (c *blockedClockIncoming) Accept() (ssh.Channel, <-chan *ssh.Request, error) {
	close(c.entered)
	if c.blockAccept {
		<-c.transport.release
		return nil, nil, net.ErrClosed
	}
	requests := make(chan *ssh.Request)
	close(requests)
	return c.channel, requests, nil
}

// Reject has no peer in this local boundary fixture and releases no unrelated resources.
func (c *blockedClockIncoming) Reject(_ ssh.RejectionReason, _ string) error { return nil }

// TestClockGateBlockedIO verifies bounded shutdown without holding the gate mutex across SSH submission.
// This injected boundary evidence is distinct from an independently impaired network case.
func TestClockGateBlockedIO(t *testing.T) {
	for _, blockAccept := range []bool{true, false} {
		t.Run(fmt.Sprintf("accept=%v", blockAccept), func(t *testing.T) { // Both pending acceptance and blocked channel-close require shared transport teardown.
			_, p, ready := evidenceFixture(t)
			p.Limits.ShutdownTimeout = 20e6
			raw := &clockReleaseCloser{release: make(chan struct{})}
			ch := &blockedClockChannel{release: raw.release, readEntered: make(chan struct{})}
			incoming := &blockedClockIncoming{blockAccept: blockAccept, entered: make(chan struct{}), transport: raw, channel: ch}
			gate := &clockGate{raw: raw}
			gate.start(time.Now(), ready, p)
			gate.accept(context.Background(), incoming)
			<-incoming.entered
			if !blockAccept {
				<-ch.readEntered
			}
			done := make(chan struct{})
			go func() { // Observe bounded gate shutdown without letting a regression deadlock the test process.
				gate.stop()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				raw.Close()
				<-done
				t.Fatal("blocked clock gate exceeded shutdown budget")
			}
			select {
			case <-raw.release:
			default:
				t.Fatal("blocked shared transport was not closed")
			}
		})
	}
}

// TestClockRetentionFailure preserves a calibration-only admission failure in reserved final-cutoff bytes.
func TestClockRetentionFailure(t *testing.T) {
	store, p, ready := evidenceFixture(t)
	p.Phases[0].Duration = 200e6
	p.Phases[0].Client.RateNumerator = 50
	p.Phases[0].Server.RateNumerator = 25
	hash, _ := p.Hash()
	ready.PlanHash = hex.EncodeToString(hash[:])
	w, err := newEvidence(store, p, ready, "client")
	if err != nil {
		t.Fatal(err)
	}
	w.clockLimit = 0
	anchors := make(chan time.Time, 1)
	client := clockPeer(t, func(_ context.Context, ch ssh.Channel) { // Use actual framing; only evidence admission is intentionally unavailable.
		serveClock(ch, <-anchors, ready, p)
	})
	anchor := time.Now()
	anchors <- anchor
	s := startClock(context.Background(), client, anchor, p, ready, w)
	select {
	case <-s.done:
	case <-time.After(time.Second):
		s.stop()
		t.Fatal("retention failure did not stop sampler")
	}
	if s.window.totals.Attempts != 0 || s.terminal != "calibration evidence budget/storage unavailable" {
		t.Fatal("unsaved capture claimed retained")
	}
	collector, err := engine.NewCollector(p, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Finalize(collector.ReplayFinish(0, 0, false)); err != nil {
		t.Fatal("calibration failure invalidated disk evidence", err)
	}
	replay, err := ReplayReport(w.path, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Clock == nil || replay.Clock.Attempts != 0 || replay.Clock.Reason != s.terminal || replay.Verification != "matched" {
		t.Fatal("retention gap concealed on replay")
	}
}
