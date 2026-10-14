package elapsed

import (
	"errors"
	"math"
	"math/big"
	"sync"
	"testing"
	"time"
)

// TestCounterConversion compares exact integer conversion with an independent big-integer oracle.
// It covers long 10MHz intervals whose tick*1e9 intermediate overflows 64 bits.
func TestCounterConversion(t *testing.T) {
	for _, frequency := range []uint64{1, 3, 10_000_000, math.MaxInt64, math.MaxUint64} {
		for _, ticks := range []uint64{0, 1, 2, 9_223_372_037, 10_000_000_000, math.MaxInt64, math.MaxUint64} {
			want := new(big.Int).Mul(new(big.Int).SetUint64(ticks), big.NewInt(1e9))
			want.Div(want, new(big.Int).SetUint64(frequency))
			got, err := ticksNS(ticks, frequency)
			if want.Cmp(big.NewInt(math.MaxInt64)) > 0 {
				if err == nil {
					t.Fatalf("overflow accepted ticks=%d hz=%d", ticks, frequency)
				}
				continue
			}
			if err != nil || got != want.Uint64() {
				t.Fatalf("ticks=%d hz=%d got=%d err=%v want=%s", ticks, frequency, got, err, want)
			}
		}
	}
	if _, err := ticksNS(1, 0); err == nil {
		t.Fatal("zero frequency accepted")
	}
}

// TestCounterFailureIsPermanent retains the last valid boundary and never rereads after failure.
func TestCounterFailureIsPermanent(t *testing.T) {
	for _, kind := range []string{"read", "backwards", "overflow"} {
		t.Run(kind, func(t *testing.T) { // Each source starts valid, then faults and would recover if reread.
			calls := 0
			read := func() (int64, error) { // Inject deterministic counter state at the actual source boundary.
				calls++
				switch calls {
				case 1:
					return 100, nil
				case 2:
					return 110, nil
				case 3:
					if kind == "read" {
						return 0, errors.New("read failed")
					}
					if kind == "backwards" {
						return 105, nil
					}
					return math.MaxInt64, nil
				default:
					return 120, nil
				}
			}
			e, err := counterEpoch("fixture counter", 1, read)
			if err != nil {
				t.Fatal(err)
			}
			last, err := e.Now()
			if err != nil || last != 10e9 {
				t.Fatal(last, err)
			}
			failed, err := e.Now()
			if err == nil || failed != last {
				t.Fatal("missing sticky fault", failed, err)
			}
			select {
			case <-e.Failed():
			default:
				t.Fatal("failure not published")
			}
			for i := 0; i < 3; i++ {
				now, nextErr := e.Now()
				if now != last || nextErr != err {
					t.Fatal("failed source resumed")
				}
			}
			if calls != 3 {
				t.Fatal("reread after failure", calls)
			}
		})
	}
}

// TestConcurrentCounterCapture requires serialization of reading and checking, not just updates.
func TestConcurrentCounterCapture(t *testing.T) {
	var ticks int64
	e, err := counterEpoch("serialized fixture", 1e9, func() (int64, error) { // This deliberately unsynchronized reader is safe only inside the epoch lock.
		ticks++
		return ticks, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Go(func() { // Concurrent data/control-like readers retain per-caller order without false backwards faults.
			var previous uint64
			for j := 0; j < 1000; j++ {
				now, err := e.Now()
				if err != nil || now < previous {
					t.Error("capture order", now, previous, err)
					return
				}
				previous = now
			}
		})
	}
	wg.Wait()
	last, err := e.Now()
	if err != nil || last != 16001 {
		t.Fatal(last, err)
	}
}

// TestNativeEpoch observes selected-source progress without claiming independent clock accuracy.
func TestNativeEpoch(t *testing.T) {
	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	a, err := e.Now()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	b, err := e.Now()
	if err != nil || b <= a {
		t.Fatal("native elapsed clock did not advance", a, b, err)
	}
	m := e.Metadata()
	if m.Source == "" || m.Quantization == "" || m.Calibrated || m.IndependentlyVerified {
		t.Fatal("unsupported source claims", m)
	}
	t.Logf("selected native elapsed source: %+v; observed interval %d ns; no accuracy bound", m, b-a)
	var zero Epoch
	if _, err := zero.Now(); err == nil {
		t.Fatal("uninitialized source accepted")
	}
	if _, err := FromTime(time.Now().Add(time.Hour)); err == nil {
		t.Fatal("future legacy epoch accepted")
	}
}
