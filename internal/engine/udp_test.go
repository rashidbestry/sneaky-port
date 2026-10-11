package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"testing"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
	"sneaky-port/internal/worker"
	"sneaky-port/internal/writer"
)

// phasePlan supplies synthetic ordered warmup/steady/burst/quiet inputs, never product defaults.
func phasePlan() profile.Plan {
	p := testutil.Plan()
	p.Warmup = 2e6
	p.Deadline = 2e6
	p.Grace = 4e6
	p.SpikeThreshold = 200
	a := p.Phases[0].Client
	b := a
	b.RateNumerator = 500
	d := profile.Decimal(4e6)
	x := a
	x.Mode = "burst"
	x.RateNumerator = 2000
	x.Active = 1e6
	x.Gap = 1e6
	x.ContentBytes = 7
	x.ContentPattern = "eHg="
	y := x
	y.RateNumerator = 1000
	quiet := a
	quiet.Mode = "quiet"
	quiet.RateNumerator = 0
	p.Phases = []profile.Phase{{ID: "steady", Duration: 3e6, Client: a, Server: b}, {ID: "burst", Duration: 5e6, Client: x, Server: y, Deadline: &d}, {ID: "quiet", Duration: 2e6, Client: quiet, Server: quiet}}
	return p
}

// apply requires every deterministic event transition to succeed before checking summaries.
func apply(t *testing.T, c *Collector, e worker.Event) {
	t.Helper()
	if err := c.Apply(e); err != nil {
		t.Fatalf("%s: %v", e.Kind, err)
	}
}

// ticketResult creates an already resolved admission fixture with explicit callback evidence.
func ticketResult(result writer.Result) <-chan writer.Result {
	ch := make(chan writer.Result, 1)
	ch <- result
	return ch
}

