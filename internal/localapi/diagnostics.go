package localapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"sneaky-port/internal/control"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
)

const plotGroups = 128

// plotRange preserves exact extrema and population; nil extrema mean no completed samples.
type plotRange struct {
	Count profile.Decimal  `json:"count"`
	Min   *profile.Decimal `json:"min_ns"`
	Max   *profile.Decimal `json:"max_ns"`
}

// timeGroup groups consecutive reached slots, keeping missing/local outcomes separate from RTT.
type timeGroup struct {
	Start     profile.Decimal `json:"start_ns"`
	End       profile.Decimal `json:"end_ns"`
	Slots     profile.Decimal `json:"slots"`
	Missing   profile.Decimal `json:"missing"`
	Local     profile.Decimal `json:"local"`
	RTT       plotRange       `json:"rtt"`
	Scheduled plotRange       `json:"scheduled"`
}

// ecdfGroup retains exact inclusive rank ranges and extrema of sorted full-sample populations.
type ecdfGroup struct {
	plotRange
	First profile.Decimal `json:"first_rank"`
	Last  profile.Decimal `json:"last_rank"`
}

// phaseView labels half-open host-local phase boundaries without implying synchronized clocks.
type phaseView struct {
	ID    string          `json:"phase_id"`
	Start profile.Decimal `json:"start_ns"`
	End   profile.Decimal `json:"end_ns"`
}

// diagnosticView sends bounded rendering inputs, exact scalar summaries and explicit evidence provenance.
// Variation samples remain in raw reports; this view never replaces replay/export evidence.
type diagnosticView struct {
	ReportID       string                           `json:"report_id"`
	Endpoint       string                           `json:"endpoint"`
	Scope          string                           `json:"scope"`
	Status         string                           `json:"status"`
	Reason         string                           `json:"reason"`
	Integrity      string                           `json:"integrity"`
	Complete       bool                             `json:"complete"`
	Cutoff         profile.Decimal                  `json:"cutoff_ns"`
	Summary        map[string]any                   `json:"summary"`
	Phases         []phaseView                      `json:"phases"`
	Time           []timeGroup                      `json:"time"`
	RTT            []ecdfGroup                      `json:"rtt_ecdf"`
	Scheduled      []ecdfGroup                      `json:"scheduled_ecdf"`
	Label          string                           `json:"plot_policy"`
	EvidencePolicy measure.EvidencePolicy           `json:"evidence_policy"`
	Labels         map[string]measure.EvidenceLabel `json:"evidence_labels"`
	ContextLabels  map[string]measure.EvidenceLabel `json:"context_evidence_labels"`
}

// addRange updates exact integer extrema without converting samples to browser floating point.
func addRange(r *plotRange, v profile.Decimal) {
	if r.Min == nil || v < *r.Min {
		x := v
		r.Min = &x
	}
	if r.Max == nil || v > *r.Max {
		x := v
		r.Max = &x
	}
	r.Count++
}

// ecdf sorts all completed samples before rank grouping; final quantiles never use grouped data.
func ecdf(values []uint64) []ecdfGroup {
	sort.Slice(values, func(i, j int) bool { // Order exact uint64 samples, including duplicates.
		return values[i] < values[j]
	})
	out := []ecdfGroup{}
	size := max(1, (len(values)+plotGroups-1)/plotGroups)
	for i := 0; i < len(values); i += size {
		end := min(len(values), i+size)
		g := ecdfGroup{First: profile.Decimal(i + 1), Last: profile.Decimal(end)}
		for _, v := range values[i:end] {
			addRange(&g.plotRange, profile.Decimal(v))
		}
		out = append(out, g)
	}
	return out
}

