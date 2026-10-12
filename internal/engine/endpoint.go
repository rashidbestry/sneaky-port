// Package engine owns finite in-memory phase/accounting integration over worker events.
// Finite lifecycle adds phase marking, bounded observation and optional local persistence.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/platform"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/worker"
	"sneaky-port/internal/writer"
)

// Record is a bounded, secret-free observation reused as persisted event details.
// All quantities that could exceed browser integer precision use canonical decimal strings.
type Record struct {
	Kind                                                          string
	Identity                                                      measure.Identity
	FrameKind, Direction                                          byte
	Epoch, PeerEpoch                                              uint32
	At, Scheduled, Call, Count, Attempt, PeerArrival, PeerAttempt profile.Decimal
	Bytes                                                         profile.Decimal
	Admission, Reason, Error                                      string
	CallbackInvoked                                               bool
	Marking                                                       *platform.Marking
	MarkLifecycle                                                 byte
	MarkPhase                                                     uint32
}

// PhaseMarking retains the local API boundary and physical phase independently of original message attribution.
type PhaseMarking struct {
	At        profile.Decimal  `json:"time_ns"`
	Lifecycle byte             `json:"lifecycle"`
	Phase     uint32           `json:"phase"`
	Marking   platform.Marking `json:"marking"`
	Error     string           `json:"error,omitempty"`
}

// Result retains local records and cutoffs; Complete applies only to this endpoint's evidence.
// Receiver contains observed peer data, independently of this endpoint's origin receipts.
type Result struct {
	PlanHash                                        [32]byte
	Run, Session                                    [16]byte
	Direction                                       byte
	Epoch, PeerEpoch                                uint32
	GenerationCutoff, ObservationCutoff, EventBytes profile.Decimal
	Complete                                        bool
	Entries                                         []measure.Entry
	Records                                         []json.RawMessage
	Receiver                                        measure.ReceiverEvidence
	Summary, Warmup                                 measure.Summary
	Phases                                          []measure.Summary
	Analysis                                        measure.Analysis
	Markings                                        []PhaseMarking
}

// Persistence accepts exact local records independently of socket timing and UI updates.
// Append must be bounded/nonblocking; Failed closes on asynchronous storage failure.
// Finalize flushes accepted records and publishes cutoffs, or leaves recognizable incomplete files.
type Persistence interface {
	Append(json.RawMessage) error
	Failed() <-chan struct{}
	Finalize(Result) error
}

// ticket keeps one bounded admission result until joined shutdown guarantees availability.
type ticket struct {
	frame probe.Frame
	done  <-chan writer.Result
}

// Collector has one event-loop owner; concurrent mutation is intentionally unsupported.
type Collector struct {
	persistence                 Persistence
	plan                        profile.Plan
	windows                     []worker.Window
	localEnds, peerEnds         []uint64
	direction                   byte
	start, plannedEnd, finalEnd uint64
	ledger                      *measure.Ledger
	receiver                    map[measure.Identity]uint64
	tickets                     []ticket
	records                     []json.RawMessage
	markings                    []PhaseMarking
	bytes                       uint64
	generation, observation     uint64
	err                         error
}

// NewCollector freezes the plan/windows and checks epoch-relative overflow before I/O ownership.
// An optional startup registry must match live execution; absent registrations reject offline replay.
func NewCollector(p profile.Plan, direction byte, start uint64, registries ...*adapter.Registry) (*Collector, error) {
	r := adapter.Default()
	if len(registries) > 0 && registries[0] != nil {
		r = registries[0]
	}
	err := r.Validate(p)
	if err != nil {
		return nil, err
	}
	ws, err := worker.PlanWindows(p, direction)
	if err != nil {
		return nil, err
	}
	if start > math.MaxInt64 || uint64(p.Warmup)+p.Duration()+uint64(p.Grace) > math.MaxInt64-start {
		return nil, errors.New("accounting epoch overflow")
	}
	for _, w := range ws {
		if w.Offset+w.Local.Duration+w.Deadline > math.MaxInt64-start {
			return nil, errors.New("deadline epoch overflow")
		}
	}
	// JSON cloning preserves normalized values and prevents mutable maps/slices escaping the frozen plan.
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var frozen profile.Plan
	if err = json.Unmarshal(b, &frozen); err != nil {
		return nil, err
	}
	l, err := measure.NewLedger(p.Limits.LedgerMessages)
	if err != nil {
		return nil, err
	}
	end := start + uint64(p.Warmup) + p.Duration()
	c := &Collector{plan: frozen, windows: ws, direction: direction, start: start, plannedEnd: end, finalEnd: end + uint64(p.Grace), ledger: l, receiver: map[measure.Identity]uint64{}}
	for _, w := range ws {
		n, _ := w.Local.Traffic.Pattern().Count(w.Local.Duration)
		m, _ := w.Peer.Traffic.Pattern().Count(w.Peer.Duration)
		c.localEnds = append(c.localEnds, w.Local.FirstSequence+n)
		c.peerEnds = append(c.peerEnds, w.Peer.FirstSequence+m)
	}
	return c, nil
}

