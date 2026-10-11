package worker

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
	"sneaky-port/internal/writer"
)

// fixtureSchedule supplies explicit arbitrary component traffic, not product defaults.
func fixtureSchedule(rate uint64) Schedule {
	d := testutil.Plan().Phases[0].Client
	d.RateNumerator = profile.Decimal(rate)
	if rate == 0 {
		d.Mode = "quiet"
	}
	return Schedule{Traffic: d, Duration: uint64(160 * time.Millisecond), Lifecycle: 2, Phase: 3, FirstSequence: 10}
}

// pair creates two real UDP adapter transports with a fixture-authenticated initial binding.
// Production SSH/start negotiation and native/live paths are outside these loopback tests.
func pair(t *testing.T, family string) (UDPConfig, UDPConfig) {
	t.Helper()
	ip, network := "127.0.0.1", "udp4"
	if family == "ipv6" {
		ip, network = "::1", "udp6"
	}
	reserve, err := net.ListenPacket(network, net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(reserve.LocalAddr().String())
	reserve.Close()
	p := testutil.Plan()
	p.Family = family
	p.ServerAddress = ip
	p.DestinationPort, _ = strconv.Atoi(port)
	factory, _ := adapter.Default().Get("udp")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	prepared, err := factory.Prepare(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Release the fixture listener on every construction/failure path.
		prepared.Close()
	})
	client, err := factory.Dial(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Retain fallback cleanup before ownership is transferred to a worker.
		client.Close()
	})
	forward := probe.Auth{Adapter: "udp", Key: [32]byte{1}, Plan: [32]byte{3}}
	reverse := probe.Auth{Adapter: "udp", Key: [32]byte{2}, Plan: forward.Plan}
	run, session := [16]byte{4}, [16]byte{5}
	bind, _ := forward.Encode(probe.Frame{Kind: probe.BindRequest, Direction: 1, Run: run, Session: session, Content: make([]byte, 32)}, probe.UDPMax)
	if _, err = client.Write(bind, time.Second); err != nil {
		t.Fatal(err)
	}
	server, err := prepared.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Release the fixture candidate if worker startup is rejected.
		server.Close()
	})
	b, err := server.Read()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = forward.Decode(b, probe.UDPMax); err != nil {
		t.Fatal(err)
	}
	epoch := time.Now().Add(-time.Second)
	start := time.Now().Add(20 * time.Millisecond)
	c := UDPConfig{Transport: client, SendAuth: forward, ReceiveAuth: reverse, Run: run, Session: session, Direction: 1, Epoch: 11, PeerEpoch: 22, EpochStart: epoch, Start: start, Local: fixtureSchedule(20), Peer: fixtureSchedule(30), Limits: p.Limits, EventCapacity: 256}
	c.Limits.MaxLag = profile.Decimal(time.Second)
	s := c
	s.Transport = server
	s.SendAuth, s.ReceiveAuth = reverse, forward
	s.Direction = 2
	s.Epoch, s.PeerEpoch = c.PeerEpoch, c.Epoch
	s.Local, s.Peer = c.Peer, c.Local
	return c, s
}

// start transfers a fixture transport and arranges bounded worker cleanup.
func start(t *testing.T, c UDPConfig) *UDP {
	t.Helper()
	u, err := NewUDP(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Join real owned workers, not merely the calling test goroutine.
		u.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := u.Wait(ctx); err != nil {
			t.Errorf("worker cleanup: %v", err)
		}
	})
	return u
}

// eventLog drains without blocking timing workers and gives race-safe snapshots.
type eventLog struct {
	mu     sync.Mutex
	events []Event
	done   chan struct{}
}

// collect owns a fixture evidence consumer with finite traffic and bounded worker lifetime.
func collect(u *UDP) *eventLog {
	l := &eventLog{done: make(chan struct{})}
	go func() { // Drain component records until joined shutdown closes the channel.
		defer close(l.done)
		for e := range u.Events() {
			l.mu.Lock()
			l.events = append(l.events, e)
			l.mu.Unlock()
		}
	}()
	return l
}

// snapshot copies records so checks never race the evidence collector.
func (l *eventLog) snapshot() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Event(nil), l.events...)
}

