package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// TestRealPhaseTCP reconciles independent IPv4/IPv6 warmup/burst/quiet schedules and framing bytes.
// These are authentic loopback fixtures; caller-supplied cancellation/grace is not task 11 lifecycle.
func TestRealPhaseTCP(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6"} {
		t.Run(family, func(t *testing.T) { // Exercise the same phase/accounting contract on both authentic stream families.
			p := phasePlan()
			p.Adapter = "tcp"
			p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
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
			parent, stop := context.WithCancel(context.Background())
			defer stop()
			client, err := NewTCP(parent, c, p)
			if err != nil {
				t.Fatal(err)
			}
			server, err := NewTCP(parent, s, p)
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
			stop()
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
				if !r.Complete || r.Summary.TransportPacketLoss != nil || r.Summary.Received != nil || r.Summary.Planned == 0 || r.Summary.Planned != r.Summary.Skipped+r.Summary.Attempted || r.Summary.Accepted != r.Summary.Written || r.Summary.Accepted != r.Summary.RTTSamples {
					t.Fatalf("reconcile %+v complete=%v", r.Summary, r.Complete)
				}
				for _, e := range r.Entries {
					if e.State == "written" {
						if e.PartialBytes != profile.Decimal(probe.Overhead+4+p.Phases[e.Identity.Phase].Client.ContentBytes) && i == 0 && e.Identity.Lifecycle == 2 {
							t.Fatal("TCP byte boundary", e.PartialBytes)
						}
						if _, ok := peer.Receiver.Arrivals[e.Identity]; !ok {
							t.Fatalf("written datum absent from peer: %+v", e.Identity)
						}
					}
				}
			}
		})
	}
}

// heldStream injects a local writer pause; it models no network loss or system-wide impairment.
type heldStream struct {
	adapter.Transport
	gate chan struct{}
	once sync.Once
}

// Write waits while the independent scheduler/reader continue, then performs a real framed write.
func (h *heldStream) Write(b []byte, budget time.Duration) (int, error) {
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case <-h.gate:
	case <-timer.C:
		return 0, context.DeadlineExceeded
	}
	return h.Transport.Write(b, budget)
}

// release removes the injected local pause exactly once, including cleanup paths.
func (h *heldStream) release() {
	h.once.Do(func() { // Release every waiting write without closing twice.
		close(h.gate)
	})
}

// Close releases the pause and interrupts only the transferred real stream.
func (h *heldStream) Close() error { h.release(); return h.Transport.Close() }

// TestTCPStallAndRecovery retains all reached slots, late complete receipts and unknown packet loss.
func TestTCPStallAndRecovery(t *testing.T) {
	p := phasePlan()
	p.Adapter = "tcp"
	p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("false")}
	p.Warmup = 0
	p.Deadline = 20e6
	p.Grace = 250e6
	p.Phases = p.Phases[:1]
	p.Phases[0].Duration = 180e6
	p.Phases[0].Client.RateNumerator = 100
	p.Phases[0].Server.RateNumerator = 30
	p.Limits.QueueMessages = 4
	p.Limits.ReceiptReserve = 1
	p.Limits.MaxLag = 1e9
	c, s, p := realPair(t, p)
	held := &heldStream{Transport: c.Transport, gate: make(chan struct{})}
	c.Transport = held
	parent, stop := context.WithCancel(context.Background())
	defer stop()
	client, err := NewTCP(parent, c, p)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewTCP(parent, s, p)
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { // Join both endpoints on every assertion failure.
		stop()
		held.release()
		client.Close()
		server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		client.Wait(ctx)
		server.Wait(ctx)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
	held.release()
	timer := time.NewTimer(time.Until(c.Start.Add(time.Duration(p.Duration() + uint64(p.Grace)))))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stop()
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
	n, _ := p.Phases[0].Client.Pattern().Count(uint64(p.Phases[0].Duration))
	if !cr.Complete || !sr.Complete || uint64(cr.Summary.Planned) != n || cr.Summary.Skipped == 0 || cr.Summary.Accepted != cr.Summary.Written || cr.Summary.Late == 0 || len(cr.Analysis.Stalls) == 0 || len(cr.Receiver.Arrivals) == 0 || sr.Summary.Written == 0 {
		t.Fatalf("stalled stream client=%+v server=%+v analysis=%+v", cr.Summary, sr.Summary, cr.Analysis)
	}
	// Only the fixture can correlate both in-memory results; production remote reconciliation is task 14.
	ledger, _ := measure.NewLedger(p.Limits.LedgerMessages)
	for _, e := range cr.Entries {
		if err := ledger.Schedule(e.Identity, uint64(e.Scheduled), uint64(e.Deadline)); err != nil {
			t.Fatal(err)
		}
		if e.Attempt != nil {
			ledger.Attempt(e.Identity, uint64(*e.Attempt))
		}
	}
	reconciled := ledger.Summarize(uint64(cr.ObservationCutoff), &sr.Receiver)
	if reconciled.Received == nil || reconciled.TransportPacketLoss != nil || cr.Summary.Received != nil || cr.Summary.TransportPacketLoss != nil {
		t.Fatal("application delivery became packet-loss proof")
	}
}

// TestTCPPartialAccounting retains terminal bytes/tickets and interrupted phase cutoffs.
func TestTCPPartialAccounting(t *testing.T) {
	p := phasePlan()
	p.Adapter = "tcp"
	p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
	p.Warmup = 100e6
	p.Limits.MaxLag = 1e9
	c, _, p := realPair(t, p)
	c.Transport = partialTransport{c.Transport}
	run, err := NewTCP(context.Background(), c, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Bound fixture cleanup after a terminal injected local write failure.
		run.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		run.Wait(ctx)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r, err := run.Wait(ctx)
	if !errors.Is(err, io.ErrShortWrite) || r.Complete || r.Receiver.Complete || r.Summary.TransportPacketLoss != nil || r.Summary.Received != nil {
		t.Fatal("partial accounting", err, r.Summary)
	}
	failed, tickets, invoked, admitted := 0, 0, 0, 0
	for _, e := range r.Entries {
		if e.State == "write_failed" {
			failed++
			if e.PartialBytes != 3 || e.Attempt == nil || e.Receipt != nil {
				t.Fatal("partial byte evidence lost")
			}
		}
	}
	for _, b := range r.Records {
		var rec Record
		if err := json.Unmarshal(b, &rec); err != nil {
			t.Fatal(err)
		}
		if rec.Kind == "ticket_observed" {
			tickets++
			if rec.CallbackInvoked {
				invoked++
				if rec.Bytes != 3 {
					t.Fatal("active partial ticket evidence lost")
				}
			} else if rec.Bytes != 0 || rec.Reason == "" {
				t.Fatal("queued unattempted ticket evidence lost")
			}
		}
		if rec.Kind == "admission" && rec.Admission == "accepted" {
			admitted++
		}
	}
	// A busy host may admit more work before the first failure; retain every canceled ticket.
	if failed != 1 || invoked != 1 || tickets != admitted {
		t.Fatal("terminal write/ticket counts", failed, tickets)
	}
}
