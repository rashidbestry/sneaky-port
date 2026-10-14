package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/elapsed"
	"sneaky-port/internal/platform"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/worker"
)

// retainedTransport interrupts local I/O without sending an early TCP EOF to the peer.
// The control owner must close the underlying transport after stop or control loss.
type retainedTransport struct{ adapter.Transport }

// Close interrupts only this endpoint's I/O; the coordinator retains final socket ownership.
func (t retainedTransport) Close() error {
	return t.Transport.(interface{ Interrupt() error }).Interrupt()
}

// Finite owns a local run, bounded drain, joined accounting and optional evidence finalization.
type Finite struct {
	persistence Persistence
	endpoint    *Endpoint
	cancel      context.CancelFunc
	done        chan struct{}
	result      Result
	err         error
	end         uint64
	clock       *elapsed.Epoch
	interrupted atomic.Bool
}

// phaseMarker runs only on the FIFO writer, applying the current physical phase to data and receipts.
// Quiet phases need no socket update until an outgoing frame; grace retains the last phase.
// A frame spanning a phase boundary cannot be remarked mid-write; API evidence is not packet capture.
func phaseMarker(t adapter.Transport, windows []worker.Window, start uint64) func(context.Context, uint64) (*platform.Marking, byte, uint32, error) {
	previous := -1
	return func(ctx context.Context, at uint64) (*platform.Marking, byte, uint32, error) { // Serialize marking before timestamp/encoding/write on this physical direction.
		index := 0
		for i := 1; i < len(windows); i++ {
			if at >= start+windows[i].Offset {
				index = i
			}
		}
		w := windows[index]
		if index == previous {
			return nil, w.Local.Lifecycle, w.Local.Phase, ctx.Err()
		}
		if err := ctx.Err(); err != nil {
			return nil, w.Local.Lifecycle, w.Local.Phase, err
		}
		m, err := t.Mark(w.Local.Traffic.DSCP)
		if err == nil && (m.Requested != w.Local.Traffic.DSCP || m.Observed != nil || m.Requested != 0 && (m.Applied == nil || *m.Applied != m.Requested)) {
			err = errors.New("physical phase marking not applied or invalid API evidence")
		}
		if err == nil {
			previous = index
		}
		return &m, w.Local.Lifecycle, w.Local.Phase, err
	}
}

// NewFinite freezes the plan, installs phase marking and owns generation/grace/cancellation.
// Clock is the endpoint-local source shared with authenticated calibration; StartNS is exact.
// retain is for a control-owned server socket whose earlier cutoff must not truncate the peer.
func NewFinite(parent context.Context, cfg worker.Config, p profile.Plan, retain bool) (*Finite, error) {
	return NewFiniteWithEvidence(parent, cfg, p, retain, nil)
}

