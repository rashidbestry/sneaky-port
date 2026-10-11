package measure

import (
	"context"
	"testing"

	"sneaky-port/internal/profile"
)

// TestReceiptIsNotReceiverLoss distinguishes lost data, lost receipts and local skipped slots.
func TestReceiptIsNotReceiverLoss(t *testing.T) {
	l, _ := NewLedger(10)
	a := Identity{Lifecycle: 2, Phase: 0, Sequence: 0}
	b := Identity{Lifecycle: 2, Phase: 0, Sequence: 1}
	c := Identity{Lifecycle: 2, Phase: 0, Sequence: 2}
	for i, id := range []Identity{a, b, c} {
		if err := l.Schedule(id, uint64(i*100), uint64(i*100+50)); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Skip(c, "queue exhausted"); err != nil {
		t.Fatal(err)
	}
	for i, id := range []Identity{a, b} {
		if err := l.Attempt(id, uint64(i*100)); err != nil {
			t.Fatal(err)
		}
		if err := l.Written(id, uint64(i*100+1), 128, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.ReceiveReceipt(a, 0, 50); err != nil {
		t.Fatal(err)
	}
	if err := l.ReceiveReceipt(a, 0, 51); err != nil {
		t.Fatal(err)
	}
	s := l.Summarize(300, nil)
	if s.Planned != 3 || s.Skipped != 1 || s.OnTime != 1 || s.Unresolved != 1 || s.DuplicateReceipts != 1 || s.Received != nil || s.TransportPacketLoss != nil {
		t.Fatalf("incorrect accounting %+v", s)
	}
	remote := &ReceiverEvidence{Complete: true, Cutoff: 500, Arrivals: map[Identity]uint64{a: 20, b: 120}}
	s = l.Summarize(300, remote)
	if *s.Received != 2 || *s.MissingAtReceiver != 0 || s.Unresolved != 1 {
		t.Fatal("lost receipt mislabeled as lost data")
	}
	delete(remote.Arrivals, b)
	s = l.Summarize(300, remote)
	if *s.MissingAtReceiver != 1 {
		t.Fatal("missing receiver datum not counted")
	}
	remote.Complete = false
	s = l.Summarize(300, remote)
	if s.MissingAtReceiver != nil {
		t.Fatal("partial remote evidence treated as complete")
	}
}

// TestCutoffAndLate preserves equality-at-deadline and strict observation cutoffs.
func TestCutoffAndLate(t *testing.T) {
	l, _ := NewLedger(4)
	for i := 0; i < 3; i++ {
		id := Identity{Lifecycle: 2, Sequence: profile.Decimal(i)}
		if err := l.Schedule(id, uint64(i*100), uint64(i*100+50)); err != nil {
			t.Fatal(err)
		}
		l.Attempt(id, uint64(i*100))
		l.Written(id, uint64(i*100+1), 124, true)
		l.ReceiveReceipt(id, uint64(i*100), uint64(i*100+50+i))
	}
	s := l.Summarize(252, nil)
	if s.OnTime != 1 || s.Late != 1 || s.Unresolved != 1 || s.RTTSamples != 2 {
		t.Fatalf("cutoff %+v", s)
	}
	s = l.Summarize(253, nil)
	if s.Late != 2 || *s.RTTP99 != "52" {
		t.Fatal("late sample/cutoff mismatch")
	}
	snapshot := l.Snapshot()
	*snapshot[0].Receipt = 999
	if *l.Snapshot()[0].Receipt != 50 {
		t.Fatal("snapshot mutated ledger")
	}
}

// TestTransitionsAndPartial checks bounded evidence and partial-write unknown outcomes.
func TestTransitionsAndPartial(t *testing.T) {
	l, _ := NewLedger(1)
	id := Identity{Lifecycle: 2}
	if err := l.Schedule(id, 10, 20); err != nil {
		t.Fatal(err)
	}
	if err := l.Schedule(Identity{Lifecycle: 2, Sequence: 1}, 11, 21); err == nil {
		t.Fatal("ledger cap ignored")
	}
	if err := l.ReceiveReceipt(id, 10, 12); err == nil {
		t.Fatal("receipt for unsent slot accepted")
	}
	l.Attempt(id, 10)
	l.Written(id, 11, 3, false)
	if err := l.Skip(id, "queue"); err == nil {
		t.Fatal("attempt relabeled as skip")
	}
	s := l.Summarize(30, nil)
	if s.WriteFailed != 1 || s.Written != 0 || s.Unresolved != 1 {
		t.Fatal("partial write relabeled")
	}
}

// TestPreflightCancellation proves a cancelled local diagnostic cannot wait for its full duration.
func TestPreflightCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Preflight(ctx, 1e9, 1e8); err != context.Canceled {
		t.Fatalf("cancellation %v", err)
	}
}

// TestWarmupDuplicatesAndVariation keeps lifecycle metrics, cutoff revisions and sequence gaps honest.
func TestWarmupDuplicatesAndVariation(t *testing.T) {
	l, _ := NewLedger(20)
	for i := 0; i < 5; i++ {
		id := Identity{Lifecycle: 2, Sequence: profile.Decimal(i)}
		if i == 0 {
			id.Lifecycle = 1
		}
		if err := l.Schedule(id, uint64(i*100), uint64(i*100+80)); err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			l.Skip(id, "scheduler lag")
			continue
		}
		l.Attempt(id, uint64(i*100))
		l.Written(id, uint64(i*100+1), 124, true)
		l.ReceiveReceipt(id, uint64(i*100), uint64(i*100+10+i))
		if i == 1 {
			l.ReceiveReceipt(id, 100, 500)
		}
	}
	s := l.Summarize(450, nil)
	if s.Planned != 4 || s.RTTSamples != 3 || s.DuplicateReceipts != 0 || s.VariationGaps != 1 || len(s.RTTVariation) != 1 {
		t.Fatalf("mixed lifecycle/cutoff/gap %+v", s)
	}
	s = l.Summarize(501, nil)
	if s.DuplicateReceipts != 1 {
		t.Fatal("eligible duplicate missing")
	}
	warmup := l.SummarizePhase(450, nil, 1, nil)
	if warmup.Planned != 1 || warmup.RTTSamples != 1 {
		t.Fatal("warmup missing separate metrics")
	}
	if err := l.Schedule(Identity{Lifecycle: 2, Phase: 1, Sequence: 4}, 500, 580); err == nil {
		t.Fatal("sequence reused across phases")
	}
}