// buildDiagnostic uses the replay ledger's strict cutoff and shared summary population, never remote clocks.
func buildDiagnostic(r *control.ReplayedReport, scope string, policies ...measure.EvidencePolicy) (diagnosticView, error) {
	v := diagnosticView{Endpoint: r.Manifest.Endpoint, Scope: scope, Status: r.Status, Reason: r.Reason, Integrity: r.Integrity,
		Complete: r.Evidence.Complete, Cutoff: r.Evidence.ObservationCutoff, Time: []timeGroup{}, Phases: []phaseView{},
		Label: "At most 128 consecutive-slot groups and 128 sorted-rank groups per metric; exact min/max/count and missing/local markers retained. SVG coordinates are approximate; tables and raw exports retain exact ns. Quantiles use all eligible completed samples. Missing = reached nonlocal slots without eligible receipt; local = skipped slots. Quiet/unreached slots are not missing. Endpoint-only; offline consistency is not authentication. Saved endpoint snapshot; run controls show separate provisional live counters."}
	if len(r.Manifest.Plan.Phases) > 64 {
		return v, errors.New("dashboard phase capacity exceeded")
	}
	lifecycle := uint8(2)
	var phase *uint32
	switch scope {
	case "main":
	case "warmup":
		lifecycle = 1
	default:
		n, err := strconv.ParseUint(scope, 10, 32)
		if err != nil || strconv.FormatUint(n, 10) != scope || n >= uint64(len(r.Manifest.Plan.Phases)) {
			return v, errors.New("invalid phase scope")
		}
		p := uint32(n)
		phase = &p
	}
	summary := measure.SummarizeSnapshot(r.Evidence.Entries, uint64(v.Cutoff), nil, lifecycle, phase)
	v.EvidencePolicy = measure.DefaultEvidencePolicy()
	if len(policies) > 0 {
		v.EvidencePolicy = policies[0]
	}
	if err := v.EvidencePolicy.Validate(); err != nil {
		return v, err
	}
	selected := *r
	selected.Reconciliation = nil
	selected.Summary.Local = &control.LocalRun{Summary: summary}
	v.Labels = control.ReportLabels(&selected, v.EvidencePolicy).Main
	v.ContextLabels = control.ReportContextLabels(r, v.EvidencePolicy)
	b, _ := json.Marshal(summary)
	// Decimal quantities are JSON strings; retain them and nulls in a scalar-only table.
	v.Summary = map[string]any{}
	json.Unmarshal(b, &v.Summary) // Decimal fields and quantiles are encoded strings; remaining fields are null/text.
	delete(v.Summary, "RTTVariation")
	variation := make([]uint64, len(summary.RTTVariation))
	for i, x := range summary.RTTVariation {
		variation[i] = uint64(x)
	}
	v.Summary["VariationSamples"] = profile.Decimal(len(variation))
	v.Summary["VariationP95"] = measure.Percentile(variation, 95)
	v.Summary["VariationP99"] = measure.Percentile(variation, 99)
	v.Summary["EndpointAccountingErrors"] = profile.Decimal(len(r.Accounting))
	v.Summary["EndpointDiscardedTailBytes"] = r.TailBytes
	faults := map[string]profile.Decimal{"EndpointReadErrors": 0, "EndpointInvalidFrames": 0, "EndpointMarkingErrors": 0}
	var observedRecords profile.Decimal
	for _, raw := range r.Evidence.Records {
		var record struct {
			Kind, Error string
			At          profile.Decimal
		}
		if json.Unmarshal(raw, &record) != nil || record.At >= v.Cutoff {
			continue
		}
		observedRecords++
		switch record.Kind {
		case "read_error":
			faults["EndpointReadErrors"]++
		case "invalid":
			faults["EndpointInvalidFrames"]++
		case "marking":
			if record.Error != "" {
				faults["EndpointMarkingErrors"]++
			}
		}
	}
	for key, count := range faults {
		v.Summary[key] = count
	}
	for _, name := range []string{"EndpointAccountingErrors", "EndpointDiscardedTailBytes", "EndpointReadErrors", "EndpointInvalidFrames", "EndpointMarkingErrors"} {
		label := measure.EvidenceLabel{Coverage: "complete", Basis: "locally measured", Eligible: observedRecords, Completed: observedRecords, Reasons: []string{}}
		if !r.Evidence.Complete || len(r.Accounting) > 0 || r.TailBytes > 0 {
			label.Coverage = "partial"
			label.Reasons = append(label.Reasons, "partial_observation")
		}
		v.Labels[name] = label
	}
	offset := r.Manifest.Plan.Warmup
	for _, ph := range r.Manifest.Plan.Phases {
		v.Phases = append(v.Phases, phaseView{ph.ID, offset, offset + ph.Duration})
		offset += ph.Duration
	}
	entries := []measure.Entry{}
	for _, e := range r.Evidence.Entries {
		if e.Scheduled < v.Cutoff && e.Identity.Lifecycle == lifecycle && (phase == nil || e.Identity.Phase == *phase) {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { // Keep time groups in original schedule order.
		return entries[i].Scheduled < entries[j].Scheduled
	})
	rtts, scheduled := []uint64{}, []uint64{}
	// Reserve one group per phase so no time envelope spans a quiet/phase boundary.
	capacity := plotGroups - len(v.Phases)
	size := max(1, (len(entries)+capacity-1)/capacity)
	for i := 0; i < len(entries); {
		end := min(len(entries), i+size)
		for j := i + 1; j < end; j++ {
			if entries[j].Identity.Phase != entries[i].Identity.Phase {
				end = j
				break
			}
		}
		g := timeGroup{Start: entries[i].Scheduled, End: entries[end-1].Scheduled, Slots: profile.Decimal(end - i)}
		for _, e := range entries[i:end] {
			local := e.State == "skipped_local" && (e.Admitted == nil || *e.Admitted < v.Cutoff) && (e.LocalResolved == nil || *e.LocalResolved < v.Cutoff)
			if local {
				g.Local++
				continue
			}
			if e.Attempt == nil || e.Receipt == nil || *e.Attempt >= v.Cutoff || *e.Receipt >= v.Cutoff {
				g.Missing++
				continue
			}
			rtt, elapsed := *e.Receipt-*e.Attempt, *e.Receipt-e.Scheduled
			addRange(&g.RTT, rtt)
			addRange(&g.Scheduled, elapsed)
			rtts = append(rtts, uint64(rtt))
			scheduled = append(scheduled, uint64(elapsed))
		}
		v.Time = append(v.Time, g)
		i = end
	}
	v.RTT, v.Scheduled = ecdf(rtts), ecdf(scheduled)
	if profile.Decimal(len(rtts)) != summary.RTTSamples {
		return v, errors.New("plot population mismatch")
	}
	return v, nil
}

// diagnostics replays one explicit saved endpoint/scope under existing serialized bounded report admission.
func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request, id, report string) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	policy, policyErr := presentationQuery(q, "endpoint", "scope")
	endpoint, scope := "local", "main"
	if q.Has("endpoint") {
		endpoint = q.Get("endpoint")
	}
	if q.Has("scope") {
		scope = q.Get("scope")
	}
	if err != nil || policyErr != nil || endpoint != "local" && endpoint != "server" {
		reject(w, 400, id, "invalid_request", "query", "explicit endpoint/scope required")
		return
	}
	p, err := s.store.SavedReport(report)
	if err == nil && endpoint == "server" {
		p = p.ServerReport
		if p == nil {
			err = errors.New("server evidence unavailable")
		}
	}
	if err != nil {
		s.diskReply(w, id, nil, err)
		return
	}
	v, err := buildDiagnostic(p, scope, policy)
	v.ReportID = report
	s.diskReply(w, id, v, err)
}