// until bounds asynchronous fixture conditions; it makes no empirical timing claim.
func until(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !check() {
		if time.Now().After(deadline) {
			t.Fatal("fixture condition timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

// count counts a specific evidence category and frame kind in an immutable snapshot.
func count(events []Event, kind string, frameKind byte) int {
	n := 0
	for _, e := range events {
		if e.Kind == kind && e.Frame.Kind == frameKind {
			n++
		}
	}
	return n
}

// accepted counts actual admissions, allowing honest scheduler skips on a busy test host.
func accepted(events []Event, frameKind byte) int {
	n := 0
	for _, e := range events {
		if e.Kind == "admission" && e.Frame.Kind == frameKind && e.Admission.State == writer.Accepted {
			n++
		}
	}
	return n
}

// reached totals generated/skipped planned slots, preserving skipped ranges without expansion.
func reached(events []Event) uint64 {
	n := uint64(0)
	for _, e := range events {
		if e.Kind == "schedule_skip" || e.Kind == "admission" && e.Frame.Kind == probe.Data {
			n += e.Count
		}
	}
	return n
}

// TestUDPBidir proves separately configured local schedules and reverse physical MAC keys.
func TestUDPBidir(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6"} {
		t.Run(family, func(t *testing.T) { // Apply identical component contracts to each real loopback family.
			c, s := pair(t, family)
			client, server := start(t, c), start(t, s)
			cl, sl := collect(client), collect(server)
			until(t, func() bool { // Reconcile actual admissions/receipts plus skipped slots, not assumed host timing.
				ce, se := cl.snapshot(), sl.snapshot()
				return reached(ce) == client.localCount && reached(se) == server.localCount && accepted(ce, probe.Data) > 0 && accepted(se, probe.Data) > 0 && count(ce, "arrival", probe.Receipt) == accepted(ce, probe.Data) && count(se, "arrival", probe.Receipt) == accepted(se, probe.Data)
			})
			for _, l := range []*eventLog{cl, sl} {
				for _, e := range l.snapshot() {
					if e.Kind == "write" && (e.Err != nil || e.At < e.Call || e.Call < e.Frame.Attempt || e.Frame.Kind == probe.Receipt && e.Call < e.Frame.ReceiptAttempt) {
						t.Fatalf("write boundaries: %+v", e)
					}
					if e.Kind == "attempt" && e.Frame.Kind == probe.Receipt && e.Frame.ReceiptAttempt < e.Frame.Arrival {
						t.Fatal("negative receipt processing")
					}
					if e.Kind == "invalid" {
						t.Fatalf("unexpected invalid: %v", e.Err)
					}
					if e.Kind == "admission" && e.Admission.State == writer.Accepted {
						select {
						case r := <-e.Admission.Done:
							if r.Err != nil || !r.Attempted {
								t.Fatalf("write ticket: %+v", r)
							}
						case <-time.After(time.Second):
							t.Fatal("missing ticket")
						}
					}
				}
			}
			if reached(cl.snapshot()) != client.localCount || reached(sl.snapshot()) != server.localCount {
				t.Fatal("lost schedule slots")
			}
		})
	}
}

// gated wraps real UDP I/O with a controlled local writer stall or partial-write result.
// This is an injected component condition, not an OS/network impairment experiment.
type gated struct {
	adapter.Transport
	gate            chan struct{}
	entered         chan struct{}
	closed          chan struct{}
	once, closeOnce sync.Once
	partial         bool
}

// Write holds only the fixture's writer; reads and independent scheduling keep running.
func (g *gated) Write(b []byte, budget time.Duration) (int, error) {
	g.once.Do(func() { // Publish a deterministic active-writer boundary exactly once.
		close(g.entered)
	})
	if g.partial {
		return 3, nil
	}
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case <-g.gate:
		return g.Transport.Write(b, budget)
	case <-g.closed:
		return 0, net.ErrClosed
	case <-timer.C:
		return 0, context.DeadlineExceeded
	}
}

// Close cooperatively releases the injected write and the actual socket.
func (g *gated) Close() error {
	g.closeOnce.Do(func() { // Interrupt the stalled callback without releasing unrelated sockets.
		close(g.closed)
	})
	return g.Transport.Close()
}

// gateTransport wraps a fixture transport with finite explicit synchronization channels.
func gateTransport(t adapter.Transport) *gated {
	return &gated{Transport: t, gate: make(chan struct{}), entered: make(chan struct{}), closed: make(chan struct{})}
}

// TestUDPStalledWriter proves both schedules/reads proceed and receipts share FIFO/reserves.
func TestUDPStalledWriter(t *testing.T) {
	c, s := pair(t, "ipv4")
	c.Local = fixtureSchedule(25)
	c.Local.Duration = uint64(400 * time.Millisecond)
	c.Peer = fixtureSchedule(15)
	c.Peer.Duration = c.Local.Duration
	s.Local = c.Peer
	s.Peer = c.Local
	c.Limits.QueueMessages = 3
	c.Limits.ReceiptReserve = 1
	g := gateTransport(c.Transport)
	c.Transport = g
	client, server := start(t, c), start(t, s)
	cl, sl := collect(client), collect(server)
	until(t, func() bool { // Require both finite schedules to finish while one write remains gated.
		ce, se := cl.snapshot(), sl.snapshot()
		return reached(ce) == client.localCount && reached(se) == server.localCount && count(ce, "arrival", probe.Data) == accepted(se, probe.Data) && count(ce, "admission", probe.Receipt) == accepted(se, probe.Data) && accepted(se, probe.Data) > 0
	})
	if count(cl.snapshot(), "write", probe.Data) != 0 {
		t.Fatal("gate released unexpectedly")
	}
	dataSkip, receiptFail := false, false
	for _, e := range cl.snapshot() {
		if e.Kind == "admission" && e.Admission.State == writer.Skipped {
			dataSkip = true
		}
		if e.Kind == "admission" && e.Frame.Kind == probe.Receipt && e.Admission.State == writer.Failed {
			receiptFail = true
		}
	}
	if !dataSkip || !receiptFail {
		t.Fatalf("missing saturation evidence: %+v", cl.snapshot())
	}
	before := cl.snapshot()
	admitted := []probe.Frame{}
	for _, e := range before {
		if e.Kind == "admission" && e.Admission.State == writer.Accepted {
			admitted = append(admitted, e.Frame)
		}
	}
	close(g.gate)
	until(t, func() bool { // Drain the already admitted shared FIFO after schedules have ended.
		return count(cl.snapshot(), "write", probe.Data)+count(cl.snapshot(), "write", probe.Receipt) == len(admitted)
	})
	writes := []probe.Frame{}
	for _, e := range cl.snapshot() {
		if e.Kind == "write" {
			writes = append(writes, e.Frame)
		}
	}
	for i, f := range admitted {
		if writes[i].Kind != f.Kind || writes[i].Sequence != f.Sequence {
			t.Fatal("receipt bypassed shared FIFO")
		}
	}
}

// TestUDPLagNeverCatchesUp proves an expired window accounts slots without socket traffic.
func TestUDPLagNeverCatchesUp(t *testing.T) {
	c, _ := pair(t, "ipv4")
	c.EpochStart = time.Now().Add(-2 * time.Second)
	c.Start = time.Now().Add(-time.Second)
	c.Limits.MaxLag = 0
	u := start(t, c)
	l := collect(u)
	until(t, func() bool { // Await range evidence for every expired slot.
		return reached(l.snapshot()) == u.localCount
	})
	if count(l.snapshot(), "admission", probe.Data) != 0 || count(l.snapshot(), "write", probe.Data) != 0 {
		t.Fatal("catch-up traffic")
	}
	if len(l.snapshot()) != 1 || l.snapshot()[0].Reason != "scheduler_lag" {
		t.Fatal("missing range reason")
	}
}

// TestUDPInvalidAndDuplicate validates peer identity/content and suppresses repeated receipts.
func TestUDPInvalidAndDuplicate(t *testing.T) {
	c, s := pair(t, "ipv4")
	s.Local = fixtureSchedule(0)
	s.Peer = fixtureSchedule(20)
	u := start(t, s)
	l := collect(u)
	f := probe.Frame{Kind: probe.Data, Direction: 1, Run: c.Run, Session: c.Session, Lifecycle: 2, Phase: 3, Sequence: 10, OriginEpoch: c.Epoch, Attempt: 17, Content: s.Peer.Traffic.Payload()}
	frames := []probe.Frame{f, f}
	wrong := f
	wrong.Session = [16]byte{99}
	frames = append(frames, wrong)
	wrong = f
	wrong.Content = []byte("nope")
	frames = append(frames, wrong)
	wrong = f
	wrong.Sequence = 999
	frames = append(frames, wrong)
	wrong = f
	wrong.Direction = 2
	frames = append(frames, wrong)
	wrong = f
	wrong.OriginEpoch = 99
	frames = append(frames, wrong)
	wrong = f
	wrong.Kind = probe.Receipt
	wrong.Direction = 2
	wrong.Content = nil
	wrong.PeerEpoch = c.Epoch
	frames = append(frames, wrong)
	for _, datum := range frames {
		b, err := c.SendAuth.Encode(datum, probe.UDPMax)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = c.Transport.Write(b, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := c.ReceiveAuth.Encode(f, probe.UDPMax)
	if _, err := c.Transport.Write(b, time.Second); err != nil {
		t.Fatal(err)
	}
	until(t, func() bool { // Every supplied datagram must be classified without producing new scheduled data.
		return len(l.snapshot()) >= len(frames)+1+3
	})
	if count(l.snapshot(), "arrival", probe.Data) != 1 || count(l.snapshot(), "duplicate", probe.Data) != 1 || count(l.snapshot(), "write", probe.Receipt) != 1 {
		t.Fatalf("unexpected classification: %+v", l.snapshot())
	}
	c.Transport.SetReadDeadline(time.Now().Add(time.Second))
	b, err := c.Transport.Read()
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.ReceiveAuth.Decode(b, probe.UDPMax)
	if err != nil || r.Kind != probe.Receipt || r.Attempt != 17 || r.Direction != 1 || r.PeerEpoch != s.Epoch || r.ReceiptAttempt < r.Arrival {
		t.Fatalf("receipt %+v %v", r, err)
	}
	if _, err = c.SendAuth.Decode(b, probe.UDPMax); err == nil {
		t.Fatal("wrong physical key authenticated receipt")
	}
}

// TestUDPFailureAndEventBound proves partial writes/timeouts/overflow stop and release sockets.
func TestUDPFailureAndEventBound(t *testing.T) {
	for _, cause := range []string{"partial", "timeout", "overflow", "cancel"} {
		t.Run(cause, func(t *testing.T) { // Isolate terminal conditions on separate owned loopback ports.
			c, _ := pair(t, "ipv4")
			g := gateTransport(c.Transport)
			c.Transport = g
			c.Peer = fixtureSchedule(0)
			if cause == "partial" {
				g.partial = true
			}
			if cause == "timeout" {
				c.Limits.WriteTimeout = profile.Decimal(30 * time.Millisecond)
			}
			if cause == "overflow" {
				c.EventCapacity = 1
			}
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			u, err := NewUDP(parent, c)
			if err != nil {
				t.Fatal(err)
			}
			var l *eventLog
			if cause != "overflow" {
				l = collect(u)
			}
			if cause == "cancel" {
				select {
				case <-g.entered:
				case <-time.After(time.Second):
					t.Fatal("writer not active")
				}
				until(t, func() bool { // Cancel after queued admissions too, proving they resolve without socket attempts.
					return reached(l.snapshot()) == u.localCount
				})
				cancel()
			}
			ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
			defer stop()
			err = u.Wait(ctx)
			if cause == "cancel" && err != nil || cause == "partial" && !errors.Is(err, io.ErrShortWrite) || cause == "timeout" && !errors.Is(err, context.DeadlineExceeded) || cause == "overflow" && (err == nil || !strings.Contains(err.Error(), "event capacity")) {
				t.Fatalf("%s: %v", cause, err)
			}
			if l != nil {
				<-l.done
				if cause == "partial" && count(l.snapshot(), "write", probe.Data) != 1 {
					t.Fatal("partial write retried")
				}
				if cause == "cancel" {
					attempted, unattempted := 0, 0
					for _, e := range l.snapshot() {
						if e.Kind == "admission" && e.Admission.State == writer.Accepted {
							r := <-e.Admission.Done
							if r.Attempted {
								attempted++
							} else {
								unattempted++
							}
						}
					}
					if attempted != 1 || unattempted < 1 || unattempted != accepted(l.snapshot(), probe.Data)-1 {
						t.Fatalf("canceled tickets attempted=%d unattempted=%d", attempted, unattempted)
					}
				}
			}
			if usage := u.queue.Snapshot(); usage != (writer.Usage{}) {
				t.Fatalf("retained queue: %+v", usage)
			}
			bound, err := net.ListenPacket("udp4", c.Transport.Local())
			if err != nil {
				t.Fatalf("socket not released: %v", err)
			}
			bound.Close()
			u.Close()
			if err := u.Wait(ctx); cause == "cancel" && err != nil {
				t.Fatal(err)
			}
		})
	}
}