// TestPhaseAccounting reconciles original identities, changed payload windows and strict cutoffs exactly.
func TestPhaseAccounting(t *testing.T) {
	p := phasePlan()
	c, err := NewCollector(p, 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.windows) != 4 || c.windows[0].Local.Lifecycle != 1 || c.windows[1].Local.FirstSequence != 2 || c.windows[2].Local.FirstSequence != 5 || c.windows[3].Local.FirstSequence != 11 || c.windows[2].Deadline != 4e6 {
		t.Fatalf("windows %+v", c.windows)
	}
	// Mutating caller-owned settings cannot rewrite the collector's plan/analysis policy.
	p.Phases[0].ID = "changed"
	p.MatchedMetadata["changed"] = "yes"
	if c.plan.Phases[0].ID != "steady" || len(c.plan.MatchedMetadata) != 0 {
		t.Fatal("plan not frozen")
	}
	for _, w := range c.windows {
		n, _ := w.Local.Traffic.Pattern().Count(w.Local.Duration)
		for k := uint64(0); k < n; k++ {
			offset, _ := w.Local.Traffic.Pattern().Offset(k, w.Local.Duration)
			scheduled := c.start + w.Offset + offset
			f := probe.Frame{Kind: probe.Data, Direction: 1, Lifecycle: w.Local.Lifecycle, Phase: w.Local.Phase, Sequence: w.Local.FirstSequence + k, OriginEpoch: 1}
			if f.Sequence == 3 || f.Sequence == 6 {
				apply(t, c, worker.Event{Kind: "schedule_skip", Frame: f, At: scheduled + 10, Scheduled: scheduled, Count: 1, Reason: "scheduler_lag"})
				continue
			}
			result := writer.Result{Attempted: true, Bytes: 128}
			if f.Sequence == 4 {
				result.Bytes = 3
				result.Err = io.ErrShortWrite
				result.Reason = writer.WriteFailure
			}
			apply(t, c, worker.Event{Kind: "admission", Frame: f, At: scheduled + 10, Scheduled: scheduled, Count: 1, Admission: writer.Admission{State: writer.Accepted, Done: ticketResult(result)}})
			f.Attempt = scheduled + 20
			apply(t, c, worker.Event{Kind: "attempt", Frame: f, At: f.Attempt})
			receipt := f
			receipt.Kind = probe.Receipt
			receipt.PeerEpoch = 2
			receipt.Arrival = 100
			receipt.ReceiptAttempt = 110
			at := f.Attempt + 100
			if f.Sequence == 0 {
				at = scheduled + 3e6
			}
			if f.Sequence == 2 {
				at = scheduled + 4e6
			}
			if f.Sequence == 10 {
				at = c.finalEnd
			}
			if f.Sequence != 4 {
				// A valid receipt can race ahead of local write completion without losing its sample.
				apply(t, c, worker.Event{Kind: "arrival", Frame: receipt, At: at, Bytes: 124})
				if f.Sequence == 2 {
					apply(t, c, worker.Event{Kind: "duplicate", Frame: receipt, At: at + 50, Bytes: 124})
				}
			}
			apply(t, c, worker.Event{Kind: "write", Frame: f, At: f.Attempt + 30, Call: f.Attempt + 5, Bytes: result.Bytes, Err: result.Err})
		}
	}
	// Peer delivery is independent of this origin's unresolved/lost receipts.
	peer := probe.Frame{Kind: probe.Data, Direction: 2, Lifecycle: 2, Phase: 0, Sequence: 1}
	apply(t, c, worker.Event{Kind: "arrival", Frame: peer, At: 3000, Bytes: 128})
	apply(t, c, worker.Event{Kind: "duplicate", Frame: peer, At: 3001, Bytes: 128})
	apply(t, c, worker.Event{Kind: "generation_end", At: c.plannedEnd})
	apply(t, c, worker.Event{Kind: "observation_end", At: c.finalEnd + 500})
	r, err := c.Finish(nil, c.finalEnd+1000)
	if err != nil {
		t.Fatal(err)
	}
	s := r.Summary
	if !r.Complete || s.Planned != 9 || s.Skipped != 2 || s.Accepted != 7 || s.Attempted != 7 || s.Written != 6 || s.WriteFailed != 1 || s.OnTime != 4 || s.Late != 1 || s.Unresolved != 2 || s.DeadlineMisses != 5 || s.AcceptedMisses != 3 || s.RTTSamples != 5 || s.DuplicateReceipts != 1 {
		t.Fatalf("summary %+v complete=%v", s, r.Complete)
	}
	if r.Warmup.Planned != 2 || r.Warmup.Late != 1 || r.Phases[0].Planned != 3 || r.Phases[1].Planned != 6 || r.Phases[2].Planned != 0 || len(r.Receiver.Arrivals) != 1 || r.Summary.Received != nil || r.Summary.TransportPacketLoss != nil {
		t.Fatal("warmup/phase/delivery semantics")
	}
	if len(r.Analysis.Spikes) != 1 || r.Analysis.Spikes[0].First.Sequence != 2 || len(r.Analysis.Stalls) == 0 || *s.ScheduledP99 != "4000000" || s.CorrectedSamples != 5 {
		t.Fatalf("analysis %+v summary %+v", r.Analysis, s)
	}
	var charged uint64
	for _, b := range r.Records {
		charged += uint64(len(b) + 1)
		var rec Record
		if err = json.Unmarshal(b, &rec); err != nil {
			t.Fatal(err)
		}
	}
	if charged != uint64(r.EventBytes) || charged > uint64(c.plan.Limits.EventBytes) {
		t.Fatal("inexact byte budget")
	}
	assertReplayAccounting(t, c.plan, c.start, r)
}

