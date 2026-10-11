package worker

import (
	"container/heap"
	"context"
	"testing"
	"time"

	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// TestOutstandingExpiryRetainsLateReceipts validates strict expiry and single release without timers.
func TestOutstandingExpiryRetainsLateReceipts(t *testing.T) {
	p := testutil.Plan()
	p.Phases[0].Duration = 2e6
	ws, err := PlanWindows(p, 1)
	if err != nil {
		t.Fatal(err)
	}
	c := UDPConfig{Windows: ws, Limits: p.Limits, Local: ws[0].Local, Peer: ws[0].Peer, Run: [16]byte{1}, Session: [16]byte{2}, Direction: 1, Epoch: 1, PeerEpoch: 2}
	runtime, n, m, err := prepareWindows(c, probe.UDPMax)
	if err != nil {
		t.Fatal(err)
	}
	u := &UDP{c: c, windows: runtime, localCount: n, peerCount: m, attempts: []attempt{{started: true, at: 3, active: true}, {started: true, at: 4, active: true}}, outstanding: 2}
	heap.Push(&u.expiries, expiry{20, 0})
	heap.Push(&u.expiries, expiry{10, 1})
	u.expireLocked(10)
	if u.outstanding != 2 {
		t.Fatal("deadline equality expired")
	}
	u.expireLocked(11)
	if u.outstanding != 1 || u.attempts[1].active {
		t.Fatal("deadline heap ordering")
	}
	f := probe.Frame{Kind: probe.Receipt, Direction: 1, Lifecycle: 2, Run: c.Run, Session: c.Session, OriginEpoch: 1, PeerEpoch: 2, Sequence: 1, Attempt: 4}
	duplicate, err := u.check(f)
	if err != nil || duplicate || u.outstanding != 1 {
		t.Fatal("late receipt released capacity twice")
	}
	duplicate, err = u.check(f)
	if err != nil || !duplicate || u.outstanding != 1 {
		t.Fatal("duplicate changed capacity")
	}
	u.expireLocked(21)
	if u.outstanding != 0 {
		t.Fatal("remaining deadline not released")
	}
	f.Sequence = 0
	f.Attempt = 3
	if _, err = u.check(f); err != nil || u.outstanding != 0 {
		t.Fatal("expired identity no longer matchable")
	}
}

// TestTimelineInterruptionAccountsReachedOnly checks stop truncation even in a quiet trailing phase.
func TestTimelineInterruptionAccountsReachedOnly(t *testing.T) {
	c, _ := pair(t, "ipv4")
	p := testutil.Plan()
	p.Phases[0].Duration = 1e9
	p.Phases[0].Client.RateNumerator = 100
	p.Phases[0].Server.Mode = "quiet"
	p.Phases[0].Server.RateNumerator = 0
	ws, err := PlanWindows(p, 1)
	if err != nil {
		t.Fatal(err)
	}
	c.Windows = ws
	c.Limits = p.Limits
	c.Limits.MaxLag = 10e6
	u := start(t, c)
	log := collect(u)
	until(t, func() bool { // Cancel only after at least one actual reached slot, independent of host wake precision.
		return reached(log.snapshot()) > 0
	})
	u.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = u.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	<-log.done
	stop := u.stoppedAt()
	elapsed := stop - uint64(c.Start.Sub(c.EpochStart))
	end, _, err := ws[0].Local.Traffic.Pattern().Due(0, ws[0].Local.Duration, elapsed-1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if reached(log.snapshot()) != end || end >= u.localCount {
		t.Fatalf("reached=%d expected=%d full=%d", reached(log.snapshot()), end, u.localCount)
	}
}

// TestTimelineValidation refuses disconnected intervals, reused ranges and omitted deadlines before ownership.
func TestTimelineValidation(t *testing.T) {
	c, _ := pair(t, "ipv4")
	p := testutil.Plan()
	p.Phases = append(p.Phases, profile.Phase{ID: "second", Duration: 1e6, Client: p.Phases[0].Client, Server: p.Phases[0].Server})
	ws, err := PlanWindows(p, 1)
	if err != nil {
		t.Fatal(err)
	}
	c.Windows = ws
	c.Limits = p.Limits
	for _, mutation := range []func([]Window){
		func(w []Window) { w[1].Offset++ },                // A phase gap must be represented by an explicit quiet interval.
		func(w []Window) { w[1].Local.FirstSequence = 0 }, // Original ranges cannot be reused across phases.
		func(w []Window) { w[0].Deadline = 0 },            // Multi-phase deadline policy is always explicit.
	} {
		bad := c
		bad.Windows = append([]Window(nil), ws...)
		mutation(bad.Windows)
		if _, err = NewUDP(context.Background(), bad); err == nil {
			t.Fatal("invalid timeline accepted")
		}
	}
}
