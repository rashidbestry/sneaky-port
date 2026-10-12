package measure

import (
	"math"
	"sort"

	"sneaky-port/internal/profile"
)

// Spike groups consecutive original-sequence samples strictly above the explicit threshold.
// Times are origin attempt offsets; duration is last minus first, not a network outage.
type Spike struct {
	First, Last                       Identity
	Count, Start, End, Duration, Peak profile.Decimal
	AdjacentFailures                  []Identity
}

// Stall is an application receipt-completion gap with accepted overdue work.
// It is bounded by observed completion/cutoff, not proof of a carrier outage.
type Stall struct{ Start, End, Duration profile.Decimal }

// Analysis keeps explicit thresholds and sample-derived baseline beside grouped episodes.
type Analysis struct {
	Threshold      profile.Decimal
	BaselineMedian *string
	Spikes         []Spike
	Stalls         []Stall
}

// Analyze groups spikes by original sequence and stalls by local completion order.
// Warmup and optional phase filters use the same eligible population as summaries.
func (l *Ledger) Analyze(cutoff, threshold uint64, lifecycle byte, phase *uint32) Analysis {
	return AnalyzeSnapshot(l.Snapshot(), cutoff, threshold, lifecycle, phase)
}

// AnalyzeSnapshot applies the same episode rules to immutable replayed entries for saved comparisons.
// It preserves original sequence order and host-local cutoff semantics; it does not establish network outages.
func AnalyzeSnapshot(snapshot []Entry, cutoff, threshold uint64, lifecycle byte, phase *uint32) Analysis {
	out := Analysis{Threshold: profile.Decimal(threshold)}
	entries := []Entry{}
	byID := map[Identity]Entry{}
	rtts := []uint64{}
	for _, e := range snapshot {
		if uint64(e.Scheduled) < cutoff && e.Identity.Lifecycle == lifecycle && (phase == nil || e.Identity.Phase == *phase) {
			entries = append(entries, e)
			byID[e.Identity] = e
		}
	}
	var last *Entry
	for _, e := range entries {
		if e.Attempt == nil || e.Receipt == nil || uint64(*e.Attempt) >= cutoff || uint64(*e.Receipt) >= cutoff {
			last = nil
			continue
		}
		rtt := uint64(*e.Receipt) - uint64(*e.Attempt)
		rtts = append(rtts, rtt)
		if rtt <= threshold {
			last = nil
			continue
		}
		if last == nil || last.Identity.Phase != e.Identity.Phase || last.Identity.Sequence+1 != e.Identity.Sequence {
			out.Spikes = append(out.Spikes, Spike{First: e.Identity, Start: *e.Attempt})
		}
		s := &out.Spikes[len(out.Spikes)-1]
		s.Last, s.End = e.Identity, *e.Attempt
		s.Count++
		s.Duration = s.End - s.Start
		if profile.Decimal(rtt) > s.Peak {
			s.Peak = profile.Decimal(rtt)
		}
		copy := e
		last = &copy
	}
	out.BaselineMedian = Percentile(rtts, 50)
	for i := range out.Spikes {
		s := &out.Spikes[i]
		adjacent := []Identity{}
		if s.First.Sequence > 0 {
			id := s.First
			id.Sequence--
			adjacent = append(adjacent, id)
		}
		if s.Last.Sequence < profile.Decimal(math.MaxUint64) {
			id := s.Last
			id.Sequence++
			adjacent = append(adjacent, id)
		}
		for _, id := range adjacent {
			if e, ok := byID[id]; ok && (locallySkipped(e, cutoff) || e.Receipt == nil || uint64(*e.Receipt) >= cutoff || *e.Receipt > e.Deadline) {
				s.AdjacentFailures = append(s.AdjacentFailures, id)
			}
		}
	}
	out.Stalls = completionStalls(entries, cutoff)
	return out
}

// completionStalls sweeps deadline/completion events rather than treating intentional quiet as loss.
// Completion at deadline is on time. Every valid first completion breaks an ongoing stall,
// even when another identity remains overdue; the next interval starts at that completion.
func completionStalls(entries []Entry, cutoff uint64) []Stall {
	type point struct {
		at         uint64
		delta      int
		completion bool
	}
	points := []point{}
	for _, e := range entries {
		if locallySkipped(e, cutoff) {
			continue
		}
		accepted := e.Admitted != nil && e.Admission == "accepted" && uint64(*e.Admitted) < cutoff
		if !accepted && (e.Attempt == nil || uint64(*e.Attempt) >= cutoff) {
			continue
		}
		hasReceipt := e.Receipt != nil && uint64(*e.Receipt) < cutoff
		if hasReceipt {
			points = append(points, point{at: uint64(*e.Receipt), completion: true})
		}
		start := uint64(e.Deadline)
		if accepted && uint64(*e.Admitted) > start {
			start = uint64(*e.Admitted)
		}
		if start >= cutoff || hasReceipt && uint64(*e.Receipt) <= start {
			continue
		}
		points = append(points, point{at: start, delta: 1})
		if hasReceipt {
			points = append(points, point{at: uint64(*e.Receipt), delta: -1})
		}
	}
	sort.Slice(points, func(i, j int) bool { // Timestamp batches avoid spurious zero-length intervals at equal deadlines/completions.
		return points[i].at < points[j].at
	})
	var out []Stall
	active := 0
	var start uint64
	for i := 0; i < len(points); {
		at := points[i].at
		completion := false
		delta := 0
		for i < len(points) && points[i].at == at {
			delta += points[i].delta
			completion = completion || points[i].completion
			i++
		}
		if active > 0 && completion && at > start {
			out = append(out, Stall{profile.Decimal(start), profile.Decimal(at), profile.Decimal(at - start)})
		}
		before := active
		active += delta
		if active > 0 && (before == 0 || completion) {
			start = at
		}
	}
	if active > 0 && cutoff > start {
		out = append(out, Stall{profile.Decimal(start), profile.Decimal(cutoff), profile.Decimal(cutoff - start)})
	}
	return out
}
