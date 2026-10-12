package control

import (
	"encoding/json"
	"errors"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
	"strings"
)

// ReportPresentation contains derived labels only; saved version-1 summaries and plan hashes stay unchanged.
type ReportPresentation struct {
	Policy  measure.EvidencePolicy             `json:"policy"`
	Main    map[string]measure.EvidenceLabel   `json:"main"`
	Warmup  map[string]measure.EvidenceLabel   `json:"warmup"`
	Phases  []map[string]measure.EvidenceLabel `json:"phases"`
	Context map[string]measure.EvidenceLabel   `json:"context"`
}

// ReportLabels uses already replayed populations and independent reconciliation, never offline authentication.
func ReportLabels(r *ReplayedReport, policy measure.EvidencePolicy) ReportPresentation {
	out := ReportPresentation{Policy: policy, Phases: []map[string]measure.EvidenceLabel{}, Context: map[string]measure.EvidenceLabel{}}
	if r == nil {
		return out
	}
	local := r.Summary.Local
	if r.Reconciliation != nil {
		if r.Manifest.Endpoint == "client" && r.Reconciliation.Client != nil {
			local = r.Reconciliation.Client
		}
		if r.Manifest.Endpoint == "server" && r.Reconciliation.Server != nil {
			local = r.Reconciliation.Server
		}
	}
	complete := r.Evidence.Complete && r.TailBytes == 0 && len(r.Accounting) == 0
	readFaults, invalidFrames, markingErrors := false, false, false
	for _, raw := range r.Evidence.Records {
		var record struct {
			Kind, Error string
			At          profile.Decimal
		}
		if json.Unmarshal(raw, &record) != nil || record.At >= r.Evidence.ObservationCutoff {
			continue
		}
		if record.Kind == "read_error" {
			readFaults = true
		}
		if record.Kind == "invalid" {
			invalidFrames = true
		}
		if record.Kind == "marking" && record.Error != "" {
			markingErrors = true
		}
	}
	if local != nil {
		out.Main = measure.LabelSummary(local.Summary, complete, policy)
		out.Warmup = measure.LabelSummary(local.Warmup, complete, policy)
		for _, s := range local.Phases {
			out.Phases = append(out.Phases, measure.LabelSummary(s, complete, policy))
		}
	}
	for _, labels := range append([]map[string]measure.EvidenceLabel{out.Main, out.Warmup}, out.Phases...) {
		for name, label := range labels {
			// Fixed reasons expose record faults without including filesystem paths, payloads or error text.
			label.Reasons = append([]string{}, label.Reasons...)
			if len(r.Accounting) > 0 {
				label.Reasons = append(label.Reasons, "accounting_errors")
			}
			if r.TailBytes > 0 {
				label.Reasons = append(label.Reasons, "discarded_tail")
			}
			if label.Basis == "locally measured" || label.Basis == "estimated" {
				if readFaults {
					label.Reasons = append(label.Reasons, "local_read_faults")
				}
				if invalidFrames {
					label.Reasons = append(label.Reasons, "invalid_frames_observed")
				}
			}
			labels[name] = label
		}
	}
	out.Context["OuterMarkings"] = measure.EvidenceLabel{Coverage: "unavailable", Basis: "unverified", Reasons: []string{"markings_unobserved"}}
	if markingErrors {
		label := out.Context["OuterMarkings"]
		label.Reasons = append(label.Reasons, "marking_application_failed")
		out.Context["OuterMarkings"] = label
	}
	env := measure.EvidenceLabel{Coverage: "complete", Basis: "operator declared", Reasons: []string{"environment_unverified"}}
	if strings.TrimSpace(r.Manifest.Plan.MatchedMetadata["environment"]) == "" {
		env.Coverage = "unavailable"
		env.Reasons = append(env.Reasons, "environment_missing")
	}
	out.Context["Environment"] = env
	for i, name := range []string{"OneWayUpstream", "OneWayDownstream"} {
		label := measure.EvidenceLabel{Coverage: "unavailable", Basis: "unverified", Reasons: []string{"missing_remote_or_clock_evidence"}}
		if r.Reconciliation != nil && r.Reconciliation.OneWay != nil {
			w := r.Reconciliation.OneWay
			c := w.Directions[i]
			label.Eligible = profile.Decimal(c.Calibrated + c.Estimated + c.Unavailable)
			label.Completed = profile.Decimal(c.Calibrated + c.Estimated)
			label.Missing = profile.Decimal(c.Unavailable)
			label.Reasons = []string{}
			if label.Completed > 0 {
				label.Coverage = "complete"
				label.Basis = "estimated"
				if label.Missing > 0 {
					label.Coverage = "partial"
				}
			}
			if !w.BoundsVerified {
				label.Reasons = append(label.Reasons, "clock_bounds_unverified")
			}
			if c.Unavailable > 0 {
				label.Reasons = append(label.Reasons, "clock_or_capture_unavailable")
			}
			for _, result := range w.Results {
				if result.Direction != byte(i+1) {
					continue
				}
				if result.Interval.Reason == "invalid/expired clock epoch" {
					label.Reasons = append(label.Reasons, "clock_epoch_or_expiry_unavailable")
					break
				}
			}
			for _, result := range w.Results {
				if result.Direction == byte(i+1) && result.Interval.Reason == "insufficient unexpired clock samples" {
					label.Reasons = append(label.Reasons, "clock_window_insufficient")
					break
				}
			}
			if w.OmittedModels > 0 || w.OmittedResults > 0 {
				label.Reasons = append(label.Reasons, "details_omitted")
			}
		}
		out.Context[name] = label
	}
	return out
}

// ReportContextLabels derives independent clock/marking/environment evidence without constructing phase labels.
func ReportContextLabels(r *ReplayedReport, policy measure.EvidencePolicy) map[string]measure.EvidenceLabel {
	if r == nil {
		return ReportLabels(nil, policy).Context
	}
	copy := *r
	copy.Summary.Local = nil
	if copy.Reconciliation != nil {
		reconciliation := *copy.Reconciliation
		reconciliation.Client = nil
		reconciliation.Server = nil
		copy.Reconciliation = &reconciliation
	}
	return ReportLabels(&copy, policy).Context
}

// ApplyPresentation recalculates only derived labels for explicit validated presentation preferences.
func ApplyPresentation(r *ReplayedReport, policy measure.EvidencePolicy) error {
	if r == nil {
		return errors.New("nil replayed report")
	}
	if err := policy.Validate(); err != nil {
		return err
	}
	r.Presentation = ReportLabels(r, policy)
	if r.ServerReport != nil {
		if err := ApplyPresentation(r.ServerReport, policy); err != nil {
			return err
		}
	}
	return nil
}