// identity extracts the original lifecycle/phase/sequence, never the current receiving phase.
func identity(f probe.Frame) measure.Identity {
	return measure.Identity{Lifecycle: f.Lifecycle, Phase: f.Phase, Sequence: profile.Decimal(f.Sequence)}
}

// record converts an event to exact public quantities, excluding keys, content and channels.
func record(e worker.Event) Record {
	r := Record{Kind: e.Kind, Identity: identity(e.Frame), FrameKind: e.Frame.Kind, Direction: e.Frame.Direction, Epoch: e.Frame.OriginEpoch, PeerEpoch: e.Frame.PeerEpoch, At: profile.Decimal(e.At), Scheduled: profile.Decimal(e.Scheduled), Call: profile.Decimal(e.Call), Count: profile.Decimal(e.Count), Attempt: profile.Decimal(e.Frame.Attempt), PeerArrival: profile.Decimal(e.Frame.Arrival), PeerAttempt: profile.Decimal(e.Frame.ReceiptAttempt), Admission: string(e.Admission.State), Reason: e.Reason}
	r.Marking, r.MarkLifecycle, r.MarkPhase = e.Marking, e.MarkLifecycle, e.MarkPhase
	if e.Bytes >= 0 {
		r.Bytes = profile.Decimal(e.Bytes)
	}
	if e.Err != nil {
		r.Error = e.Err.Error()
	}
	return r
}

