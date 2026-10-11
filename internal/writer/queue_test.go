package writer

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// limits supplies only queue inputs; unrelated engine limits are outside this component.
func limits(messages, bytes, reserve int) profile.Limits {
	return profile.Limits{QueueMessages: messages, QueueBytes: profile.Decimal(bytes), ReceiptReserve: reserve, WriteTimeout: profile.Decimal(2 * time.Second)}
}

// datum constructs arbitrary content/identity evidence without encoding an attempt early.
func datum(sequence uint64, content int) probe.Frame {
	return probe.Frame{Kind: probe.Data, Sequence: sequence, Content: make([]byte, content)}
}

// receipt constructs an empty common receipt, whose authenticated size is fixed.
func receipt(sequence uint64) probe.Frame {
	return probe.Frame{Kind: probe.Receipt, Sequence: sequence}
}

// join bounds fixture cleanup and proves the owned worker exited, not just its caller.
func join(t *testing.T, q *Queue) {
	t.Helper()
	q.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := q.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if got := q.Snapshot(); got != (Usage{}) {
		t.Fatalf("occupancy retained after join: %+v", got)
	}
}

// result waits for exactly one bounded completion and verifies its channel closes.
func result(t *testing.T, a Admission) Result {
	t.Helper()
	if a.State != Accepted || a.Done == nil {
		t.Fatalf("not admitted: %+v", a)
	}
	select {
	case r, ok := <-a.Done:
		if !ok {
			t.Fatal("missing completion")
		}
		if _, ok := <-a.Done; ok {
			t.Fatal("multiple completions")
		}
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("completion did not arrive")
		return Result{}
	}
}

// started waits for a callback boundary, avoiding timing guesses about queue occupancy.
func started(t *testing.T, ch <-chan probe.Frame) probe.Frame {
	t.Helper()
	select {
	case f := <-ch:
		return f
	case <-time.After(2 * time.Second):
		t.Fatal("writer did not start")
		return probe.Frame{}
	}
}

