package measure

import (
	"container/heap"
	"context"
	"slices"
	"sneaky-port/internal/profile"
	"sync/atomic"
)

// LiveCounts are provisional reached-slot outcomes, excluding tails and unverified remote delivery.
type LiveCounts struct {
	Planned        profile.Decimal `json:"planned"`
	DeadlineMisses profile.Decimal `json:"deadline_misses"`
	LocalMisses    profile.Decimal `json:"local_misses"`
	Pending        profile.Decimal `json:"pending"`
	Attempted      profile.Decimal `json:"attempted"`
	Written        profile.Decimal `json:"written"`
	WriteFailed    profile.Decimal `json:"write_failed"`
	OnTime         profile.Decimal `json:"on_time"`
	Late           profile.Decimal `json:"late"`
	Completed      profile.Decimal `json:"completed"`
}

// LiveRTTBucketNS and LiveRTTBuckets bound provisional RTT ranges independently of network queues.
const LiveRTTBucketNS, LiveRTTBuckets = uint64(1000000000), 128

// LiveRTTRange retains exact completed RTT extrema/count in a half-open scheduled-time bucket.
// Times are client-local elapsed nanoseconds; absent receipts never become zero-duration samples.
type LiveRTTRange struct {
	Start profile.Decimal `json:"start_ns"`
	End   profile.Decimal `json:"end_ns"`
	Count profile.Decimal `json:"count"`
	Min   profile.Decimal `json:"min_ns"`
	Max   profile.Decimal `json:"max_ns"`
}

// Progress has fixed-size counters, bounded RTT ranges and phase identity; it exposes no keys or raw records.
// AsOf is host-local publication time, not a remote clock or proof of finalized observation completeness.
type Progress struct {
	AsOf            profile.Decimal `json:"as_of_ns"`
	Cutoff          profile.Decimal `json:"counts_cutoff_ns"`
	AccountingValid bool            `json:"accounting_valid"`
	Elapsed         profile.Decimal `json:"elapsed_ns"`
	Total           profile.Decimal `json:"total_ns"`
	Stage           string          `json:"stage"`
	Phase           string          `json:"phase_id"`
	Main            LiveCounts      `json:"main"`
	Warmup          LiveCounts      `json:"warmup"`
	PeerReceived    profile.Decimal `json:"peer_data_received"`
	RTTTime         []LiveRTTRange  `json:"rtt_time"`
}

// ProgressFeed coalesces into one atomic immutable snapshot; readers never execute on the accounting owner.
type ProgressFeed struct{ latest atomic.Pointer[Progress] }
type progressKey struct{}

// WithProgress attaches a run-owned feed without changing scheduling, authentication or runner signatures.
func WithProgress(ctx context.Context, feed *ProgressFeed) context.Context {
	return context.WithValue(ctx, progressKey{}, feed)
}

// ProgressSink returns an optional feed; ordinary CLI and offline replay have no live accounting overhead.
func ProgressSink(ctx context.Context) *ProgressFeed {
	if ctx == nil {
		return nil
	}
	feed, _ := ctx.Value(progressKey{}).(*ProgressFeed)
	return feed
}

// Publish replaces one bounded snapshot without waiting on a consumer or invoking UI callbacks.
func (f *ProgressFeed) Publish(value Progress) {
	copy := value
	copy.RTTTime = slices.Clone(value.RTTTime)
	f.latest.Store(&copy)
}

// Snapshot copies counters and bounded ranges; nil means no engine observation has been published.
func (f *ProgressFeed) Snapshot() *Progress {
	if value := f.latest.Load(); value != nil {
		copy := *value
		copy.RTTTime = slices.Clone(value.RTTTime)
		return &copy
	}
	return nil
}

// liveSlot owns one original entry and its last provisional contribution on the ledger owner only.
type liveSlot struct {
	entry  *Entry
	counts LiveCounts
}
type liveDeadline struct {
	at   uint64
	slot *liveSlot
}
type liveDeadlines []liveDeadline

// Len supplies the bounded heap size; deadlines are inserted once per reached identity.
func (h liveDeadlines) Len() int { return len(h) }

