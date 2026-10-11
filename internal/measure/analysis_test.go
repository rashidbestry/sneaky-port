package measure

import (
	"reflect"
	"testing"

	"sneaky-port/internal/profile"
)

// measured adds a deterministic accepted/attempted datum with an optional first receipt.
func measured(t *testing.T, l *Ledger, seq uint64, phase uint32, scheduled, deadline, rtt uint64, receipt bool) Identity {
	t.Helper()
	id := Identity{Lifecycle: 2, Phase: phase, Sequence: profile.Decimal(seq)}
	if err := l.Schedule(id, scheduled, deadline); err != nil {
		t.Fatal(err)
	}
	if err := l.Admit(id, scheduled, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Attempt(id, scheduled); err != nil {
		t.Fatal(err)
	}
	if err := l.Written(id, scheduled+1, 124, true); err != nil {
		t.Fatal(err)
	}
	if receipt {
		if err := l.ReceiveReceipt(id, scheduled, scheduled+rtt); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// TestSpikeGroupingAndCorrection exercises threshold equality, sequence/phase gaps and impossible correction.
func TestSpikeGroupingAndCorrection(t *testing.T) {
	l, _ := NewLedger(30)
	for i, rtt := range []uint64{10, 20, 30, 0, 40, 50, 60} {
		phase := uint32(0)
		if i == 6 {
			phase = 1
		}
		if i == 3 {
			id := Identity{Lifecycle: 2, Phase: phase, Sequence: profile.Decimal(i)}
			l.Schedule(id, uint64(i*100), uint64(i*100+15))
			l.Skip(id, "scheduler_lag")
			continue
		}
		id := measured(t, l, uint64(i), phase, uint64(i*100), uint64(i*100+15), rtt, true)
		if i == 1 {
			if err := l.ReceiptProcessing(id, 100, 105); err != nil {
				t.Fatal(err)
			}
		}
		if i == 2 {
			if err := l.ReceiptProcessing(id, 100, 140); err != nil {
				t.Fatal(err)
			}
		}
	}
	a := l.Analyze(1000, 10, 2, nil)
	if len(a.Spikes) != 3 || a.Spikes[0].Count != 2 || a.Spikes[0].First.Sequence != 1 || a.Spikes[0].Last.Sequence != 2 || a.Spikes[0].Duration != 100 || a.Spikes[0].Peak != 30 || len(a.Spikes[0].AdjacentFailures) != 1 || a.Spikes[1].Count != 2 || a.Spikes[2].First.Phase != 1 || *a.BaselineMedian != "30" {
		t.Fatalf("spikes %+v", a)
	}
	s := l.Summarize(1000, nil)
	if s.RTTSamples != 6 || s.CorrectedSamples != 1 || s.InvalidCorrections != 1 || *s.CorrectedP95 != "15" || len(s.RTTVariation) != 3 || s.VariationGaps != 1 || *s.RTTMin != "10" || *s.RTTMax != "60" {
		t.Fatalf("metrics %+v", s)
	}
}

// TestCompletionStalls excludes quiet without overdue work and splits on any valid completion.
func TestCompletionStalls(t *testing.T) {
	l, _ := NewLedger(20)
	measured(t, l, 0, 0, 0, 10, 10, true) // Equality at deadline is on time, not an episode.
	measured(t, l, 1, 0, 20, 30, 30, true)
	measured(t, l, 2, 0, 25, 35, 55, true)
	id := Identity{Lifecycle: 2, Sequence: 3}
	l.Schedule(id, 90, 100)
	l.Skip(id, "queue")
	measured(t, l, 4, 1, 150, 160, 0, false)
	a := l.Analyze(200, 20, 2, nil)
	expected := []Stall{{30, 50, 20}, {50, 80, 30}, {160, 200, 40}}
	if !reflect.DeepEqual(a.Stalls, expected) {
		t.Fatalf("stalls %+v want %+v", a.Stalls, expected)
	}
	if l.Summarize(160, nil).Pending != 1 || l.Summarize(161, nil).Pending != 0 {
		t.Fatal("strict overdue boundary")
	}
}

// TestCutoffDoesNotRewriteAttemptsOrTickets preserves post-cutoff writes/receipts/local resolution as evidence.
func TestCutoffDoesNotRewriteAttemptsOrTickets(t *testing.T) {
	l, _ := NewLedger(8)
	id := Identity{Lifecycle: 2}
	l.Schedule(id, 0, 10)
	l.Admit(id, 0, "accepted", "")
	l.Attempt(id, 20)
	l.Written(id, 25, 124, true)
	l.ReceiveReceipt(id, 20, 30)
	s := l.Summarize(15, nil)
	if s.Attempted != 0 || s.Written != 0 || s.RTTSamples != 0 || s.DeadlineMisses != 1 {
		t.Fatalf("post-cutoff events rewrote window %+v", s)
	}
	id.Sequence = 1
	l.Schedule(id, 40, 50)
	l.Admit(id, 40, "accepted", "")
	l.Unattempted(id, 100, "canceled")
	s = l.Summarize(80, nil)
	if s.Skipped != 0 || s.Unresolved != 1 || s.Accepted != 2 {
		t.Fatalf("post-cutoff ticket resolution %+v", s)
	}
	s = l.Summarize(101, nil)
	if s.Skipped != 1 || s.LocalMisses != 1 {
		t.Fatal("observed local cancellation absent")
	}
	groups := l.SummarizePhases(101, nil, 2)
	if groups[0].Planned != 2 || groups[1].Planned != 0 {
		t.Fatal("phase partition")
	}
}
