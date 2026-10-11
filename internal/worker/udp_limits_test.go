package worker

import (
	"context"
	"math"
	"testing"
	"time"

	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/writer"
)

// TestUDPReceiptCorrelation proves only first matching receipts release outstanding budget;
// forged echoes and duplicates neither produce new samples nor bypass planned scheduling.
func TestUDPReceiptCorrelation(t *testing.T) {
	c, s := pair(t, "ipv4")
	c.Limits.Outstanding = 1
	c.Peer = fixtureSchedule(0)
	c.Local.Traffic.RateNumerator = 5
	c.Local.Duration = uint64(650 * time.Millisecond)
	u := start(t, c)
	l := collect(u)
	s.Transport.SetReadDeadline(time.Now().Add(time.Second))
	b, err := s.Transport.Read()
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.ReceiveAuth.Decode(b, probe.UDPMax)
	if err != nil {
		t.Fatal(err)
	}
	f.Kind = probe.Receipt
	f.Content = nil
	f.PeerEpoch = c.PeerEpoch
	f.Arrival = 7
	f.ReceiptAttempt = 8
	wrongAttempt := f
	wrongAttempt.Attempt++
	wrongEpoch := f
	wrongEpoch.PeerEpoch++
	for _, r := range []probe.Frame{wrongAttempt, wrongEpoch, f, f} {
		b, err := s.SendAuth.Encode(r, probe.UDPMax)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Transport.Write(b, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	until(t, func() bool { // Bound waiting for complete classifications and all planned local slots.
		return reached(l.snapshot()) == u.localCount && count(l.snapshot(), "arrival", probe.Receipt) == 1 && count(l.snapshot(), "duplicate", probe.Receipt) == 1
	})
	invalid, admitted, skipped := 0, 0, 0
	for _, e := range l.snapshot() {
		if e.Kind == "invalid" {
			invalid++
		}
		if e.Kind == "admission" && e.Frame.Kind == probe.Data {
			if e.Admission.State == writer.Accepted {
				admitted++
			}
			if e.Admission.State == writer.Skipped && e.Reason == "outstanding_data" {
				skipped++
			}
		}
	}
	if invalid != 2 || admitted != 2 || skipped != 2 {
		t.Fatalf("invalid=%d admitted=%d skipped=%d events=%+v", invalid, admitted, skipped, l.snapshot())
	}
}

// TestUDPRejectsConfig checks failures before ownership transfer and finite allocations.
func TestUDPRejectsConfig(t *testing.T) {
	c, _ := pair(t, "ipv4")
	mutations := []func(*UDPConfig){
		func(c *UDPConfig) { // Refuse a different real carrier; no silent UDP substitution.
			c.SendAuth.Adapter = "tcp"
		},
		func(c *UDPConfig) { // Physical traffic directions require different authenticated keys.
			c.ReceiveAuth.Key = c.SendAuth.Key
		},
		func(c *UDPConfig) { // Reject a missing session identity before creating workers.
			c.Session = [16]byte{}
		},
		func(c *UDPConfig) { // Reject omitted capacity rather than choosing an implicit budget.
			c.EventCapacity = 0
		},
		func(c *UDPConfig) { // Predicted attempts and duplicate tracking must fit the identity budget.
			c.Limits.LedgerMessages = 1
		},
		func(c *UDPConfig) { // Reject sequence overflow before deriving array indices.
			c.Local.FirstSequence = math.MaxUint64
		},
		func(c *UDPConfig) { // Reject invalid repeated content rather than panicking in Payload.
			c.Local.Traffic.ContentPattern = ""
		},
		func(c *UDPConfig) { // Reject content that cannot fit an authenticated UDP datagram.
			c.Peer.Traffic.ContentBytes = probe.UDPMax
		},
		func(c *UDPConfig) { // Reject a future epoch that would create unsigned timestamp underflow.
			c.EpochStart = time.Now().Add(time.Hour)
			c.Start = c.EpochStart
		},
		func(c *UDPConfig) { // Queue-specific validation still governs the real writer connection.
			c.Limits.ReceiptReserve = c.Limits.QueueMessages
		},
		func(c *UDPConfig) { // Reject a duration outside the local monotonic range.
			c.Local.Duration = uint64(math.MaxInt64) + 1
		},
	}
	for i, mutate := range mutations {
		bad := c
		mutate(&bad)
		if u, err := NewUDP(context.Background(), bad); err == nil {
			u.Close()
			t.Fatalf("invalid case %d accepted", i)
		}
	}
	b, _ := c.SendAuth.Encode(probe.Frame{Kind: probe.BindRequest, Direction: 1, Content: make([]byte, 32)}, probe.UDPMax)
	if _, err := c.Transport.Write(b, time.Second); err != nil {
		t.Fatalf("rejection took socket ownership: %v", err)
	}
}

// TestUDPCancelBeforeStart releases blocked reads and a future timer without creating slots.
func TestUDPCancelBeforeStart(t *testing.T) {
	c, _ := pair(t, "ipv4")
	c.Start = time.Now().Add(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u, err := NewUDP(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	join, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := u.Wait(join); err != nil {
		t.Fatal(err)
	}
	for e := range u.Events() {
		t.Fatalf("planned after cancellation: %+v", e)
	}
	if got := u.queue.Snapshot(); got != (writer.Usage{}) {
		t.Fatalf("retained queue: %+v", got)
	}
}

// TestUDPBoundedWait exposes a still-active component without canceling it implicitly.
func TestUDPBoundedWait(t *testing.T) {
	c, _ := pair(t, "ipv4")
	c.Start = time.Now().Add(time.Hour)
	c.Limits.MaxLag = profile.Decimal(time.Second)
	u := start(t, c)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := u.Wait(ctx); err != context.Canceled {
		t.Fatalf("unbounded wait: %v", err)
	}
}