// Less orders strict deadline expiry; equal deadlines have identical accounting semantics.
func (h liveDeadlines) Less(i, j int) bool { return h[i].at < h[j].at }

// Swap preserves the heap order without touching the actual scheduling ledger.
func (h liveDeadlines) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push admits one caller-bounded presentation deadline; it never schedules network work.
func (h *liveDeadlines) Push(x any) { *h = append(*h, x.(liveDeadline)) }

// Pop releases an expired heap reference while the bounded identity index retains the contribution.
func (h *liveDeadlines) Pop() any {
	old := *h
	x := old[len(old)-1]
	old[len(old)-1] = liveDeadline{}
	*h = old[:len(old)-1]
	return x
}

// LiveAccounting incrementally maintains the same strict-cutoff count rules without copying/sorting the ledger.
// Only the collector owns this object; API readers see the separate immutable ProgressFeed.
type LiveAccounting struct {
	slots        map[Identity]*liveSlot
	deadlines    liveDeadlines
	main, warmup LiveCounts
	cutoff       uint64
	ceiling      uint64
	sealed       bool
	rtt          map[uint64]LiveRTTRange
	lastBucket   uint64
}

// NewLiveAccounting allocates optional accounting state bounded by the existing ledger admission cap.
func NewLiveAccounting() *LiveAccounting {
	return &LiveAccounting{slots: map[Identity]*liveSlot{}, rtt: map[uint64]LiveRTTRange{}}
}

// liveContribution matches summary count semantics for one reached origin; missing receipts are never packet loss.
func liveContribution(e *Entry, cutoff uint64) LiveCounts {
	c := LiveCounts{}
	if uint64(e.Scheduled) >= cutoff {
		return c
	}
	c.Planned = 1
	if locallySkipped(*e, cutoff) {
		c.LocalMisses = 1
		c.DeadlineMisses = 1
		return c
	}
	if e.Attempt != nil && uint64(*e.Attempt) < cutoff {
		c.Attempted = 1
		if e.WriteCompleted != nil && uint64(*e.WriteCompleted) < cutoff {
			if e.State == "written" {
				c.Written = 1
			}
			if e.State == "write_failed" {
				c.WriteFailed = 1
			}
		}
		if e.Receipt != nil && uint64(*e.Receipt) < cutoff {
			c.Completed = 1
			if *e.Receipt <= e.Deadline {
				c.OnTime = 1
			} else {
				c.Late = 1
				c.DeadlineMisses = 1
			}
			return c
		}
	}
	if uint64(e.Deadline) < cutoff {
		c.DeadlineMisses = 1
	} else {
		c.Pending = 1
	}
	return c
}

// replaceContribution adjusts exact unsigned totals by subtracting this slot's previous contribution first.
func replaceContribution(total *LiveCounts, old, next LiveCounts) {
	total.Planned = total.Planned - old.Planned + next.Planned
	total.DeadlineMisses = total.DeadlineMisses - old.DeadlineMisses + next.DeadlineMisses
	total.LocalMisses = total.LocalMisses - old.LocalMisses + next.LocalMisses
	total.Pending = total.Pending - old.Pending + next.Pending
	total.Attempted = total.Attempted - old.Attempted + next.Attempted
	total.Written = total.Written - old.Written + next.Written
	total.WriteFailed = total.WriteFailed - old.WriteFailed + next.WriteFailed
	total.OnTime = total.OnTime - old.OnTime + next.OnTime
	total.Late = total.Late - old.Late + next.Late
	total.Completed = total.Completed - old.Completed + next.Completed
}

// update revises one original contribution after an event or strict deadline expiry.
func (a *LiveAccounting) update(slot *liveSlot, cutoff uint64) {
	next := liveContribution(slot.entry, cutoff)
	total := &a.main
	if slot.entry.Identity.Lifecycle == 1 {
		total = &a.warmup
	}
	replaceContribution(total, slot.counts, next)
	if slot.entry.Identity.Lifecycle == 2 && slot.counts.Completed == 0 && next.Completed == 1 {
		a.addRTT(slot.entry)
	}
	slot.counts = next
}

