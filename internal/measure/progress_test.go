package measure

import (
	"context"
	"sneaky-port/internal/profile"
	"sync"
	"testing"
)

// TestLiveCountsMatchStrictSummary exercises expiry equality, late/duplicate receipt, local cancellation and warmup.
func TestLiveCountsMatchStrictSummary(t *testing.T) {
	l, _ := NewLedger(32)
	a := NewLiveAccounting()
	check := func(cutoff uint64) { // Compare every published count against the canonical full-ledger rule at this cutoff.
		t.Helper()
		main, warmup := a.Counts(cutoff)
		for i, live := range []LiveCounts{main, warmup} {
			lifecycle := uint8(2)
			if i == 1 {
				lifecycle = 1
			}
			s := l.SummarizePhase(cutoff, nil, lifecycle, nil)
			want := LiveCounts{Planned: s.Planned, DeadlineMisses: s.DeadlineMisses, LocalMisses: s.LocalMisses, Pending: s.Pending, Attempted: s.Attempted, Written: s.Written, WriteFailed: s.WriteFailed, OnTime: s.OnTime, Late: s.Late, Completed: s.RTTSamples}
			if live != want {
				t.Fatalf("cutoff=%d lifecycle=%d got %+v want %+v", cutoff, lifecycle, live, want)
			}
		}
	}
	id := Identity{Lifecycle: 2, Sequence: 0}
	warm := Identity{Lifecycle: 1, Sequence: 1}
	local := Identity{Lifecycle: 2, Sequence: 2}
	if err := l.Schedule(id, 10, 20); err != nil {
		t.Fatal(err)
	}
	l.Admit(id, 10, "accepted", "")
	l.Attempt(id, 11)
	l.Written(id, 12, 100, true)
	l.TrackProgress(a, id, 13)
	check(13)
	check(20)
	check(21)
	if err := l.ReceiveReceipt(id, 11, 22); err != nil {
		t.Fatal(err)
	}
	l.TrackProgress(a, id, 23)
	check(23)
	if err := l.ReceiveReceipt(id, 11, 23); err != nil {
		t.Fatal(err)
	}
	l.TrackProgress(a, id, 24)
	check(24)
	l.Schedule(warm, 25, 35)
	l.Admit(warm, 25, "accepted", "")
	l.Attempt(warm, 26)
	l.TrackProgress(a, warm, 27)
	check(27)
	l.ReceiveReceipt(warm, 26, 35)
	l.TrackProgress(a, warm, 35)
	check(35)
	l.TrackProgress(a, warm, 36)
	check(36)
	l.Schedule(local, 40, 50)
	l.Admit(local, 40, "accepted", "")
	l.TrackProgress(a, local, 41)
	check(41)
	check(51)
	l.Unattempted(local, 52, "canceled_unattempted")
	l.TrackProgress(a, local, 53)
	check(53)
	if len(a.slots) != 3 || len(a.deadlines) != 0 {
		t.Fatal("live state exceeded reached identities or retained expired deadlines")
	}
}

// TestLiveObservationCutoffRebase excludes a receipt/expiry past a late-learned cancellation cutoff.
func TestLiveObservationCutoffRebase(t *testing.T) {
	l, _ := NewLedger(8)
	a := NewLiveAccounting()
	id := Identity{Lifecycle: 2}
	l.Schedule(id, 10, 20)
	l.Admit(id, 10, "accepted", "")
	l.Attempt(id, 11)
	l.TrackProgress(a, id, 12)
	late, _ := a.Counts(30)
	if late.DeadlineMisses != 1 {
		t.Fatal("fixture deadline did not expire")
	}
	a.Limit(20)
	sealed, _ := a.Counts(40)
	canonical := l.Summarize(20, nil)
	if sealed.DeadlineMisses != canonical.DeadlineMisses || sealed.Pending != canonical.Pending || a.cutoff != 20 {
		t.Fatal("strict actual cutoff not restored", sealed, canonical)
	}
	l.ReceiveReceipt(id, 11, 22)
	l.TrackProgress(a, id, 23)
	sealed, _ = a.Counts(50)
	if sealed.Completed != 0 || sealed.Pending != 1 || sealed.DeadlineMisses != 0 {
		t.Fatal("post-cutoff receipt or expiry changed sealed counts", sealed)
	}
	a.Limit(21)
	if a.ceiling != 20 {
		t.Fatal("actual cutoff was widened")
	}
}

