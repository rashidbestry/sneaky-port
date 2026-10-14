package localapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"sneaky-port/internal/control"
	"sneaky-port/internal/measure"
)

// Comparison replay has five seconds of response headroom after its cooperative work deadline.
const comparisonWorkBudget = 25 * time.Second
const comparisonResponseBudget = 30 * time.Second

// compareInput binds explicit saved selections to the existing strict versioned request envelope.
type compareInput struct {
	Version        int                     `json:"schema_version"`
	RequestID      string                  `json:"request_id"`
	Pairs          []control.TrialPair     `json:"pairs"`
	Axes           []string                `json:"axes"`
	Endpoint       string                  `json:"endpoint"`
	Scope          string                  `json:"scope"`
	EvidencePolicy *measure.EvidencePolicy `json:"evidence_policy,omitempty"`
}

// compareReports performs read-only comparison under the existing protected serialized saved-operation guard.
func (s *Server) compareReports(w http.ResponseWriter, r *http.Request, id string) {
	var in compareInput
	if !savedBody(w, r, id, &in) {
		return
	}
	q := control.ComparisonRequest{Pairs: in.Pairs, Axes: in.Axes, Endpoint: in.Endpoint, Scope: in.Scope, EvidencePolicy: in.EvidencePolicy}
	if in.Version != 1 || in.RequestID != id || q.Validate() != nil {
		reject(w, 400, id, "invalid_request", "selection", "explicit valid pairs, axes, endpoint, scope and request identity required")
		return
	}
	s.replyComparison(w, r, id, q, s.store.CompareSavedReports)
}

// replyComparison gives admitted multi-report replay its own finite response and cooperative work budget.
// Cancellation is checked between bounded replays; filesystem calls are not forcibly interruptible.
// The private backend argument keeps budget ownership separate from replay and permits controlled boundary tests.
func (s *Server) replyComparison(w http.ResponseWriter, r *http.Request, id string, q control.ComparisonRequest, compare func(context.Context, control.ComparisonRequest) (control.Comparison, error)) {
	ctx, cancel := context.WithTimeout(r.Context(), comparisonWorkBudget)
	defer cancel()
	deadline := time.Now().Add(comparisonResponseBudget)
	if parentDeadline, ok := r.Context().Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	if err := http.NewResponseController(w).SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		reject(w, 503, id, "resource_limit", "comparison", "saved comparison response deadline unavailable")
		return
	}
	out, err := compare(ctx, q)
	if err == nil {
		err = ctx.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		// An expired/disconnected response may not be writable; no partial comparison is published.
		reject(w, 503, id, "resource_limit", "comparison", "saved comparison interrupted or exceeded its work budget")
		return
	}
	s.diskReply(w, id, out, err)
}