// TestRangeInterruptionAndEvidenceLimits checks skipped ranges, canceled admissions and partial evidence.
func TestRangeInterruptionAndEvidenceLimits(t *testing.T) {
	c, err := NewCollector(phasePlan(), 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	f := probe.Frame{Kind: probe.Data, Direction: 1, Lifecycle: 1, Sequence: 0}
	apply(t, c, worker.Event{Kind: "schedule_skip", Frame: f, At: 1e6 + 1010, Scheduled: 1000, Count: 2, Reason: "scheduler_lag"})
	f.Lifecycle = 2
	f.Sequence = 2
	apply(t, c, worker.Event{Kind: "admission", Frame: f, At: 2e6 + 1010, Scheduled: 2e6 + 1000, Count: 1, Admission: writer.Admission{State: writer.Accepted, Done: ticketResult(writer.Result{Reason: writer.Canceled, Err: context.Canceled})}})
	apply(t, c, worker.Event{Kind: "generation_end", At: 2e6 + 1020})
	apply(t, c, worker.Event{Kind: "observation_end", At: 2e6 + 1030})
	r, err := c.Finish(nil, 2e6+1040)
	if err != nil {
		t.Fatal(err)
	}
	if r.Complete || r.Summary.Planned != 1 || r.Summary.Accepted != 1 || r.Summary.Pending != 1 || len(r.Entries) != 3 || r.Entries[2].State != "skipped_local" || r.Entries[2].Attempt != nil {
		t.Fatalf("interruption %+v entries=%+v", r.Summary, r.Entries)
	}
	assertReplayAccounting(t, c.plan, c.start, r)
	// Exhaustion is explicit and cannot grow retained raw bytes beyond the frozen cap.
	c, err = NewCollector(phasePlan(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10000; i++ {
		err = c.Apply(worker.Event{Kind: "invalid", At: uint64(i), Err: errors.New("fixture invalid")})
		if err != nil {
			break
		}
	}
	if err == nil || c.bytes > uint64(c.plan.Limits.EventBytes) {
		t.Fatal("evidence cap ignored")
	}
	c.err = err
	r, err = c.Finish(nil, 100)
	if err == nil || r.Complete || r.Receiver.Complete {
		t.Fatal("budget exhaustion reported complete")
	}
	if _, err = NewCollector(phasePlan(), 1, math.MaxInt64); err == nil {
		t.Fatal("epoch overflow accepted")
	}
}

// realPair prepares and fixture-authenticates actual carrier sockets without control/start negotiation.
func realPair(t *testing.T, p profile.Plan) (worker.UDPConfig, worker.UDPConfig, profile.Plan) {
	t.Helper()
	network, ip := "udp4", "127.0.0.1"
	if p.Family == "ipv6" {
		network, ip = "udp6", "::1"
	}
	p.ServerAddress = ip
	if p.Adapter == "tcp" {
		network = "tcp4"
		if p.Family == "ipv6" {
			network = "tcp6"
		}
		reserve, err := net.Listen(network, net.JoinHostPort(ip, "0"))
		if err != nil {
			t.Fatal(err)
		}
		p.DestinationPort = reserve.Addr().(*net.TCPAddr).Port
		reserve.Close()
	} else {
		reserve, err := net.ListenPacket(network, net.JoinHostPort(ip, "0"))
		if err != nil {
			t.Fatal(err)
		}
		p.DestinationPort = reserve.LocalAddr().(*net.UDPAddr).Port
		reserve.Close()
	}
	factory, err := adapter.Default().Get(p.Adapter)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	prepared, err := factory.Prepare(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Close only the fixture's listener after owned sessions are joined.
		prepared.Close()
	})
	client, err := factory.Dial(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Preserve cleanup on failed ownership transfer.
		client.Close()
	})
	hash, _ := p.Hash()
	forward := probe.Auth{Adapter: p.Adapter, Key: [32]byte{1}, Plan: hash}
	reverse := probe.Auth{Adapter: p.Adapter, Key: [32]byte{2}, Plan: hash}
	run, session := [16]byte{3}, [16]byte{4}
	bind, _ := forward.Encode(probe.Frame{Kind: probe.BindRequest, Direction: 1, Run: run, Session: session, Content: make([]byte, 32)}, probe.UDPMax)
	if _, err = client.Write(bind, time.Second); err != nil {
		t.Fatal(err)
	}
	server, err := prepared.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Release the fixture-authenticated candidate if construction fails.
		server.Close()
	})
	if _, err = server.Read(); err != nil {
		t.Fatal(err)
	}
	epoch := time.Now().Add(-time.Second)
	start := time.Now().Add(40 * time.Millisecond)
	c := worker.UDPConfig{Transport: client, SendAuth: forward, ReceiveAuth: reverse, Run: run, Session: session, Direction: 1, Epoch: 1, PeerEpoch: 2, EpochStart: epoch, Start: start, EventCapacity: 1024}
	s := c
	s.Transport = server
	s.Direction = 2
	s.SendAuth, s.ReceiveAuth = reverse, forward
	s.Epoch, s.PeerEpoch = 2, 1
	return c, s, p
}

