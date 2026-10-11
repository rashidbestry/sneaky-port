package engine

import (
	"errors"

	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/worker"
	"sneaky-port/internal/writer"
)

// ReplayRecord applies saved observations through the live accounting transitions without sockets or tickets.
// Completion records resolve accepted unsent data only; receipt tickets never create local data.
func (c *Collector) ReplayRecord(r Record) error {
	if err := c.keep(r); err != nil {
		return err
	}
	if r.Kind == "ticket_observed" {
		if r.FrameKind == probe.Data && !r.CallbackInvoked {
			return c.ledger.Unattempted(r.Identity, uint64(r.At), "canceled_unattempted: "+r.Reason)
		}
		return nil
	}
	e := worker.Event{Kind: r.Kind, At: uint64(r.At), Scheduled: uint64(r.Scheduled), Call: uint64(r.Call), Count: uint64(r.Count), Bytes: int(r.Bytes), Reason: r.Reason, Marking: r.Marking, MarkLifecycle: r.MarkLifecycle, MarkPhase: r.MarkPhase,
		Frame: probe.Frame{Kind: r.FrameKind, Direction: r.Direction, Lifecycle: r.Identity.Lifecycle, Phase: r.Identity.Phase, Sequence: uint64(r.Identity.Sequence), OriginEpoch: r.Epoch, PeerEpoch: r.PeerEpoch, Attempt: uint64(r.Attempt), Arrival: uint64(r.PeerArrival), ReceiptAttempt: uint64(r.PeerAttempt)}, Admission: writer.Admission{State: writer.AdmissionState(r.Admission)}}
	if r.Error != "" {
		e.Err = errors.New(r.Error)
	}
	return c.apply(e, false)
}

// ReplayFinish uses the declared final observation cutoff, preserving partial status independently of samples.
// A missing cutoff must use a conservative observed prefix bound supplied by the report reader.
func (c *Collector) ReplayFinish(generation, observation profile.Decimal, complete bool) Result {
	c.generation, c.observation = uint64(generation), uint64(observation)
	var failure error
	if !complete {
		failure = errors.New("partial local evidence")
	}
	r, _ := c.Finish(failure, uint64(observation))
	return r
}
