package localapi

import (
	"net/http"
	"sneaky-port/internal/control"
)

// compareInput binds explicit saved selections to the existing strict versioned request envelope.
type compareInput struct {
	Version   int                 `json:"schema_version"`
	RequestID string              `json:"request_id"`
	Pairs     []control.TrialPair `json:"pairs"`
	Axes      []string            `json:"axes"`
	Endpoint  string              `json:"endpoint"`
	Scope     string              `json:"scope"`
}

// compareReports performs read-only comparison under the existing protected serialized saved-operation guard.
func (s *Server) compareReports(w http.ResponseWriter, r *http.Request, id string) {
	var in compareInput
	if !savedBody(w, r, id, &in) {
		return
	}
	q := control.ComparisonRequest{Pairs: in.Pairs, Axes: in.Axes, Endpoint: in.Endpoint, Scope: in.Scope}
	if in.Version != 1 || in.RequestID != id || q.Validate() != nil {
		reject(w, 400, id, "invalid_request", "selection", "explicit valid pairs, axes, endpoint, scope and request identity required")
		return
	}
	out, err := s.store.CompareSavedReports(r.Context(), q)
	s.diskReply(w, id, out, err)
}
