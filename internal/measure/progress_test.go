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
				}
			}
		})
	}
	for j := 1; j <= 10000; j++ {
		feed.Publish(Progress{AsOf: profile.Decimal(j), Main: LiveCounts{Planned: profile.Decimal(j)}})
	}
	wg.Wait()
	if feed.Snapshot().Main.Planned != 10000 {
		t.Fatal("reader changed immutable snapshot")
	}
}
