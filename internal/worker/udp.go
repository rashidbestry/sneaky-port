// Package worker connects bounded schedules and queues to owned measurement I/O.
package worker

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/schedule"
	"sneaky-port/internal/writer"
)

// Schedule describes one explicit component window, not a full warmup/phase run.
// Sequence ranges include skipped slots; callers must not reuse them within the same run.
type Schedule struct {
	Traffic       profile.Direction
	Duration      uint64
	Lifecycle     byte
	Phase         uint32
	FirstSequence uint64
}

// UDPConfig transfers an already bound/authenticated UDP transport on successful
// construction. Start/EpochStart are local monotonic anchors; Peer has no shared clock.
// EventCapacity bounds in-memory records, independently of future persisted budgets.
type UDPConfig struct {
	Transport             adapter.Transport
	SendAuth, ReceiveAuth probe.Auth
	Run, Session          [16]byte
	Direction             byte
	Epoch, PeerEpoch      uint32
	EpochStart, Start     time.Time
	Local, Peer           Schedule
	Limits                profile.Limits
	EventCapacity         int
}

// Event preserves local boundaries without treating socket acceptance as delivery.
// Frame omits content (validated against the frozen pattern); skipped ranges use Count.
// Admission tickets retain completion evidence even for queued work canceled unattempted.
// Concurrent arrivals may precede write completion, so consumers must correlate identities.
type Event struct {
	Kind                       string
	Frame                      probe.Frame
	At, Scheduled, Call, Count uint64
	Admission                  writer.Admission
	Bytes                      int
	Err                        error
	Reason                     string
}

// attempt records only actual data attempts for validating echoed receipt evidence.
type attempt struct {
	started, received bool
	at                uint64
}

// UDP owns independent scheduler, reader and FIFO writer workers for one window.
// Exhausted event capacity terminates the component visibly instead of blocking timing.
// Full phase/deadline/ledger/grace/control integration belongs to the run engine.
type UDP struct {
	c                         UDPConfig
	localPayload, peerPayload []byte
	localCount, peerCount     uint64
	ctx                       context.Context
	cancel                    context.CancelFunc
	queue                     *writer.Queue
	events                    chan Event
	done, scheduled, readDone chan struct{}
	mu                        sync.Mutex
	admitMu                   sync.Mutex
	err                       error
	attempts                  []attempt
	seen                      []bool
	outstanding               int
}

// validateSchedule checks finite identity ranges and content before allocating workers.
func validateSchedule(s Schedule) (uint64, []byte, error) {
	n, err := s.Traffic.Pattern().Count(s.Duration)
	pattern, decodeErr := base64.StdEncoding.DecodeString(s.Traffic.ContentPattern)
	if err != nil || decodeErr != nil || base64.StdEncoding.EncodeToString(pattern) != s.Traffic.ContentPattern || s.Duration == 0 || s.Lifecycle < 1 || s.Lifecycle > 2 || s.FirstSequence > math.MaxUint64-n || s.Traffic.ContentBytes < 0 || s.Traffic.ContentBytes > probe.UDPMax-probe.Overhead || s.Traffic.ContentBytes > 0 && len(pattern) == 0 {
		return 0, nil, errors.New("invalid UDP component schedule/content/identity")
	}
	return n, s.Traffic.Payload(), nil
}

// NewUDP validates one explicit window and starts independent real-I/O workers.
// It performs no binding/start negotiation, marking, metrics or CLI lifecycle work.
func NewUDP(parent context.Context, c UDPConfig) (*UDP, error) {
	localCount, localPayload, err := validateSchedule(c.Local)
	if err != nil {
		return nil, err
	}
	peerCount, peerPayload, err := validateSchedule(c.Peer)
	if err != nil {
		return nil, err
	}
	startOffset := c.Start.Sub(c.EpochStart)
	if parent == nil || c.Transport == nil || c.Run == ([16]byte{}) || c.Session == ([16]byte{}) || c.SendAuth.Key == ([32]byte{}) || c.ReceiveAuth.Key == ([32]byte{}) || c.SendAuth.Adapter != "udp" || c.ReceiveAuth.Adapter != "udp" || c.SendAuth.Plan != c.ReceiveAuth.Plan || c.SendAuth.Key == c.ReceiveAuth.Key || c.Direction < 1 || c.Direction > 2 || c.Epoch == 0 || c.PeerEpoch == 0 || c.EpochStart.IsZero() || c.EpochStart.After(time.Now()) || c.Start.IsZero() || startOffset < 0 || c.Local.Duration > uint64(math.MaxInt64-int64(startOffset)) || c.EventCapacity < 1 || c.EventCapacity > 1<<20 || c.Limits.LedgerMessages < 1 || c.Limits.LedgerMessages > 1<<20 || localCount+peerCount < localCount || localCount+peerCount > uint64(c.Limits.LedgerMessages) || c.Limits.Outstanding < 1 || c.Limits.Outstanding > c.Limits.LedgerMessages || uint64(c.Limits.MaxLag) > math.MaxInt64 {
		return nil, errors.New("invalid UDP component configuration/budgets")
	}
	ctx, cancel := context.WithCancel(parent)
	u := &UDP{c: c, localPayload: localPayload, peerPayload: peerPayload, localCount: localCount, peerCount: peerCount, ctx: ctx, cancel: cancel, events: make(chan Event, c.EventCapacity), done: make(chan struct{}), scheduled: make(chan struct{}), readDone: make(chan struct{}), attempts: make([]attempt, localCount), seen: make([]bool, peerCount)}
	u.queue, err = writer.New(ctx, c.Limits, u.write)
	if err != nil {
		cancel()
		return nil, err
	}
	go u.shutdown()
	go u.receive()
	go u.schedule()
	return u, nil
}