// TestRealPhaseUDP reconciles independent IPv4/IPv6 loopback schedules without assuming zero host lag.
func TestRealPhaseUDP(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6"} {
		t.Run(family, func(t *testing.T) { // Exercise the same phase/accounting contract on both authentic datagram families.
			p := phasePlan()
			p.Family = family
			p.Warmup = 40e6
			p.Grace = 500e6
			p.Deadline = 100e6
			p.Limits.MaxLag = 10e6
			for i := range p.Phases {
				p.Phases[i].Duration *= 20
				// Explicit active/gap windows avoid accidentally offering one datum every 2 ms.
				if p.Phases[i].Client.Mode == "burst" {
					p.Phases[i].Client.Active = 20e6
					p.Phases[i].Client.Gap = 20e6
					p.Phases[i].Server.Active = 20e6
					p.Phases[i].Server.Gap = 20e6
				}
				p.Phases[i].Client.RateNumerator = 50
				p.Phases[i].Server.RateNumerator = 25
				if p.Phases[i].Client.Mode == "quiet" {
					p.Phases[i].Client.RateNumerator = 0
					p.Phases[i].Server.RateNumerator = 0
				}
				if p.Phases[i].Deadline != nil {
					d := profile.Decimal(100e6)
					p.Phases[i].Deadline = &d
				}
			}
			c, s, p := realPair(t, p)
			client, err := NewUDP(context.Background(), c, p)
			if err != nil {
				t.Fatal(err)
			}
			server, err := NewUDP(context.Background(), s, p)
			if err != nil {
				client.Close()
				t.Fatal(err)
			}
			t.Cleanup(func() { // Join real readers/writers even when a reconciliation assertion fails.
				client.Close()
				server.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				client.Wait(ctx)
				server.Wait(ctx)
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			select {
			case <-client.Scheduled():
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case <-server.Scheduled():
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			// The harness supplies grace; task 11 will negotiate/own this finite lifecycle.
			timer := time.NewTimer(time.Until(c.Start.Add(time.Duration(uint64(p.Warmup) + p.Duration() + uint64(p.Grace)))))
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			client.Close()
			server.Close()
			cr, err := client.Wait(ctx)
			if err != nil {
				t.Fatal(err)
			}
			sr, err := server.Wait(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for i, r := range []Result{cr, sr} {
				peer := sr
				if i == 1 {
					peer = cr
				}
				if !r.Complete || r.Summary.Planned == 0 || r.Summary.Planned != r.Summary.Skipped+r.Summary.Attempted || r.Summary.Accepted != r.Summary.Written || r.Summary.Accepted != r.Summary.RTTSamples {
					t.Fatalf("reconcile %+v complete=%v", r.Summary, r.Complete)
				}
				for _, e := range r.Entries {
					if e.State == "written" {
						if _, ok := peer.Receiver.Arrivals[e.Identity]; !ok {
							t.Fatalf("written datum absent from peer: %+v", e.Identity)
						}
					}
				}
			}
		})
	}
}

// partialTransport injects one local short datagram write; this is not network impairment evidence.
type partialTransport struct{ adapter.Transport }

// Write retains three accepted bytes and a terminal short-write error without emitting a real datum.
func (p partialTransport) Write(b []byte, budget time.Duration) (int, error) {
	return 3, io.ErrShortWrite
}

// TestEnginePartialWrite joins every owned worker/ticket and preserves truncated local observations.
func TestEnginePartialWrite(t *testing.T) {
	p := phasePlan()
	p.Warmup = 100e6
	p.Limits.MaxLag = 1e9
	c, _, p := realPair(t, p)
	c.Transport = partialTransport{c.Transport}
	u, err := NewUDP(context.Background(), c, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Ensure fixture-owned I/O is joined even when terminal assertions fail.
		u.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		u.Wait(ctx)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r, err := u.Wait(ctx)
	// Quantized completion and stop timestamps may be equal; strict cutoff summaries exclude equality,
	// while the retained entry/ticket still must preserve the terminal partial write exactly.
	if !errors.Is(err, io.ErrShortWrite) || r.Complete || r.Receiver.Complete {
		t.Fatalf("partial write err=%v complete=%v", err, r.Complete)
	}
	if uint64(r.GenerationCutoff) >= uint64(c.Start.Sub(c.EpochStart))+uint64(p.Warmup)+p.Duration() {
		t.Fatal("full schedule fabricated after failure")
	}
	failed := 0
	for _, e := range r.Entries {
		if e.State == "write_failed" {
			failed++
			if e.PartialBytes != 3 || e.Receipt != nil || e.Attempt == nil || e.WriteCompleted == nil {
				t.Fatal("partial byte/unknown delivery lost")
			}
		}
	}
	if failed != 1 {
		t.Fatal("terminal write entry missing")
	}
}

// TestEngineRejectsUnimplementedMarkingAndDigest makes unsupported transitions explicit before ownership.
func TestEngineRejectsUnimplementedMarkingAndDigest(t *testing.T) {
	p := phasePlan()
	c, _, p := realPair(t, p)
	bad := c
	bad.SendAuth.Plan = [32]byte{99}
	if _, err := NewUDP(context.Background(), bad, p); err == nil {
		t.Fatal("different plan digest accepted")
	}
	p.Phases[1].Client.DSCP = 1
	hash, _ := p.Hash()
	c.SendAuth.Plan, c.ReceiveAuth.Plan = hash, hash
	if _, err := NewUDP(context.Background(), c, p); err == nil {
		t.Fatal("per-phase marking silently ignored")
	}
	if _, err := c.Transport.Write([]byte("ownership retained"), time.Second); err != nil {
		t.Fatal("rejected engine closed caller transport")
	}
}
