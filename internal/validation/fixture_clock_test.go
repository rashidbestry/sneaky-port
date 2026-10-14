package validation

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"sneaky-port/internal/probe"
)

// frozenFaultClock separates the fixture's selected source from real timer wakes;
// each capture publishes a sequence so assertions wait for state rather than wall delays.
type frozenFaultClock struct {
	mu       sync.Mutex
	now      uint64
	captures int
	observed chan int
	failAt   int
	err      error
}

// Now records each selected-source capture, retaining the injected terminal error
// after its chosen boundary; operating-system time cannot advance this source.
func (c *frozenFaultClock) Now() (uint64, error) {
	c.mu.Lock()
	c.captures++
	capture, now := c.captures, c.now
	var err error
	if c.failAt != 0 && capture >= c.failAt {
		err = c.err
	}
	c.mu.Unlock()
	select {
	case c.observed <- capture:
	default:
	}
	return now, err
}

// advance moves only the selected source, keeping concurrent dispatcher captures race-safe.
func (c *frozenFaultClock) advance(now uint64) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

// fixtureDispatcher runs only the bounded release loop with public fixture framing;
// authentic sockets and product authentication remain covered by the controlled runs.
func fixtureDispatcher(t *testing.T, clock faultClock) (*faultTransport, []byte) {
	t.Helper()
	w := &faultTransport{scenario: newScenario("delay", clock), role: "server", stop: make(chan struct{}), output: make(chan receiveResult, 1)}
	input := make(chan receiveResult, 1)
	w.wg.Add(1)
	go w.dispatch(input)
	t.Cleanup(func() { // Stop and join the fixture even if an assertion or watchdog fails.
		close(w.stop)
		w.wg.Wait()
	})
	datum := make([]byte, probe.Overhead)
	copy(datum, "SPV1")
	datum[6], datum[7], datum[8] = probe.Data, 1, 2
	input <- receiveResult{bytes: datum}
	return w, datum
}

// awaitFixtureCapture uses a watchdog only to bound a broken loop; the selected
// source's capture sequence establishes that a real timer wake was rechecked.
func awaitFixtureCapture(t *testing.T, clock *frozenFaultClock, want int) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case capture := <-clock.observed:
			if capture >= want {
				return
			}
		case <-timer.C:
			t.Fatalf("fixture never reached selected-source capture %d", want)
		}
	}
}

// TestFixtureTimerCannotAdvanceSelectedClock forces an operating-system timer
// to wake while the selected source remains frozen, then releases exactly at its floor.
func TestFixtureTimerCannotAdvanceSelectedClock(t *testing.T) {
	clock := &frozenFaultClock{now: 1000, observed: make(chan int, 64)}
	w, datum := fixtureDispatcher(t, clock)
	// Capture 1 sets the release floor, 2 sleeps, 3 rejects an early timer wake,
	// and 4 starts the next sleep. Waiting for 4 proves the rejection completed.
	awaitFixtureCapture(t, clock, 4)
	select {
	case got := <-w.output:
		t.Fatalf("OS timer released before selected-source floor: %+v", got)
	default:
	}
	clock.advance(1000 + uint64(9*time.Millisecond))
	select {
	case got := <-w.output:
		if got.err != nil || !bytes.Equal(got.bytes, datum) {
			t.Fatalf("selected-source release changed datum: %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("selected-source floor did not release datum")
	}
}

// TestFixtureClockFailureStopsRelease checks source failures at receive capture,
// timer setup and timer wake; no datum may escape when its floor is unproven.
func TestFixtureClockFailureStopsRelease(t *testing.T) {
	for _, boundary := range []struct {
		name    string
		capture int
	}{{"receive", 1}, {"sleep", 2}, {"wake", 3}} {
		t.Run(boundary.name, func(t *testing.T) { // Each fresh loop fails at one timing boundary.
			cause := errors.New("selected fixture clock failed")
			clock := &frozenFaultClock{observed: make(chan int, 64), failAt: boundary.capture, err: cause}
			w, _ := fixtureDispatcher(t, clock)
			select {
			case got := <-w.output:
				if !errors.Is(got.err, cause) || len(got.bytes) != 0 {
					t.Fatalf("selected-source error lost or datum released: %+v", got)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("selected-source failure did not reach receive owner")
			}
		})
	}
}