// Events exposes finite evidence; consumers must keep up or Wait reports overflow.
func (u *UDP) Events() <-chan Event { return u.events }

// Scheduled closes after generation stops; it does not imply writes/receipts finished.
func (u *UDP) Scheduled() <-chan struct{} { return u.scheduled }

// Close requests idempotent cleanup of only the transferred component transport.
func (u *UDP) Close() { u.cancel() }

// Wait joins every owned worker within the caller's bound, retaining the first failure.
// Explicit/parent cancellation is normal cleanup, not successful full-run acceptance.
func (u *UDP) Wait(ctx context.Context) error {
	select {
	case <-u.done:
		u.mu.Lock()
		defer u.mu.Unlock()
		return u.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// now captures host-local epoch offsets; these are not on-wire or cross-host times.
func (u *UDP) now() uint64 { return uint64(time.Since(u.c.EpochStart)) }

// fail fixes the first operational error and interrupts all owned workers.
func (u *UDP) fail(err error) {
	u.mu.Lock()
	if u.err == nil {
		u.err = err
	}
	u.mu.Unlock()
	u.cancel()
}

// emit never waits for a consumer; overflow remains visible through Wait's error.
func (u *UDP) emit(e Event) bool {
	e.Frame.Content = nil
	select {
	case u.events <- e:
		return true
	default:
		u.fail(errors.New("UDP component event capacity exhausted"))
		return false
	}
}

// shutdown closes the socket to interrupt reads/writes, then joins before closing events.
// A transport violating Close's interruption contract is exposed by bounded Wait.
func (u *UDP) shutdown() {
	<-u.ctx.Done()
	u.queue.Close()
	u.c.Transport.Close()
	<-u.scheduled
	<-u.readDone
	u.queue.Wait(context.Background())
	close(u.events)
	close(u.done)
}

// base constructs the component's frozen identity without timestamping admission early.
func (u *UDP) base(s Schedule, sequence uint64) probe.Frame {
	return probe.Frame{Kind: probe.Data, Direction: u.c.Direction, Run: u.c.Run, Session: u.c.Session, Lifecycle: s.Lifecycle, Phase: s.Phase, Sequence: sequence, OriginEpoch: u.c.Epoch}
}

// schedule advances absolute slots independently of reads, receipts and blocked writers.
// A delayed wake emits one skipped range and at most the latest admissible datum.
func (u *UDP) schedule() {
	defer close(u.scheduled)
	p := u.c.Local.Traffic.Pattern()
	next := uint64(0)
	for next < u.localCount {
		offset, _ := p.Offset(next, u.c.Local.Duration)
		if !schedule.Wait(u.ctx.Done(), u.c.Start, offset) {
			return
		}
		elapsed := uint64(time.Since(u.c.Start))
		end, send, err := p.Due(next, u.c.Local.Duration, elapsed, uint64(u.c.Limits.MaxLag))
		if err != nil {
			u.fail(err)
			return
		}
		if u.ctx.Err() != nil {
			return
		}
		skippedEnd := end
		if send {
			skippedEnd--
		}
		if skippedEnd > next {
			f := u.base(u.c.Local, u.c.Local.FirstSequence+next)
			if !u.emit(Event{Kind: "schedule_skip", Frame: f, At: u.now(), Scheduled: uint64(u.c.Start.Sub(u.c.EpochStart)) + offset, Count: skippedEnd - next, Reason: "scheduler_lag"}) {
				return
			}
		}
		if send {
			offset, _ = p.Offset(end-1, u.c.Local.Duration)
			f := u.base(u.c.Local, u.c.Local.FirstSequence+end-1)
			f.Content = u.localPayload
			u.admit(f, uint64(u.c.Start.Sub(u.c.EpochStart))+offset)
		}
		next = end
	}
}

// admit preserves admission-before-attempt events and outstanding capacity without waiting
// for I/O. Outstanding entries expire only on receipts in this component; deadlines are task 9.
func (u *UDP) admit(f probe.Frame, scheduled uint64) {
	u.admitMu.Lock()
	defer u.admitMu.Unlock()
	u.mu.Lock()
	full := f.Kind == probe.Data && u.outstanding >= u.c.Limits.Outstanding
	if f.Kind == probe.Data && !full {
		u.outstanding++
	}
	u.mu.Unlock()
	a := writer.Admission{State: writer.Skipped}
	reason := "outstanding_data"
	if !full {
		a = u.queue.Submit(f)
		reason = string(a.Reason)
		if f.Kind == probe.Data && a.State != writer.Accepted {
			u.mu.Lock()
			u.outstanding--
			u.mu.Unlock()
		}
	}
	u.emit(Event{Kind: "admission", Frame: f, At: u.now(), Scheduled: scheduled, Count: 1, Admission: a, Reason: reason})
}

// write timestamps after queue residence, encodes once and records socket-call and completion
// boundaries separately. Partial datagrams, errors/timeouts terminate without retries.
func (u *UDP) write(ctx context.Context, f probe.Frame) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	u.admitMu.Lock()
	at := u.now()
	if f.Kind == probe.Data {
		f.Attempt = at
		u.mu.Lock()
		u.attempts[f.Sequence-u.c.Local.FirstSequence] = attempt{started: true, at: at}
		u.mu.Unlock()
	} else {
		f.ReceiptAttempt = at
	}
	ok := u.emit(Event{Kind: "attempt", Frame: f, At: at})
	u.admitMu.Unlock()
	if !ok {
		return 0, errors.New("UDP component event capacity exhausted")
	}
	b, err := u.c.SendAuth.Encode(f, probe.UDPMax)
	if err != nil {
		u.fail(err)
		return 0, err
	}
	stop := context.AfterFunc(ctx, func() { // Close owned I/O on queue timeout, even if socket deadlines are ineffective.
		if u.ctx.Err() == nil {
			u.fail(ctx.Err())
		}
	})
	defer stop()
	if err = ctx.Err(); err != nil {
		return 0, err
	}
	deadline, _ := ctx.Deadline()
	budget := time.Until(deadline)
	if budget <= 0 {
		u.fail(context.DeadlineExceeded)
		return 0, context.DeadlineExceeded
	}
	call := u.now()
	n, err := u.c.Transport.Write(b, budget)
	completed := u.now()
	if ctx.Err() != nil {
		err = ctx.Err()
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		err = context.DeadlineExceeded
	}
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	u.emit(Event{Kind: "write", Frame: f, At: completed, Call: call, Bytes: n, Err: err})
	if err != nil && u.ctx.Err() == nil {
		u.fail(err)
	}
	return n, err
}

// check validates frozen identities/content and actual echoed attempts without clock mapping.
// Reader-owned duplicate bits and bounded attempt records add no RTT/loss classifications.
func (u *UDP) check(f probe.Frame) (bool, error) {
	if f.Run != u.c.Run || f.Session != u.c.Session {
		return false, errors.New("foreign run/session")
	}
	s := u.c.Peer
	if f.Kind == probe.Data {
		if f.Direction != 3-u.c.Direction || f.OriginEpoch != u.c.PeerEpoch || f.Lifecycle != s.Lifecycle || f.Phase != s.Phase || f.Sequence < s.FirstSequence || f.Sequence-s.FirstSequence >= u.peerCount || !bytes.Equal(f.Content, u.peerPayload) {
			return false, errors.New("invalid peer scheduled data")
		}
		i := f.Sequence - s.FirstSequence
		duplicate := u.seen[i]
		u.seen[i] = true
		return duplicate, nil
	}
	s = u.c.Local
	if f.Kind != probe.Receipt || f.Direction != u.c.Direction || f.OriginEpoch != u.c.Epoch || f.PeerEpoch != u.c.PeerEpoch || f.Lifecycle != s.Lifecycle || f.Phase != s.Phase || f.Sequence < s.FirstSequence || f.Sequence-s.FirstSequence >= u.localCount {
		return false, errors.New("invalid receipt identity")
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	a := &u.attempts[f.Sequence-s.FirstSequence]
	if !a.started || a.at != f.Attempt {
		return false, errors.New("receipt without matching attempt")
	}
	duplicate := a.received
	if !duplicate {
		a.received = true
		u.outstanding--
	}
	return duplicate, nil
}

// receive timestamps complete reads before authentication and generates exactly one queued
// reverse receipt per eligible datum. Invalid/duplicate frames never generate fresh data.
func (u *UDP) receive() {
	defer close(u.readDone)
	for {
		b, err := u.c.Transport.Read()
		arrival := u.now()
		if err != nil {
			if u.ctx.Err() == nil {
				u.fail(err)
			}
			return
		}
		if u.ctx.Err() != nil {
			return
		}
		f, err := u.c.ReceiveAuth.Decode(b, probe.UDPMax)
		duplicate := false
		if err == nil {
			duplicate, err = u.check(f)
		}
		if err != nil {
			if !u.emit(Event{Kind: "invalid", At: arrival, Bytes: len(b), Err: err}) {
				return
			}
			continue
		}
		kind := "arrival"
		if duplicate {
			kind = "duplicate"
		}
		if !u.emit(Event{Kind: kind, Frame: f, At: arrival, Bytes: len(b)}) {
			return
		}
		if f.Kind == probe.Data && !duplicate {
			f.Kind = probe.Receipt
			f.Content = nil
			f.PeerEpoch = u.c.Epoch
			f.Arrival = arrival
			u.admit(f, 0)
		}
	}
}
