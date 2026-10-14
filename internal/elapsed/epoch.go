// Package elapsed owns one checked host-local clock domain for a finite measurement.
package elapsed

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"sync"
	"time"
)

// Metadata identifies the actual source and tick quantum without claiming clock accuracy.
type Metadata struct {
	Source                string `json:"source"`
	FrequencyHz           string `json:"frequency_hz,omitempty"`
	TickNumeratorNS       string `json:"tick_ns_numerator,omitempty"`
	TickDenominator       string `json:"tick_ns_denominator,omitempty"`
	Quantization          string `json:"quantization"`
	Calibrated            bool   `json:"calibrated"`
	IndependentlyVerified bool   `json:"independently_verified"`
}

// Epoch serializes capture and order checks; a failed source never resumes or changes clock.
// Last valid elapsed time is retained only for partial-stop evidence after a failure.
type Epoch struct {
	mu       sync.Mutex
	read     func() (uint64, error)
	last     uint64
	err      error
	failed   chan struct{}
	metadata Metadata
}

// New captures the platform source's zero exactly once, before any run worker starts.
func New() (*Epoch, error) { return newPlatform() }

// FromTime intentionally preserves earlier component fixtures' Go monotonic time domain.
// Production finite runs use New; this compatibility path never backprojects QPC onto Go.
func FromTime(anchor time.Time) (*Epoch, error) {
	if anchor.IsZero() || anchor.After(time.Now()) {
		return nil, errors.New("invalid legacy elapsed anchor")
	}
	return &Epoch{failed: make(chan struct{}), metadata: Metadata{Source: "Go time.Since legacy monotonic anchor", Quantization: "runtime/platform dependent; unverified"}, read: func() (uint64, error) { // Preserve the supplied Go epoch rather than creating a new measurement origin.
		d := time.Since(anchor)
		if d < 0 {
			return 0, errors.New("elapsed clock moved before epoch")
		}
		return uint64(d), nil
	}}, nil
}

// Now captures one timestamp in the selected domain and permanently latches any failure.
func (e *Epoch) Now() (uint64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err != nil {
		return e.last, e.err
	}
	if e.failed == nil {
		e.failed = make(chan struct{})
	}
	if e.read == nil {
		e.err = errors.New("uninitialized elapsed clock")
		close(e.failed)
		return e.last, e.err
	}
	n, err := e.read()
	if err == nil && n < e.last {
		err = errors.New("elapsed clock moved backwards")
	}
	if err == nil && n > math.MaxInt64 {
		err = errors.New("elapsed clock exceeds nanosecond range")
	}
	if err != nil {
		e.err = fmt.Errorf("%s: %w", e.metadata.Source, err)
		close(e.failed)
		return e.last, e.err
	}
	e.last = n
	return n, nil
}

// Err reports a sticky capture failure without attempting another source or capture.
func (e *Epoch) Err() error { e.mu.Lock(); defer e.mu.Unlock(); return e.err }

// Failed interrupts every run owner when any shared-domain capture permanently fails.
func (e *Epoch) Failed() <-chan struct{} { e.mu.Lock(); defer e.mu.Unlock(); return e.failed }

// Metadata returns immutable provenance; frequency and quantization are not error bounds.
func (e *Epoch) Metadata() Metadata { return e.metadata }

// ticksNS converts relative ticks with a wide intermediate and floors fractional nanoseconds.
// The final signed duration bound is checked even when the intermediate product exceeds 64 bits.
func ticksNS(ticks, frequency uint64) (uint64, error) {
	if frequency == 0 {
		return 0, errors.New("zero counter frequency")
	}
	hi, lo := bits.Mul64(ticks, 1e9)
	if hi >= frequency {
		return 0, errors.New("elapsed counter conversion overflow")
	}
	n, _ := bits.Div64(hi, lo, frequency)
	if n > math.MaxInt64 {
		return 0, errors.New("elapsed counter exceeds nanosecond range")
	}
	return n, nil
}

// counterEpoch creates a checked relative counter epoch with no wall-clock or Go bridge.
func counterEpoch(source string, frequency uint64, read func() (int64, error)) (*Epoch, error) {
	if frequency == 0 {
		return nil, errors.New("invalid counter frequency")
	}
	origin, err := read()
	if err != nil || origin < 0 {
		return nil, fmt.Errorf("elapsed counter origin unavailable: %w", errors.Join(err, errors.New("counter must be nonnegative")))
	}
	e := &Epoch{failed: make(chan struct{}), metadata: Metadata{Source: source, FrequencyHz: strconv.FormatUint(frequency, 10), TickNumeratorNS: "1000000000", TickDenominator: strconv.FormatUint(frequency, 10), Quantization: "reported counter tick; floor to integer nanoseconds; access/scheduling/oscillator bounds unverified"}}
	previous := origin
	e.read = func() (uint64, error) { // Read and validate counter order inside Epoch.Now's mutex, including thread migration.
		current, err := read()
		if err != nil {
			return 0, err
		}
		if current < previous || current < origin {
			return 0, errors.New("elapsed counter moved backwards")
		}
		previous = current
		return ticksNS(uint64(current-origin), frequency)
	}
	return e, nil
}
