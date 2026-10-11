// Package measure records identity-based outcomes without equating receipts with delivery.
package measure

import (
	"errors"
	"sort"
	"strconv"

	"sneaky-port/internal/profile"
)

// Identity keeps warmup/main, originating phase and original sequence distinct.
type Identity struct {
	Lifecycle uint8           `json:"lifecycle"`
	Phase     uint32          `json:"phase"`
	Sequence  profile.Decimal `json:"sequence"`
}

// Entry preserves the local schedule, actual writes and first eligible receipt.
type Entry struct {
	Identity       Identity          `json:"identity"`
	Scheduled      profile.Decimal   `json:"scheduled_ns"`
	Deadline       profile.Decimal   `json:"deadline_ns"`
	State          string            `json:"state"`
	Reason         string            `json:"reason"`
	Attempt        *profile.Decimal  `json:"attempt_ns"`
	WriteCompleted *profile.Decimal  `json:"write_completed_ns"`
	PartialBytes   profile.Decimal   `json:"partial_bytes"`
	Receipt        *profile.Decimal  `json:"receipt_ns"`
	Duplicates     profile.Decimal   `json:"duplicate_receipts"`
	DuplicateTimes []profile.Decimal `json:"duplicate_receipt_times_ns"`
}

// Ledger is owned by one engine event loop; adapters do not mutate its classifications.
type Ledger struct {
	cap            int
	entries        map[Identity]*Entry
	lastScheduled  uint64
	hasScheduled   bool
	sequences      map[profile.Decimal]bool
	duplicateCount int
}

// NewLedger enforces a positive finite evidence cap before observing traffic.
func NewLedger(capacity int) (*Ledger, error) {
	if capacity < 1 || capacity > 1<<20 {
		return nil, errors.New("invalid ledger capacity")
	}
	return &Ledger{cap: capacity, entries: map[Identity]*Entry{}, sequences: map[profile.Decimal]bool{}}, nil
}

// Schedule records only slots actually reached before the endpoint's stop boundary.
func (l *Ledger) Schedule(id Identity, scheduled, deadline uint64) error {
	if id.Lifecycle < 1 || id.Lifecycle > 2 || deadline < scheduled || l.hasScheduled && scheduled < l.lastScheduled {
		return errors.New("invalid schedule identity/times")
	}
	if _, exists := l.entries[id]; exists {
		return errors.New("identity reuse")
	}
	if l.sequences[id.Sequence] {
		return errors.New("sequence reuse across lifecycle/phases")
	}
	if len(l.entries)+l.duplicateCount >= l.cap {
		return errors.New("ledger budget exhausted")
	}
	l.entries[id] = &Entry{Identity: id, Scheduled: profile.Decimal(scheduled), Deadline: profile.Decimal(deadline), State: "planned"}
	l.lastScheduled = scheduled
	l.hasScheduled = true
	l.sequences[id.Sequence] = true
	return nil
}

// Skip accounts a local failure separately from attempted traffic or network delivery.
func (l *Ledger) Skip(id Identity, reason string) error {
	e, ok := l.entries[id]
	if !ok || e.State != "planned" || reason == "" {
		return errors.New("invalid skip transition")
	}
	e.State = "skipped_local"
	e.Reason = reason
	return nil
}

// Attempt marks the declared serialization/socket-submission boundary after queue waiting.
func (l *Ledger) Attempt(id Identity, at uint64) error {
	e, ok := l.entries[id]
	if !ok || e.State != "planned" || at < uint64(e.Scheduled) {
		return errors.New("invalid attempt transition")
	}
	v := profile.Decimal(at)
	e.Attempt = &v
	e.State = "attempted"
	return nil
}

// Written preserves complete versus failed/partial socket writes without asserting arrival.
func (l *Ledger) Written(id Identity, at, bytes uint64, complete bool) error {
	e, ok := l.entries[id]
	if !ok || e.Attempt == nil || e.WriteCompleted != nil || at < uint64(*e.Attempt) {
		return errors.New("invalid write transition")
	}
	v := profile.Decimal(at)
	e.WriteCompleted = &v
	e.PartialBytes = profile.Decimal(bytes)
	if complete {
		e.State = "written"
	} else {
		e.State = "write_failed"
		e.Reason = "partial/failed write; remote delivery may be unknown"
	}
	return nil
}

// ReceiveReceipt records the first authenticated matching receipt; duplicates add no sample.
func (l *Ledger) ReceiveReceipt(id Identity, originAttempt, at uint64) error {
	e, ok := l.entries[id]
	if !ok || e.Attempt == nil || uint64(*e.Attempt) != originAttempt || at < originAttempt {
		return errors.New("foreign/invalid receipt")
	}
	if e.Receipt != nil {
		if len(l.entries)+l.duplicateCount >= l.cap {
			return errors.New("duplicate evidence budget exhausted")
		}
		e.Duplicates++
		e.DuplicateTimes = append(e.DuplicateTimes, profile.Decimal(at))
		l.duplicateCount++
		return nil
	}
	v := profile.Decimal(at)
	e.Receipt = &v
	return nil
}