// TestProgressFeedIndependentCopies verifies nonblocking coalescing, concurrent readers and run-local context ownership.
func TestProgressFeedIndependentCopies(t *testing.T) {
	feed := &ProgressFeed{}
	if feed.Snapshot() != nil || ProgressSink(context.Background()) != nil || ProgressSink(nil) != nil {
		t.Fatal("unexpected initial progress")
	}
	ctx := WithProgress(context.Background(), feed)
	if ProgressSink(ctx) != feed {
		t.Fatal("run feed lost")
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Go(func() { // Slow/repeated readers cannot mutate or block the publishing owner.
			for j := 0; j < 10000; j++ {
				if s := feed.Snapshot(); s != nil {
					s.Main.Planned = 0
					s.RTTTime[0].Count = 0
				}
			}
		})
	}
	for j := 1; j <= 10000; j++ {
		value := Progress{AsOf: profile.Decimal(j), Main: LiveCounts{Planned: profile.Decimal(j)}, RTTTime: []LiveRTTRange{{Count: 1, Min: 9007199254740993, Max: 9007199254740993}}}
		feed.Publish(value)
		value.RTTTime[0].Count = 0
	}
	wg.Wait()
	if s := feed.Snapshot(); s.Main.Planned != 10000 || s.RTTTime[0].Count != 1 || s.RTTTime[0].Max != 9007199254740993 {
		t.Fatal("reader changed immutable snapshot")
	}
}

// TestLiveRTTRanges preserves actual attempt-to-receipt extrema, late samples and strict cutoff rebasing.
func TestLiveRTTRanges(t *testing.T) {
	l, _ := NewLedger(16)
	a := NewLiveAccounting()
	// complete adds one valid origin receipt; its local write/processing times are not RTT boundaries.
	complete := func(sequence uint64, lifecycle uint8, scheduled, attempt, receipt uint64) Identity {
		t.Helper()
		id := Identity{Lifecycle: lifecycle, Sequence: profile.Decimal(sequence)}
		if err := l.Schedule(id, scheduled, scheduled+10); err != nil {
			t.Fatal(err)
		}
		l.Admit(id, scheduled, "accepted", "")
		l.Attempt(id, attempt)
		if err := l.ReceiveReceipt(id, attempt, receipt); err != nil {
			t.Fatal(err)
		}
		l.TrackProgress(a, id, receipt+1)
		return id
	}
	id := complete(0, 2, 100, 103, 108)
	complete(1, 2, 200, 203, 250) // Late completed RTT remains a real sample.
	complete(2, 1, 300, 303, 310) // Warmup cannot enter main latency.
	l.ReceiveReceipt(id, 103, 311)
	l.TrackProgress(a, id, 312)
	local := Identity{Lifecycle: 2, Sequence: 3}
	l.Schedule(local, 400, 410)
	l.Admit(local, 400, "skipped_local", "fixture")
	l.TrackProgress(a, local, 401)
	a.Counts(1000000100)
	ranges := a.RTTRanges(100)
	if len(ranges) != 1 || ranges[0] != (LiveRTTRange{Start: 0, End: 999999900, Count: 2, Min: 5, Max: 47}) {
		t.Fatal("RTT boundaries/population/elapsed range changed", ranges)
	}
	ranges[0].Min = 0
	if a.RTTRanges(100)[0].Min != 5 {
		t.Fatal("range reader changed owner state")
	}
	a.Limit(250) // Equality excludes the late receipt at 250, preserving the first receipt.
	ranges = a.RTTRanges(100)
	if len(ranges) != 1 || ranges[0] != (LiveRTTRange{Start: 0, End: 150, Count: 1, Min: 5, Max: 5}) {
		t.Fatal("actual cutoff did not rebase RTT", ranges)
	}
	a.Limit(108)
	if len(a.RTTRanges(100)) != 0 {
		t.Fatal("cutoff-equal receipt became a latency sample")
	}
}

// TestLiveRTTRangeBound retains a fixed recent time window despite reordering and late old receipts.
func TestLiveRTTRangeBound(t *testing.T) {
	l, _ := NewLedger(512)
	a := NewLiveAccounting()
	id := Identity{Lifecycle: 2, Sequence: 200}
	l.Schedule(id, 0, 100)
	l.Admit(id, 0, "accepted", "")
	l.Attempt(id, 1)
	l.TrackProgress(a, id, 2)
	for i := uint64(0); i < 200; i++ {
		id := Identity{Lifecycle: 2, Sequence: profile.Decimal(i)}
		scheduled := i * LiveRTTBucketNS
		if err := l.Schedule(id, scheduled, scheduled+100); err != nil {
			t.Fatal(err)
		}
		l.Admit(id, scheduled, "accepted", "")
		l.Attempt(id, scheduled+1)
		if err := l.ReceiveReceipt(id, scheduled+1, scheduled+7); err != nil {
			t.Fatal(err)
		}
		l.TrackProgress(a, id, scheduled+8)
	}
	if err := l.ReceiveReceipt(id, 1, 200*LiveRTTBucketNS); err != nil {
		t.Fatal(err)
	}
	l.TrackProgress(a, id, 200*LiveRTTBucketNS+1)
	ranges := a.RTTRanges(0)
	main, _ := a.Counts(200*LiveRTTBucketNS + 1)
	if len(ranges) != LiveRTTBuckets || main.Completed != 201 || ranges[0].Start != 72*profile.Decimal(LiveRTTBucketNS) || ranges[len(ranges)-1].Start != 199*profile.Decimal(LiveRTTBucketNS) {
		t.Fatal("latency retention changed counters/window", main, len(ranges))
	}
	for _, r := range ranges {
		if r.Count != 1 || r.Min != 6 || r.Max != 6 {
			t.Fatal("old late sample changed retained envelopes", r)
		}
	}
}
