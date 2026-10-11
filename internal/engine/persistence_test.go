package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"sneaky-port/internal/testutil"
)

// failedPersistence injects failures at admission, asynchronous disk notification or final publication.
// Its retained result is inspected only after finite ownership joins, without a real disk dependency.
type failedPersistence struct {
	failed                 chan struct{}
	admission, publication bool
	final                  Result
}

// Append models bounded storage admission; it never changes measurement observations.
func (p *failedPersistence) Append(json.RawMessage) error {
	if p.admission {
		return errors.New("injected evidence admission exhaustion")
	}
	return nil
}

// Failed models a disk worker notification that can arrive independently of workload events.
func (p *failedPersistence) Failed() <-chan struct{} { return p.failed }

// Finalize retains joined cutoffs and models a failure to publish a final summary.
func (p *failedPersistence) Finalize(r Result) error {
	p.final = r
	if p.publication {
		return errors.New("injected summary publication failure")
	}
	return nil
}

// TestFinitePersistenceFailures makes admission/disk/publication failures partial and wakes quiet runs.
// This uses authentic loopback UDP/TCP sockets; injected storage is not an OS impairment claim.
func TestFinitePersistenceFailures(t *testing.T) {
	for _, carrier := range []string{"udp", "tcp"} {
		for _, kind := range []string{"admission", "quiet_disk", "publication"} {
			t.Run(carrier+"/"+kind, func(t *testing.T) { // Failure must join workers and retain endpoint cutoffs, including no-traffic windows.
				p := testutil.Plan()
				p.Adapter = carrier
				if carrier == "tcp" {
					p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
				}
				p.Phases[0].Duration = 50e6
				p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 20, 20
				if kind == "quiet_disk" {
					p.Phases[0].Duration = 2e9
					p.Phases[0].Client.Mode, p.Phases[0].Server.Mode = "quiet", "quiet"
					p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 0, 0
				}
				c, _, p := realPair(t, p)
				sink := &failedPersistence{failed: make(chan struct{}), admission: kind == "admission", publication: kind == "publication"}
				if kind == "quiet_disk" {
					close(sink.failed)
				}
				finite, err := NewFiniteWithEvidence(context.Background(), c, p, false, sink)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				r, err := finite.Wait(ctx)
				if err == nil || r.Complete || r.Receiver.Complete {
					finite.Cancel()
					t.Fatal("storage failure hidden or quiet collector did not wake")
				}
				if sink.final.ObservationCutoff != r.ObservationCutoff {
					t.Fatal("storage finalized before worker cutoff")
				}
				if sink.final.GenerationCutoff == 0 || sink.final.ObservationCutoff == 0 {
					t.Fatal("storage failure discarded the joined worker cutoff events")
				}
				if kind == "publication" && !sink.final.Complete {
					t.Fatal("publication fixture never reached a complete local cutoff")
				}
			})
		}
	}
}