// Snapshot copies entries in original sequence order for deterministic raw replay/export.
func (l *Ledger) Snapshot() []Entry {
	out := make([]Entry, 0, len(l.entries))
	for _, e := range l.entries {
		copy := *e
		copy.DuplicateTimes = append([]profile.Decimal(nil), e.DuplicateTimes...)
		if e.Attempt != nil {
			v := *e.Attempt
			copy.Attempt = &v
		}
		if e.WriteCompleted != nil {
			v := *e.WriteCompleted
			copy.WriteCompleted = &v
		}
		if e.Receipt != nil {
			v := *e.Receipt
			copy.Receipt = &v
		}
		out = append(out, copy)
	}
	sort.Slice(out, func(i, j int) bool { // Original sequence controls RTT variation; arrival order does not.
		return out[i].Identity.Sequence < out[j].Identity.Sequence
	})
	return out
}

// ReceiverEvidence contains compatible endpoint observations, not receipt-derived guesses.
type ReceiverEvidence struct {
	Complete bool
	Arrivals map[Identity]uint64
	Cutoff   uint64
}

// Summary preserves denominators and nullable network-loss/receiver evidence.
type Summary struct {
	Planned, Skipped, Attempted, Written, WriteFailed, OnTime, Late, Unresolved, DuplicateReceipts profile.Decimal
	Received, MissingAtReceiver                                                                    *profile.Decimal
	RTTMedian, RTTP95, RTTP99                                                                      *string
	RTTSamples                                                                                     profile.Decimal
	RTTVariation                                                                                   []profile.Decimal
	VariationGaps                                                                                  profile.Decimal
	TransportPacketLoss                                                                            *string
	ReceiverStatus                                                                                 string
}

// Summarize reconciles main traffic at a strict cutoff; warmup never changes main metrics.
func (l *Ledger) Summarize(cutoff uint64, remote *ReceiverEvidence) Summary {
	return l.SummarizePhase(cutoff, remote, 2, nil)
}

// SummarizePhase exposes separate lifecycle/phase denominators and preserves network loss unknown.
func (l *Ledger) SummarizePhase(cutoff uint64, remote *ReceiverEvidence, lifecycle uint8, phase *uint32) Summary {
	out := Summary{ReceiverStatus: "unknown; no complete compatible receiver ledger"}
	var rtts []uint64
	var previous *Entry
	var received, missing profile.Decimal
	for _, e := range l.Snapshot() {
		if uint64(e.Scheduled) >= cutoff || e.Identity.Lifecycle != lifecycle || phase != nil && e.Identity.Phase != *phase {
			continue
		}
		out.Planned++
		if e.State == "skipped_local" {
			out.Skipped++
			continue
		}
		if e.Attempt == nil {
			out.Unresolved++
			continue
		}
		out.Attempted++
		if e.State == "written" {
			out.Written++
		} else if e.State == "write_failed" {
			out.WriteFailed++
		}
		for _, at := range e.DuplicateTimes {
			if uint64(at) < cutoff {
				out.DuplicateReceipts++
			}
		}
		if e.Receipt != nil && uint64(*e.Receipt) < cutoff {
			rtt := uint64(*e.Receipt) - uint64(*e.Attempt)
			rtts = append(rtts, rtt)
			if *e.Receipt <= e.Deadline {
				out.OnTime++
			} else {
				out.Late++
			}
			if previous != nil && e.Identity.Lifecycle == previous.Identity.Lifecycle && e.Identity.Phase == previous.Identity.Phase && e.Identity.Sequence == previous.Identity.Sequence+1 {
				before := uint64(*previous.Receipt) - uint64(*previous.Attempt)
				diff := rtt
				if diff >= before {
					diff -= before
				} else {
					diff = before - diff
				}
				out.RTTVariation = append(out.RTTVariation, profile.Decimal(diff))
			} else if previous != nil && e.Identity.Lifecycle == previous.Identity.Lifecycle && e.Identity.Phase == previous.Identity.Phase {
				out.VariationGaps++
			}
			copy := e
			previous = &copy
		} else {
			out.Unresolved++
		}
		if remote != nil && remote.Complete && remote.Cutoff > 0 {
			if arrival, ok := remote.Arrivals[e.Identity]; ok && arrival < remote.Cutoff {
				received++
			} else if e.State == "written" {
				missing++
			}
		}
	}
	out.RTTSamples = profile.Decimal(len(rtts))
	out.RTTMedian = Percentile(rtts, 50)
	out.RTTP95 = Percentile(rtts, 95)
	out.RTTP99 = Percentile(rtts, 99)
	if remote != nil && remote.Complete && remote.Cutoff > 0 {
		out.Received = &received
		out.MissingAtReceiver = &missing
		out.ReceiverStatus = "complete compatible receiver evidence; missing application data is not TCP packet-loss evidence"
	}
	return out
}

// Percentile uses nearest rank on all completed samples; empty input remains unavailable.
func Percentile(values []uint64, percent uint64) *string {
	if len(values) == 0 || percent == 0 || percent > 100 {
		return nil
	}
	sorted := append([]uint64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { // Preserve raw input order while computing exact final quantiles.
		return sorted[i] < sorted[j]
	})
	index := (uint64(len(sorted))*percent+99)/100 - 1
	s := strconv.FormatUint(sorted[index], 10)
	return &s
}
