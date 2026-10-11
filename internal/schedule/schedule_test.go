package schedule

import (
	"context"
	"testing"
	"time"
)

// TestRationalAndHalfOpen checks noninteger intervals and excludes the phase boundary.
func TestRationalAndHalfOpen(t *testing.T) {
	p := Pattern{Mode: "steady", Numerator: 3, Denominator: 1}
	n, err := p.Count(1e9)
	if err != nil || n != 3 {
		t.Fatalf("count %d %v", n, err)
	}
	for k, want := range []uint64{0, 333333333, 666666666} {
		got, err := p.Offset(uint64(k), 1e9)
		if err != nil || got != want {
			t.Fatalf("slot %d: %d %v", k, got, err)
		}
	}
	if _, err = p.Offset(3, 1e9); err == nil {
		t.Fatal("phase end included")
	}
	if n, err = p.Count(333333333); err != nil || n != 1 {
		t.Fatal("rounded offset incorrectly included at rational boundary")
	}
}

// TestBurstAndQuiet checks truncated active windows, gaps and quiet denominators.
func TestBurstAndQuiet(t *testing.T) {
	p := Pattern{Mode: "burst", Numerator: 10, Denominator: 1, Active: 250e6, Gap: 250e6}
	n, err := p.Count(610e6)
	if err != nil || n != 5 {
		t.Fatalf("burst count %d %v", n, err)
	}
	for i, want := range []uint64{0, 100e6, 200e6, 500e6, 600e6} {
		got, err := p.Offset(uint64(i), 610e6)
		if err != nil || got != want {
			t.Fatalf("burst %d %d %v", i, got, err)
		}
	}
	q := Pattern{Mode: "quiet", Denominator: 1}
	if n, err = q.Count(1e9); err != nil || n != 0 {
		t.Fatal("quiet creates data")
	}
}

// TestDelayedWake forbids catch-up bursts while admitting the latest slot within allowance.
func TestDelayedWake(t *testing.T) {
	p := Pattern{Mode: "steady", Numerator: 10, Denominator: 1}
	end, send, err := p.Due(0, 1e9, 305e6, 10e6)
	if err != nil || end != 4 || !send {
		t.Fatalf("wake %d %t %v", end, send, err)
	}
	end, send, err = p.Due(0, 1e9, 320e6, 10e6)
	if err != nil || end != 4 || send {
		t.Fatal("stale latest slot admitted")
	}
	end, send, err = p.Due(0, 1e9, 1e9, 1e9)
	if err != nil || end != 10 || send {
		t.Fatal("send past phase boundary")
	}
}

// TestLimitsAndCancellation checks exact overflow rejection and interruptible waiting.
func TestLimitsAndCancellation(t *testing.T) {
	for _, p := range []Pattern{{Mode: "steady", Numerator: 1000000001, Denominator: 1}, {Mode: "burst", Numerator: 1, Denominator: 1, Active: 1<<63 - 1, Gap: 1}} {
		if _, err := p.Count(1e9); err == nil {
			t.Fatal("invalid pattern accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Wait(ctx.Done(), time.Now(), 1e9) {
		t.Fatal("canceled wait completed")
	}
}