// blocked starts a cooperative writer held until cancellation; no network work occurs.
func blocked(t *testing.T, l profile.Limits) (*Queue, <-chan probe.Frame) {
	t.Helper()
	start := make(chan probe.Frame, 1)
	q, err := New(context.Background(), l, func(ctx context.Context, f probe.Frame) (int, error) { // Expose the active frame, then honor queue cancellation without polling.
		start <- f
		<-ctx.Done()
		return 0, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { join(t, q) }) // Every fixture must release and join its worker.
	return q, start
}

// wantRejected verifies typed local outcomes and that rejected work creates no ticket.
func wantRejected(t *testing.T, a Admission, state AdmissionState, reason Reason) {
	t.Helper()
	if a.State != state || a.Reason != reason || a.Done != nil {
		t.Fatalf("admission %+v, want %s/%s without ticket", a, state, reason)
	}
}

// TestMessageReserveAndTotalCap proves active+waiting data cannot occupy receipt slots.
func TestMessageReserveAndTotalCap(t *testing.T) {
	q, start := blocked(t, limits(4, 10000, 2))
	active := q.Submit(datum(1, 0))
	started(t, start)
	queued := q.Submit(datum(2, 0))
	wantRejected(t, q.Submit(datum(3, 0)), Skipped, MessageCap)
	r1, r2 := q.Submit(receipt(4)), q.Submit(receipt(5))
	if r1.State != Accepted || r2.State != Accepted {
		t.Fatal("data consumed receipt message reserve")
	}
	wantRejected(t, q.Submit(receipt(6)), Failed, MessageCap)
	wantRejected(t, q.Submit(datum(7, 0)), Skipped, MessageCap)
	if got := q.Snapshot(); got != (Usage{Messages: 4, Bytes: 496, DataMessages: 2, DataBytes: 248, Queued: 3, Writing: true}) {
		t.Fatalf("wrong occupied caps: %+v", got)
	}
	q.Close()
	if got := result(t, active); !got.Attempted || got.Reason != Canceled {
		t.Fatalf("active cancellation: %+v", got)
	}
	for _, a := range []Admission{queued, r1, r2} {
		if got := result(t, a); got.Attempted || got.Bytes != 0 || got.Reason != Canceled {
			t.Fatalf("queued cancellation fabricated attempt: %+v", got)
		}
	}
	wantRejected(t, q.Submit(receipt(8)), Failed, Canceled)
}

// TestByteReserveAndTotalCap exhausts data bytes before messages, then total bytes
// with receipts. Failed admission never changes usage or consumes reserved capacity.
func TestByteReserveAndTotalCap(t *testing.T) {
	q, start := blocked(t, limits(10, 900, 2))
	q.Submit(datum(1, 200)) // 324 active authenticated bytes.
	started(t, start)
	q.Submit(datum(2, 204)) // 328 waiting bytes; 652 is the exact data ceiling.
	before := q.Snapshot()
	wantRejected(t, q.Submit(datum(3, 0)), Skipped, ByteCap)
	if q.Snapshot() != before {
		t.Fatal("failed admission changed byte accounting")
	}
	if q.Submit(receipt(4)).State != Accepted || q.Submit(receipt(5)).State != Accepted {
		t.Fatal("data consumed receipt byte reserve")
	}
	wantRejected(t, q.Submit(receipt(6)), Failed, ByteCap)
	if got := q.Snapshot(); got.Bytes != 900 || got.Messages != 4 || got.DataBytes != 652 {
		t.Fatalf("wrong total byte accounting: %+v", got)
	}
}

// TestReceiptTrafficCanUseSharedCapacity proves reserve is a data exclusion, not
// a receipt-only cap: receipts may fill all available messages/bytes in FIFO order.
func TestReceiptTrafficCanUseSharedCapacity(t *testing.T) {
	q, start := blocked(t, limits(4, 496, 1))
	q.Submit(receipt(1))
	started(t, start)
	for i := uint64(2); i <= 4; i++ {
		if q.Submit(receipt(i)).State != Accepted {
			t.Fatal("receipt incorrectly capped by reserve")
		}
	}
	wantRejected(t, q.Submit(receipt(5)), Failed, MessageCap)
	wantRejected(t, q.Submit(datum(6, 0)), Skipped, MessageCap)
}

// TestFIFOOwnershipAndBudgetRelease checks receipts never bypass admitted data,
// content snapshots survive caller edits, and active budgets release exactly once.
func TestFIFOOwnershipAndBudgetRelease(t *testing.T) {
	start := make(chan probe.Frame, 1)
	release := make(chan struct{})
	q, err := New(context.Background(), limits(4, 1000, 1), func(ctx context.Context, f probe.Frame) (int, error) { // Serialize each frame behind a fixture gate; cancellation unblocks the gate.
		start <- f
		select {
		case <-release:
			return probe.Overhead + len(f.Content), nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer join(t, q)
	first := q.Submit(datum(1, 1))
	started(t, start)
	frame := datum(2, 2)
	frame.Content[0] = 7
	second := q.Submit(frame)
	frame.Content[0] = 9
	third := q.Submit(receipt(3))
	if got := q.Snapshot(); got.Bytes != 375 || got.DataBytes != 251 || got.Queued != 2 {
		t.Fatalf("wrong initial usage: %+v", got)
	}
	release <- struct{}{}
	if got := result(t, first); got.Err != nil || got.Bytes != 125 || !got.Attempted {
		t.Fatalf("first completion: %+v", got)
	}
	if got := started(t, start); got.Sequence != 2 || got.Content[0] != 7 {
		t.Fatalf("FIFO/content snapshot changed: %+v", got)
	}
	if got := q.Snapshot(); got.Messages != 2 || got.Bytes != 250 || got.DataBytes != 126 {
		t.Fatalf("first budget not released: %+v", got)
	}
	release <- struct{}{}
	result(t, second)
	if got := started(t, start); got.Sequence != 3 || got.Kind != probe.Receipt {
		t.Fatalf("receipt reordered: %+v", got)
	}
	if got := q.Snapshot(); got.Messages != 1 || got.Bytes != 124 || got.DataMessages != 0 || got.DataBytes != 0 {
		t.Fatalf("data budget retained: %+v", got)
	}
	release <- struct{}{}
	result(t, third)
	if got := q.Snapshot(); got != (Usage{}) {
		t.Fatalf("budget retained after success: %+v", got)
	}
	// Reuse capacity after ring wrap rather than growing storage with admission history.
	fourth := q.Submit(datum(4, 1))
	started(t, start)
	release <- struct{}{}
	result(t, fourth)
	fifth := q.Submit(receipt(5))
	started(t, start)
	release <- struct{}{}
	result(t, fifth)
}

// TestPartialWriteStopsFIFO retains actual callback bytes and prevents another
// frame from being attempted after a potentially broken stream framing boundary.
func TestPartialWriteStopsFIFO(t *testing.T) {
	start := make(chan probe.Frame, 1)
	release := make(chan struct{})
	q, err := New(context.Background(), limits(4, 1000, 1), func(ctx context.Context, f probe.Frame) (int, error) { // Return explicit partial evidence after the second frame has queued.
		start <- f
		<-release
		return 17, io.ErrUnexpectedEOF
	})
	if err != nil {
		t.Fatal(err)
	}
	defer join(t, q)
	first := q.Submit(datum(1, 1))
	started(t, start)
	second := q.Submit(receipt(2))
	close(release)
	if got := result(t, first); got.Reason != WriteFailure || !got.Attempted || got.Bytes != 17 || !errors.Is(got.Err, io.ErrUnexpectedEOF) {
		t.Fatalf("partial evidence lost: %+v", got)
	}
	if got := result(t, second); got.Reason != WriteFailure || got.Attempted || got.Bytes != 0 {
		t.Fatalf("unsafe following frame: %+v", got)
	}
	wantRejected(t, q.Submit(datum(3, 0)), Failed, WriteFailure)
}

// TestWriteTimeoutStopsQueue uses the context deadline, not an elapsed-time band,
// to prove a blocked callback ends and later admissions fail explicitly.
func TestWriteTimeoutStopsQueue(t *testing.T) {
	l := limits(4, 1000, 1)
	l.WriteTimeout = profile.Decimal(50 * time.Millisecond)
	q, start := blocked(t, l)
	first := q.Submit(datum(1, 0))
	started(t, start)
	if got := result(t, first); got.Reason != WriteTimeout || !got.Attempted || !errors.Is(got.Err, context.DeadlineExceeded) {
		t.Fatalf("timeout evidence: %+v", got)
	}
	wantRejected(t, q.Submit(receipt(2)), Failed, WriteTimeout)
}

// TestParentCancellation covers an idle worker and an active cooperative callback;
// no readiness/scheduler/control worker is needed to release either queue.
func TestParentCancellation(t *testing.T) {
	for _, active := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		start := make(chan probe.Frame, 1)
		q, err := New(ctx, limits(4, 1000, 1), func(ctx context.Context, f probe.Frame) (int, error) { // Observe active work and retain bytes on cancellation.
			start <- f
			<-ctx.Done()
			return 5, ctx.Err()
		})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		var ticket Admission
		if active {
			ticket = q.Submit(datum(1, 0))
			started(t, start)
		}
		cancel()
		wantRejected(t, q.Submit(receipt(2)), Failed, Canceled)
		join(t, q)
		if active {
			if got := result(t, ticket); got.Reason != Canceled || got.Bytes != 5 || !got.Attempted {
				t.Fatalf("cancellation lost active evidence: %+v", got)
			}
		}
	}
}

// TestConcurrentAdmissionAndClose exercises lock/accounting races while a writer
// is stalled. Result consumers are deliberately absent, so they cannot block cleanup.
func TestConcurrentAdmissionAndClose(t *testing.T) {
	q, start := blocked(t, limits(8, 1500, 2))
	q.Submit(datum(0, 10))
	started(t, start)
	var producers sync.WaitGroup
	var ready sync.WaitGroup
	ready.Add(32)
	resume := make(chan struct{})
	tickets := make(chan Admission, 8)
	for i := 0; i < 32; i++ {
		producers.Go(func() { // Race finite data/receipt admissions and snapshots with repeated cancellation.
			for j := 0; j < 30; j++ {
				for _, frame := range []probe.Frame{datum(uint64(j+1), 10), receipt(uint64(j + 1))} {
					if a := q.Submit(frame); a.State == Accepted {
						tickets <- a
					}
				}
				u := q.Snapshot()
				if u.Messages > 8 || u.Bytes > 1500 || u.DataMessages > 6 || u.DataBytes > 1252 || u.Messages < 0 || u.Bytes < 0 {
					t.Errorf("cap violation: %+v", u)
				}
				if j == 9 {
					ready.Done()
					<-resume
				}
			}
		})
	}
	ready.Wait()
	close(resume)
	q.Close()
	q.Close()
	producers.Wait()
	join(t, q)
	close(tickets)
	for a := range tickets {
		if got := result(t, a); got.Attempted || got.Reason != Canceled {
			t.Fatalf("concurrent queued completion: %+v", got)
		}
	}
}

// TestInvalidLimitsAndFrames rejects unsafe capacities before allocating a worker
// and malformed queue classes before creating a ticket or changing occupancy.
func TestInvalidLimitsAndFrames(t *testing.T) {
	write := func(context.Context, probe.Frame) (int, error) { return 124, nil } // This writer is never called by rejected inputs.
	for _, l := range []profile.Limits{limits(1, 1000, 1), limits(4, 247, 1), limits(4, 1000, 0), limits(4, 1000, 4), limits(4, 371, 2), limits(1<<20+1, 1000, 1), limits(4, 1<<30+1, 1), {QueueMessages: 4, QueueBytes: 1000, ReceiptReserve: 1}, {QueueMessages: 4, QueueBytes: 1000, ReceiptReserve: 1, WriteTimeout: 1 << 63}} {
		if q, err := New(context.Background(), l, write); err == nil {
			join(t, q)
			t.Fatalf("accepted invalid limits: %+v", l)
		}
	}
	if _, err := New(nil, limits(4, 1000, 1), write); err == nil {
		t.Fatal("accepted nil context")
	}
	if _, err := New(context.Background(), limits(4, 1000, 1), nil); err == nil {
		t.Fatal("accepted nil callback")
	}
	q, start := blocked(t, limits(4, 1000, 1))
	q.Submit(datum(1, 0))
	started(t, start)
	before := q.Snapshot()
	for _, f := range []probe.Frame{{Kind: probe.BindRequest}, {Kind: probe.Receipt, Content: []byte{1}}, datum(2, probe.TCPMax)} {
		wantRejected(t, q.Submit(f), Failed, InvalidFrame)
	}
	if q.Snapshot() != before {
		t.Fatal("invalid work changed occupancy")
	}
}

// TestWaitBudgetExposesCallbackContractViolation demonstrates a shutdown deadline
// cannot forcibly interrupt arbitrary code; the fixture releases and joins it afterward.
func TestWaitBudgetExposesCallbackContractViolation(t *testing.T) {
	start := make(chan probe.Frame, 1)
	release := make(chan struct{})
	q, err := New(context.Background(), limits(4, 1000, 1), func(_ context.Context, f probe.Frame) (int, error) { // Intentionally ignore cancellation to check bounded waiting, then exit on fixture release.
		start <- f
		<-release
		return 124, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer join(t, q)
	defer close(release)
	ticket := q.Submit(datum(1, 0))
	started(t, start)
	q.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("callback violation was hidden: %v", err)
	}
	if got := q.Snapshot(); !got.Writing || got.Messages != 1 || got.Bytes != 124 {
		t.Fatalf("active budget released before callback returned: %+v", got)
	}
	if ticket.State != Accepted {
		t.Fatal("fixture was not admitted")
	}
}