// NewFiniteWithEvidence connects a pre-created secret-free store before workers can send data.
// The caller owns finalizing the store if construction fails; successful runs finalize after join.
func NewFiniteWithEvidence(parent context.Context, cfg worker.Config, p profile.Plan, retain bool, persistence Persistence) (*Finite, error) {
	if parent == nil || cfg.Transport == nil {
		return nil, errors.New("invalid finite lifecycle configuration")
	}
	if err := cfg.NormalizeClock(); err != nil {
		return nil, err
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	ws, err := worker.PlanWindows(p, cfg.Direction)
	if err != nil {
		return nil, err
	}
	if retain {
		if _, ok := cfg.Transport.(interface{ Interrupt() error }); !ok {
			return nil, errors.New("adapter cannot retain transport across endpoint cutoff")
		}
		cfg.Transport = retainedTransport{cfg.Transport}
	}
	cfg.BeforeWrite = phaseMarker(cfg.Transport, ws, cfg.StartNS)
	cfg.EventCapacity = p.Limits.LedgerMessages
	total := uint64(p.Warmup) + p.Duration() + uint64(p.Grace)
	if total > uint64(1<<63-1)-cfg.StartNS {
		return nil, errors.New("finite elapsed timeline overflow")
	}
	cfg.ObservationNS = cfg.StartNS + total
	ctx, cancel := context.WithCancel(parent)
	// The I/O context survives lifecycle cancellation only for the explicit bounded drain.
	ioCtx := context.WithoutCancel(parent)
	endpoint, err := newEndpoint(ioCtx, cfg, p, p.Adapter, worker.NewRegistered, persistence)
	if err != nil {
		cancel()
		return nil, err
	}
	f := &Finite{endpoint: endpoint, cancel: cancel, done: make(chan struct{}), end: cfg.ObservationNS, clock: cfg.Clock, persistence: persistence}
	go f.finish(ctx, cfg.ObservationNS, p)
	return f, nil
}

// finish bounds observation by the planned grace end, or cancellation plus min(grace, shutdown).
// Worker failures are already joined; interrupted results never claim complete receiver evidence.
func (f *Finite) finish(ctx context.Context, end uint64, p profile.Plan) {
	defer close(f.done)
	defer f.cancel()
	var cause error
observe:
	for {
		remaining, err := worker.Remaining(f.clock, end)
		if err != nil {
			cause = err
			f.interrupted.Store(true)
			f.endpoint.Close()
			break
		}
		if remaining == 0 {
			f.endpoint.Close()
			break
		}
		timer := time.NewTimer(remaining)
		select {
		case <-f.endpoint.done:
			timer.Stop()
			break observe
		case <-f.clock.Failed():
			timer.Stop()
			cause = f.clock.Err()
			f.interrupted.Store(true)
			f.endpoint.Close()
			break observe
		case <-timer.C: // Recheck the measurement source; timer expiry alone never establishes cutoff.
		case <-ctx.Done():
			timer.Stop()
			now, clockErr := f.clock.Now()
			if clockErr != nil {
				cause = clockErr
				f.interrupted.Store(true)
			}
			if now < end || f.interrupted.Load() {
				cause = errors.Join(cause, ctx.Err())
				f.interrupted.Store(true)
				f.endpoint.StopGeneration()
				drain := min(uint64(p.Grace), uint64(p.Limits.ShutdownTimeout), end-min(end, now))
				f.drain(now + drain)
			}
			f.endpoint.Close()
			break observe
		}
	}
	join, cancel := context.WithTimeout(context.Background(), time.Duration(p.Limits.ShutdownTimeout))
	defer cancel()
	f.result, f.err = f.endpoint.Wait(join)
	if cause != nil || f.interrupted.Load() {
		f.result.Complete, f.result.Receiver.Complete = false, false
		if f.err == nil {
			if cause == nil {
				cause = context.Canceled
			}
			f.err = cause
		}
	}
	if f.persistence != nil {
		if err := f.persistence.Finalize(f.result); err != nil {
			f.result.Complete, f.result.Receiver.Complete = false, false
			f.err = errors.Join(f.err, err)
		}
	}
}

// drain waits only within the selected epoch and stops early on joined endpoint failure.
func (f *Finite) drain(end uint64) {
	for {
		remaining, err := worker.Remaining(f.clock, end)
		if err != nil || remaining == 0 {
			return
		}
		t := time.NewTimer(remaining)
		select {
		case <-f.endpoint.done:
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// Cancel stops new generation immediately, allowing only the configured bounded drain.
func (f *Finite) Cancel() {
	now, err := f.clock.Now()
	if err != nil || now < f.end {
		f.interrupted.Store(true)
		f.endpoint.StopGeneration()
		f.cancel()
	} else {
		// A stop processed after the half-open cutoff finalizes elapsed observation even if the timer woke late.
		f.endpoint.Close()
	}
}

// Done closes after finite observation and bounded worker join, independently of control heartbeats.
func (f *Finite) Done() <-chan struct{} { return f.done }

// Wait returns immutable local evidence; caller timeouts do not silently abandon run ownership.
func (f *Finite) Wait(ctx context.Context) (Result, error) {
	select {
	case <-f.done:
		return f.result, f.err
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}