// addRTT aggregates the first eligible main receipt only; old buckets age out without changing counts.
func (a *LiveAccounting) addRTT(e *Entry) {
	key := uint64(e.Scheduled) / LiveRTTBucketNS
	if key > a.lastBucket {
		a.lastBucket = key
		for bucket := range a.rtt {
			if a.lastBucket-bucket >= LiveRTTBuckets {
				delete(a.rtt, bucket)
			}
		}
	}
	if a.lastBucket-key >= LiveRTTBuckets {
		return
	}
	r := a.rtt[key]
	duration := *e.Receipt - *e.Attempt
	if r.Count == 0 {
		r.Min, r.Max = duration, duration
	}
	r.Count++
	r.Min, r.Max = min(r.Min, duration), max(r.Max, duration)
	a.rtt[key] = r
}

// RTTRanges copies sorted bounded ranges relative to run start and clips the actual observation cutoff.
// These provisional envelopes cannot supply quantiles, one-way delay or missing-receipt latency.
func (a *LiveAccounting) RTTRanges(start uint64) []LiveRTTRange {
	keys := make([]uint64, 0, len(a.rtt))
	for key := range a.rtt {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make([]LiveRTTRange, 0, len(keys))
	for _, key := range keys {
		r := a.rtt[key]
		begin := key * LiveRTTBucketNS
		end := min(begin+min(LiveRTTBucketNS, ^uint64(0)-begin), a.cutoff)
		if end <= start {
			continue
		}
		r.Start, r.End = profile.Decimal(max(begin, start)-start), profile.Decimal(end-start)
		out = append(out, r)
	}
	return out
}

// TrackProgress observes existing ledger entries without exposing mutable references to consumers.
func (l *Ledger) TrackProgress(a *LiveAccounting, id Identity, cutoff uint64) {
	if a.sealed {
		cutoff = min(cutoff, a.ceiling)
	}
	cutoff = max(cutoff, a.cutoff)
	a.cutoff = cutoff
	e, ok := l.entries[id]
	if !ok {
		return
	}
	slot, ok := a.slots[id]
	if !ok {
		slot = &liveSlot{entry: e}
		a.slots[id] = slot
		heap.Push(&a.deadlines, liveDeadline{uint64(e.Deadline), slot})
	}
	a.update(slot, cutoff)
}

// Counts advances only expired presentation deadlines and returns scalar copies, never full sample populations.
func (a *LiveAccounting) Counts(cutoff uint64) (LiveCounts, LiveCounts) {
	if a.sealed {
		cutoff = min(cutoff, a.ceiling)
	}
	cutoff = max(cutoff, a.cutoff)
	a.cutoff = cutoff
	for len(a.deadlines) > 0 && a.deadlines[0].at < cutoff {
		item := heap.Pop(&a.deadlines).(liveDeadline)
		a.update(item.slot, cutoff)
	}
	return a.main, a.warmup
}

// Limit seals the actual half-open observation cutoff when cancellation/join reveals it.
// A publication may have used a later provisional time before the end event was drained;
// rebase those bounded slot contributions once, rather than claiming observations past shutdown.
func (a *LiveAccounting) Limit(cutoff uint64) {
	if a.sealed && cutoff >= a.ceiling {
		return
	}
	a.sealed = true
	a.ceiling = cutoff
	if a.cutoff <= cutoff {
		return
	}
	a.cutoff = cutoff
	a.main = LiveCounts{}
	a.warmup = LiveCounts{}
	clear(a.rtt)
	a.lastBucket = 0
	clear(a.deadlines)
	a.deadlines = a.deadlines[:0]
	for _, slot := range a.slots {
		slot.counts = LiveCounts{}
		a.update(slot, cutoff)
		if uint64(slot.entry.Deadline) >= cutoff {
			a.deadlines = append(a.deadlines, liveDeadline{uint64(slot.entry.Deadline), slot})
		}
	}
	heap.Init(&a.deadlines)
}