// preflight runs only explicit bounded host-local timer observations; it never sends data or tunes OS settings.
// Request cancellation and API shutdown interrupt waits; a second saved/preflight operation is rejected.
func (s *Server) preflight(w http.ResponseWriter, r *http.Request, id string) {
	var in struct {
		Version  int             `json:"schema_version"`
		ID       string          `json:"request_id"`
		Duration profile.Decimal `json:"duration_ns"`
		Interval profile.Decimal `json:"interval_ns"`
	}
	if !savedBody(w, r, id, &in) {
		return
	}
	if in.Version != 1 || in.ID != id || in.Duration == 0 || in.Duration > 1000000000 || in.Interval == 0 || (in.Duration-1)/in.Interval+1 > 4096 {
		reject(w, 400, id, "invalid_request", "preflight", "duration 1..1000000000 ns, positive interval, at most 4096 slots required")
		return
	}
	wakes, err := measure.Preflight(r.Context(), uint64(in.Duration), uint64(in.Interval))
	lags := []uint64{}
	var missed profile.Decimal
	for _, wake := range wakes {
		lags = append(lags, uint64(wake.Lag))
		missed += wake.Missed
	}
	status := "completed"
	if err != nil {
		status = "interrupted"
	}
	reply(w, 200, id, struct {
		Status   string          `json:"status"`
		Clock    string          `json:"clock"`
		Duration profile.Decimal `json:"duration_ns"`
		Interval profile.Decimal `json:"interval_ns"`
		Missed   profile.Decimal `json:"missed_slots"`
		P95      *string         `json:"lag_p95_ns"`
		P99      *string         `json:"lag_p99_ns"`
		Wakes    []measure.Wake  `json:"wakes"`
	}{status, "Go time.Since host-local monotonic clock; source/resolution unverified; timer lag is not network delay or calibration", in.Duration, in.Interval, missed, measure.Percentile(lags, 95), measure.Percentile(lags, 99), wakes}, nil)
}
