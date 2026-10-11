// Package writer bounds per-direction frame work independently of scheduling and receipts.
package writer

import (
	"context"
	"errors"
	"sync"
	"time"

	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// AdmissionState separates local data skips from receipt/invalid/stopped failures.
type AdmissionState string

const (
	Accepted AdmissionState = "accepted"
	Skipped  AdmissionState = "skipped_local"
	Failed   AdmissionState = "failed_local"
)

// Reason is local queue evidence, never a network-loss classification.
type Reason string

const (
	MessageCap   Reason = "queue_messages"
	ByteCap      Reason = "queue_bytes"
	InvalidFrame Reason = "invalid_frame"
	Canceled     Reason = "canceled"
	WriteFailure Reason = "write_failed"
	WriteTimeout Reason = "write_timeout"
)

// Admission supplies exactly one eventual result only when work was accepted.
type Admission struct {
	State  AdmissionState
	Reason Reason
	Done   <-chan Result
}

// Result retains callback byte evidence; even a complete write does not prove delivery.
// Bytes use the callback's declared transport boundary (which may include TCP framing).
type Result struct {
	Reason    Reason // Empty means the callback completed without a local error.
	Attempted bool   // Callback invoked; not a socket/ledger timestamp. False means waiting work was removed.
	Bytes     int
	Err       error
}

// WriteFunc serializes one admitted frame at a time, taking attempt timestamps after
// queue waiting and before encoding. It must return on context cancellation/timeout,
// closing owned I/O if needed, check cancellation before I/O, and return an error
// for incomplete/partial writes.
// Queue code supplies no attempt/arrival clock and cannot interrupt arbitrary callbacks.
type WriteFunc func(context.Context, probe.Frame) (int, error)

// Usage includes the active frame until its callback returns, so a blocked writer
// cannot release budget early. Bytes count authenticated frames, excluding outer framing.
type Usage struct {
	Messages, Bytes, DataMessages, DataBytes int
	Queued                                   int
	Writing                                  bool
}

// pending holds an immutable content snapshot and a single bounded completion slot.
type pending struct {
	frame probe.Frame
	bytes int
	done  chan Result
}

// Queue owns one FIFO worker; receipts share order and caps but data cannot consume
// the reserved messages or receipt-sized bytes. No priority bypass interrupts a frame.
type Queue struct {
	mu                          sync.Mutex
	items                       []*pending
	head, length                int
	usage                       Usage
	messageCap, byteCap         int
	dataMessageCap, dataByteCap int
	writeTimeout                time.Duration
	write                       WriteFunc
	ctx                         context.Context
	cancel                      context.CancelFunc
	wake                        chan struct{}
	done                        chan struct{}
	stopped                     bool
	reason                      Reason
}

// New checks queue-specific profile bounds and starts one owned worker, with no
// transport, scheduler or ledger connection. Receipt reserve is N * probe.Overhead;
// the existing profile feasibility check is conservative and remains unchanged.
func New(parent context.Context, limits profile.Limits, write WriteFunc) (*Queue, error) {
	if parent == nil || write == nil || limits.QueueMessages < 2 || limits.QueueMessages > 1<<20 || limits.ReceiptReserve < 1 || limits.ReceiptReserve >= limits.QueueMessages || limits.QueueBytes < 2*probe.Overhead || limits.QueueBytes > 1<<30 || limits.WriteTimeout == 0 || limits.WriteTimeout > 1<<63-1 {
		return nil, errors.New("invalid writer queue limits or callback")
	}
	reserveBytes := limits.ReceiptReserve * probe.Overhead
	if reserveBytes > int(limits.QueueBytes)-probe.Overhead {
		return nil, errors.New("queue cannot hold data plus reserved receipts")
	}
	ctx, cancel := context.WithCancel(parent)
	q := &Queue{
		items: make([]*pending, limits.QueueMessages), messageCap: limits.QueueMessages,
		byteCap: int(limits.QueueBytes), dataMessageCap: limits.QueueMessages - limits.ReceiptReserve,
		dataByteCap: int(limits.QueueBytes) - reserveBytes, writeTimeout: time.Duration(limits.WriteTimeout),
		write: write, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
	go q.run()
	return q, nil
}

// Submit never waits for the writer or space. Successful admissions copy content
// under the FIFO lock; callers retain their original frame. Identity/authentication
// validation belongs to the engine/codec. Only data and empty-content receipts queue.
func (q *Queue) Submit(frame probe.Frame) Admission {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ctx.Err() != nil && !q.stopped {
		q.stopLocked(Canceled, q.ctx.Err())
	}
	if q.stopped {
		return Admission{State: Failed, Reason: q.reason}
	}
	if frame.Kind != probe.Data && frame.Kind != probe.Receipt || frame.Kind == probe.Receipt && len(frame.Content) != 0 || len(frame.Content) > probe.TCPMax-probe.Overhead {
		return Admission{State: Failed, Reason: InvalidFrame}
	}
	bytes := probe.Overhead + len(frame.Content)
	reason := Reason("")
	if q.usage.Messages >= q.messageCap || frame.Kind == probe.Data && q.usage.DataMessages >= q.dataMessageCap {
		reason = MessageCap
	} else if bytes > q.byteCap-q.usage.Bytes || frame.Kind == probe.Data && bytes > q.dataByteCap-q.usage.DataBytes {
		reason = ByteCap
	}
	if reason != "" {
		state := Failed
		if frame.Kind == probe.Data {
			state = Skipped
		}
		return Admission{State: state, Reason: reason}
	}
	frame.Content = append([]byte(nil), frame.Content...)
	item := &pending{frame: frame, bytes: bytes, done: make(chan Result, 1)}
	q.items[(q.head+q.length)%len(q.items)] = item
	q.length++
	q.usage.Messages++
	q.usage.Bytes += bytes
	if frame.Kind == probe.Data {
		q.usage.DataMessages++
		q.usage.DataBytes += bytes
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return Admission{State: Accepted, Done: item.done}
}

// Snapshot reads finite current occupancy; it is diagnostic, not an admission reservation.
func (q *Queue) Snapshot() Usage {
	q.mu.Lock()
	defer q.mu.Unlock()
	u := q.usage
	u.Queued = q.length
	return u
}

// Close idempotently rejects new work, resolves queued work without attempts and
// cancels the active callback. It neither drains nor closes user-owned resources.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.stopLocked(Canceled, context.Canceled)
}

// Wait joins the owned worker within the caller's shutdown budget; a callback that
// violates cancellation remains visible as a wait error rather than a detached worker.
func (q *Queue) Wait(ctx context.Context) error {
	select {
	case <-q.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// stopLocked fixes the first stop reason and releases all unattempted queued frames.
// The active frame retains its budget and callback evidence until it returns.
func (q *Queue) stopLocked(reason Reason, err error) {
	if q.stopped {
		return
	}
	q.stopped, q.reason = true, reason
	q.cancel()
	for q.length > 0 {
		item := q.popLocked()
		q.finishLocked(item, Result{Reason: reason, Err: err})
	}
}

// popLocked removes the oldest waiting reference without freeing its active budget.
func (q *Queue) popLocked() *pending {
	item := q.items[q.head]
	q.items[q.head] = nil
	q.head = (q.head + 1) % len(q.items)
	q.length--
	return item
}

// finishLocked releases exact occupancy and publishes one result without blocking
// the worker on a report consumer; callers own retention of completed tickets.
func (q *Queue) finishLocked(item *pending, result Result) {
	q.usage.Messages--
	q.usage.Bytes -= item.bytes
	if item.frame.Kind == probe.Data {
		q.usage.DataMessages--
		q.usage.DataBytes -= item.bytes
	}
	item.done <- result
	close(item.done)
}

// run consumes FIFO work with one cancellation-aware callback and timeout per frame.
// Any write failure stops the queue: partial stream work must never be followed by
// another frame as if framing survived. Transport-specific evidence stays in callbacks.
func (q *Queue) run() {
	defer close(q.done)
	defer q.cancel()
	for {
		q.mu.Lock()
		if q.ctx.Err() != nil && !q.stopped {
			q.stopLocked(Canceled, q.ctx.Err())
		}
		if q.stopped {
			q.mu.Unlock()
			return
		}
		if q.length == 0 {
			q.mu.Unlock()
			select {
			case <-q.ctx.Done():
			case <-q.wake:
			}
			continue
		}
		item := q.popLocked()
		q.usage.Writing = true
		q.mu.Unlock()

		ctx, cancel := context.WithTimeout(q.ctx, q.writeTimeout)
		bytes, err := q.write(ctx, item.frame)
		if err == nil {
			err = ctx.Err()
		}
		cancel()
		q.mu.Lock()
		reason := Reason("")
		if err != nil {
			reason = WriteFailure
			if q.ctx.Err() != nil {
				reason = Canceled
			} else if errors.Is(err, context.DeadlineExceeded) {
				reason = WriteTimeout
			}
			q.stopLocked(reason, err)
		}
		q.usage.Writing = false
		q.finishLocked(item, Result{Reason: reason, Attempted: true, Bytes: bytes, Err: err})
		q.mu.Unlock()
	}
}
