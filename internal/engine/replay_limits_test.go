package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"sneaky-port/internal/probe"
	"sneaky-port/internal/testutil"
	"sneaky-port/internal/worker"
	"sneaky-port/internal/writer"
)

// deniedCompletionEvidence fails only final completion admission, modeling a exhausted durable raw budget.
type deniedCompletionEvidence struct{}

// Append preserves normal records but refuses a completion before it can mutate the replayable ledger.
func (deniedCompletionEvidence) Append(b json.RawMessage) error {
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	if r.Kind == "ticket_observed" {
		return errors.New("injected completion storage exhaustion")
	}
	return nil
}

// Failed has no asynchronous failure; this fixture tests the nonblocking admission boundary only.
func (deniedCompletionEvidence) Failed() <-chan struct{} { return nil }

// Finalize does not publish files; control fixtures cover real disk publication separately.
func (deniedCompletionEvidence) Finalize(Result) error { return nil }

// TestReplayUnsavedCompletion cannot turn a missing raw ticket into a locally resolved final outcome.
func TestReplayUnsavedCompletion(t *testing.T) {
	p := testutil.Plan()
	c, err := NewCollector(p, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	c.persistence = deniedCompletionEvidence{}
	f := probe.Frame{Kind: probe.Data, Direction: 1, Lifecycle: 2, Sequence: 0}
	apply(t, c, worker.Event{Kind: "admission", Frame: f, At: 1, Scheduled: 0, Count: 1, Admission: writer.Admission{State: writer.Accepted, Done: ticketResult(writer.Result{Reason: writer.Canceled, Err: context.Canceled})}})
	apply(t, c, worker.Event{Kind: "generation_end", At: 10})
	apply(t, c, worker.Event{Kind: "observation_end", At: 20})
	r, err := c.Finish(nil, 21)
	if err == nil || r.Complete || r.Entries[0].LocalResolved != nil || r.Entries[0].State != "planned" {
		t.Fatal("unsaved completion changed final ledger")
	}
	assertReplayAccounting(t, p, 0, r)
}