// keep charges the exact internal JSONL byte size, terminating on exhaustion instead of dropping silently.
// Durable envelope/file bytes are independently charged by the optional persistence owner.
func (c *Collector) keep(r Record) error {
	if len(r.Reason) > 4096 || len(r.Error) > 4096 {
		return errors.New("oversized evidence detail")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	n := uint64(len(b) + 1)
	if n > uint64(c.plan.Limits.EventBytes)-c.bytes {
		return errors.New("run event byte budget exhausted")
	}
	if c.persistence != nil {
		if err = c.persistence.Append(b); err != nil {
			return err
		}
	}
	c.records = append(c.records, b)
	c.bytes += n
	return nil
}

// window finds an original identity and its independent rational schedule within frozen phases.
func (c *Collector) window(f probe.Frame, local bool) (*worker.Window, uint64, error) {
	ends := c.peerEnds
	if local {
		ends = c.localEnds
	}
	i := sort.Search(len(ends), func(i int) bool { // Find the original window without rescanning every phase pattern per event.
		return ends[i] > f.Sequence
	})
	if i < len(c.windows) {
		w := &c.windows[i]
		s := w.Peer
		if local {
			s = w.Local
		}
		if f.Lifecycle == s.Lifecycle && f.Phase == s.Phase && f.Sequence >= s.FirstSequence {
			return w, f.Sequence - s.FirstSequence, nil
		}
	}
	return nil, 0, errors.New("event outside frozen identities")
}

// Apply correlates unordered I/O boundaries after admission, while schedules stay in origin order.
// Invalid/duplicate receipt/receipt-write events remain evidence without adding data or RTT samples.
func (c *Collector) Apply(e worker.Event) error {
	if err := c.keep(record(e)); err != nil {
		return err
	}
	return c.apply(e, true)
}

// apply shares accounting transitions with offline replay; only live events own completion channels.
func (c *Collector) apply(e worker.Event, tickets bool) error {
	if e.Kind == "marking" && e.Marking != nil {
		m := PhaseMarking{At: profile.Decimal(e.At), Lifecycle: e.MarkLifecycle, Phase: e.MarkPhase, Marking: *e.Marking}
		if e.Err != nil {
			m.Error = e.Err.Error()
		}
		c.markings = append(c.markings, m)
		return nil
	}
	if e.Kind == "generation_end" {
		c.generation = e.At
		return nil
	}
	if e.Kind == "observation_end" {
		c.observation = e.At
		return nil
	}
	if e.Kind == "invalid" {
		return nil
	}
	if e.Kind == "schedule_skip" || e.Kind == "admission" && e.Frame.Kind == probe.Data {
		w, index, err := c.window(e.Frame, true)
		if err != nil {
			return err
		}
		if e.Count == 0 {
			return errors.New("empty reached-slot range")
		}
		n, _ := w.Local.Traffic.Pattern().Count(w.Local.Duration)
		if e.Count > n-index || e.Kind == "admission" && e.Count != 1 {
			return errors.New("invalid reached-slot range")
		}
		for k := uint64(0); k < e.Count; k++ {
			offset, _ := w.Local.Traffic.Pattern().Offset(index+k, w.Local.Duration)
			at := c.start + w.Offset + offset
			if k == 0 && at != e.Scheduled {
				return errors.New("event schedule mismatch")
			}
			f := e.Frame
			f.Sequence += k
			id := identity(f)
			if err = c.ledger.Schedule(id, at, at+w.Deadline); err != nil {
				return err
			}
			if e.Kind == "schedule_skip" {
				err = c.ledger.Skip(id, e.Reason)
			} else {
				err = c.ledger.Admit(id, e.At, string(e.Admission.State), e.Reason)
			}
			if err != nil {
				return err
			}
		}
	}
	if tickets && e.Kind == "admission" && e.Admission.State == writer.Accepted {
		if e.Admission.Done == nil || len(c.tickets) >= c.plan.Limits.LedgerMessages {
			return errors.New("invalid ticket/budget")
		}
		c.tickets = append(c.tickets, ticket{e.Frame, e.Admission.Done})
	}
	id := identity(e.Frame)
	if e.Frame.Kind == probe.Data {
		switch e.Kind {
		case "unattempted":
			return c.ledger.Unattempted(id, e.At, e.Reason)
		case "attempt":
			return c.ledger.Attempt(id, e.At)
		case "write":
			if e.Bytes < 0 || e.Call < e.Frame.Attempt || e.At < e.Call {
				return errors.New("invalid write boundaries")
			}
			return c.ledger.Written(id, e.At, uint64(e.Bytes), e.Err == nil)
		case "arrival":
			if _, _, err := c.window(e.Frame, false); err != nil {
				return err
			}
			if _, ok := c.receiver[id]; ok {
				return errors.New("repeated unique receiver arrival")
			}
			if len(c.receiver) >= c.plan.Limits.LedgerMessages {
				return errors.New("receiver evidence budget exhausted")
			}
			c.receiver[id] = e.At
		}
	}
	if e.Frame.Kind == probe.Receipt && (e.Kind == "arrival" || e.Kind == "duplicate") {
		if err := c.ledger.ReceiveReceipt(id, e.Frame.Attempt, e.At); err != nil {
			return err
		}
		if e.Kind == "arrival" {
			return c.ledger.ReceiptProcessing(id, e.Frame.Arrival, e.Frame.ReceiptAttempt)
		}
	}
	return nil
}

// Finish reconciles every accepted ticket after worker join; it never blocks on unfinished I/O.
// Unavailable tickets/evidence keep the endpoint partial, and full-run percentages are not produced here.
func (c *Collector) Finish(workerErr error, observed uint64) (Result, error) {
	if c.observation == 0 {
		c.observation = observed
	}
	err := c.err
	if err == nil {
		err = workerErr
	}
	for _, t := range c.tickets {
		select {
		case result := <-t.done:
			r := Record{Kind: "ticket_observed", Identity: identity(t.frame), FrameKind: t.frame.Kind, Direction: t.frame.Direction, At: profile.Decimal(observed), Reason: string(result.Reason), CallbackInvoked: result.Attempted}
			if result.Bytes >= 0 {
				r.Bytes = profile.Decimal(result.Bytes)
			}
			if result.Err != nil {
				r.Error = result.Err.Error()
			}
			kept := c.keep(r)
			if kept != nil && err == nil {
				err = kept
			}
			// An unsaved completion cannot revise the final ledger into an unreplayable local resolution.
			if kept == nil && t.frame.Kind == probe.Data && !result.Attempted {
				if e := c.ledger.Unattempted(identity(t.frame), observed, "canceled_unattempted: "+string(result.Reason)); e != nil && err == nil {
					err = e
				}
			}
		default:
			if err == nil {
				err = errors.New("unresolved completion ticket after join")
			}
		}
	}
	cutoff := c.observation
	if cutoff > c.finalEnd {
		cutoff = c.finalEnd
	}
	complete := err == nil && c.generation >= c.plannedEnd && c.observation >= c.finalEnd
	hash, _ := c.plan.Hash()
	r := Result{PlanHash: hash, Direction: c.direction, GenerationCutoff: profile.Decimal(c.generation), ObservationCutoff: profile.Decimal(cutoff), Complete: complete, EventBytes: profile.Decimal(c.bytes), Records: c.records, Entries: c.ledger.Snapshot(), Receiver: measure.ReceiverEvidence{Complete: complete, Cutoff: cutoff, Arrivals: c.receiver}, Summary: c.ledger.Summarize(cutoff, nil), Warmup: c.ledger.SummarizePhase(cutoff, nil, 1, nil), Analysis: c.ledger.Analyze(cutoff, uint64(c.plan.SpikeThreshold), 2, nil)}
	r.Phases = c.ledger.SummarizePhases(cutoff, nil, len(c.plan.Phases))
	r.Markings = c.markings
	return r, err
}

// Endpoint connects a caller-owned prepared/start-anchored worker to one accounting owner.
// The caller stops after its chosen observation window; no CLI/control lifecycle is started.
type Endpoint struct {
	worker *worker.Endpoint
	done   chan struct{}
	result Result
	err    error
}

// newEndpoint validates the frozen plan digest and connects shared multi-phase accounting.
// Standalone callers apply constant markings; finite lifecycle supplies the physical-phase writer hook.
func newEndpoint(parent context.Context, cfg worker.Config, p profile.Plan, id string, create func(context.Context, worker.Config) (*worker.Endpoint, error), persistence ...Persistence) (*Endpoint, error) {
	if p.Adapter != id || cfg.SendAuth.Adapter != id || cfg.ReceiveAuth.Adapter != id {
		return nil, errors.New("accounting adapter mismatch")
	}
	start := cfg.Start.Sub(cfg.EpochStart)
	if start < 0 {
		return nil, errors.New("invalid accounting start")
	}
	c, err := NewCollector(p, cfg.Direction, uint64(start), cfg.Registry)
	if err != nil {
		return nil, err
	}
	if len(persistence) > 0 {
		c.persistence = persistence[0]
	}
	hash, _ := c.plan.Hash()
	if cfg.SendAuth.Plan != hash || cfg.ReceiveAuth.Plan != hash {
		return nil, errors.New("accounting plan digest mismatch")
	}
	// Standalone constant-marking callers must not silently run a changing plan without the finite hook.
	for _, w := range c.windows {
		if cfg.BeforeWrite == nil && w.Local.Traffic.DSCP != c.windows[0].Local.Traffic.DSCP {
			return nil, errors.New("per-phase markings require lifecycle integration")
		}
	}
	cfg.Windows, cfg.Limits = c.windows, c.plan.Limits
	u, err := create(parent, cfg)
	if err != nil {
		return nil, err
	}
	run := &Endpoint{worker: u, done: make(chan struct{})}
	go run.collect(c, cfg)
	return run, nil
}

// collect drains until joined worker shutdown, recording the first accounting failure as partial.
func (u *Endpoint) collect(c *Collector, cfg worker.Config) {
	defer close(u.done)
	var failed <-chan struct{}
	if c.persistence != nil {
		failed = c.persistence.Failed()
	}
events:
	for {
		var e worker.Event
		select {
		case <-failed:
			c.err = errors.New("local evidence storage failed")
			failed = nil
			u.worker.Close()
			continue
		case value, ok := <-u.worker.Events():
			if !ok {
				break events
			}
			e = value
		}
		// Cutoffs remain observable after raw admission fails; the reserved final record retains them.
		// Other events cannot revise a failed ledger or become claimed complete evidence.
		if e.Kind == "generation_end" {
			c.generation = e.At
		}
		if e.Kind == "observation_end" {
			c.observation = e.At
		}
		if c.err == nil {
			if err := c.Apply(e); err != nil {
				c.err = err
				u.worker.Close()
			}
		}
	}
	err := u.worker.Wait(context.Background())
	u.result, u.err = c.Finish(err, uint64(time.Since(cfg.EpochStart)))
	u.result.Run, u.result.Session, u.result.Epoch, u.result.PeerEpoch = cfg.Run, cfg.Session, cfg.Epoch, cfg.PeerEpoch
}

// Scheduled exposes generation completion independently of remaining writes/receipts.
func (u *Endpoint) Scheduled() <-chan struct{} { return u.worker.Scheduled() }

// Close stops only the transferred endpoint socket and its workers, retaining partial evidence.
func (u *Endpoint) Close() { u.worker.Close() }

// StopGeneration retains reads and queued work for a lifecycle-owned bounded drain.
func (u *Endpoint) StopGeneration() { u.worker.StopGeneration() }

// Wait returns a finalized immutable local result after all owned workers/tickets have joined.
func (u *Endpoint) Wait(ctx context.Context) (Result, error) {
	select {
	case <-u.done:
		return u.result, u.err
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}
