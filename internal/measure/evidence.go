package measure

import (
	"errors"
	"strings"

	"sneaky-port/internal/profile"
)

// EvidencePolicy versions advisory presentation targets independently of traffic and raw hashes.
type EvidencePolicy struct {
	Version  int             `json:"policy_version"`
	Median   profile.Decimal `json:"median_target"`
	P95      profile.Decimal `json:"p95_target"`
	P99      profile.Decimal `json:"p99_target"`
	Repeated profile.Decimal `json:"repeat_target"`
}

// DefaultEvidencePolicy selects engineering display heuristics, never statistical confidence.
func DefaultEvidencePolicy() EvidencePolicy { return EvidencePolicy{1, 20, 200, 1000, 3} }

// Validate bounds explicit preferences without relaxing any measurement or comparison validity gate.
func (p EvidencePolicy) Validate() error {
	if p.Version != 1 || p.Median < 1 || p.P95 < p.Median || p.P99 < p.P95 || p.P99 > 1000000 || p.Repeated < 1 || p.Repeated > 8 {
		return errors.New("invalid evidence presentation policy")
	}
	return nil
}

// EvidenceLabel describes a metric's own population and fixed reasons; values stay in their original fields.
// Eligible is the reached nonlocal workload for durations, or the declared count population for counters.
type EvidenceLabel struct {
	Coverage    string          `json:"coverage"`
	Basis       string          `json:"basis"`
	Eligible    profile.Decimal `json:"eligible"`
	Completed   profile.Decimal `json:"completed"`
	Missing     profile.Decimal `json:"missing"`
	LocalMisses profile.Decimal `json:"local_misses"`
	Gaps        profile.Decimal `json:"gaps"`
	Target      profile.Decimal `json:"advisory_target"`
	Reasons     []string        `json:"reasons"`
}

// LabelSummary annotates shared strict-cutoff summaries without recomputing or hiding measured values.
// Endpoint completeness includes losses; missing completions and small samples are independent reasons.
func LabelSummary(s Summary, complete bool, policy EvidencePolicy) map[string]EvidenceLabel {
	base := EvidenceLabel{Coverage: "complete", Basis: "locally measured", Eligible: s.Planned, Completed: s.Planned, LocalMisses: s.LocalMisses, Reasons: []string{}}
	if !complete {
		base.Coverage = "partial"
		base.Reasons = append(base.Reasons, "partial_observation")
	}
	if s.LocalMisses > 0 {
		base.Reasons = append(base.Reasons, "local_misses")
	}
	if s.Pending > 0 {
		base.Reasons = append(base.Reasons, "pending_deadlines")
	}
	out := map[string]EvidenceLabel{}
	for _, name := range []string{"Planned", "Skipped", "Attempted", "Written", "WriteFailed", "OnTime", "Late", "Unresolved", "DuplicateReceipts", "RTTSamples", "VariationGaps", "Accepted", "LocalMisses", "DeadlineMisses", "DeadlineMissFraction", "AcceptedMisses", "Pending", "CorrectedSamples", "InvalidCorrections", "VariationSamples", "SpikeEpisodes", "ReceiptStallEpisodes", "SpikePeak", "ReceiptStallMax"} {
		out[name] = base
	}
	for _, name := range []string{"RTTMedian", "RTTP95", "RTTP99", "RTTMin", "RTTMax", "ScheduledMedian", "ScheduledP95", "ScheduledP99", "CorrectedP95", "RTTVariation", "VariationP95", "VariationP99"} {
		label := base
		label.Reasons = append([]string{}, base.Reasons...)
		label.Eligible = s.Planned - min(s.Planned, s.Skipped)
		label.Completed = s.RTTSamples
		if strings.HasPrefix(name, "Variation") || name == "RTTVariation" {
			label.Completed = profile.Decimal(len(s.RTTVariation))
			label.Eligible = label.Completed + s.VariationGaps
			label.Gaps = s.VariationGaps
			if label.Gaps > 0 {
				label.Reasons = append(label.Reasons, "variation_gaps")
			}
		}
		if name == "CorrectedP95" {
			label.Basis = "estimated"
			label.Completed = s.CorrectedSamples
			if s.InvalidCorrections > 0 {
				label.Reasons = append(label.Reasons, "invalid_processing_corrections")
			}
		}
		label.Missing = label.Eligible - min(label.Eligible, label.Completed)
		if label.Missing > 0 {
			label.Reasons = append(label.Reasons, "missing_completions")
		}
		switch {
		case strings.Contains(name, "Median"):
			label.Target = policy.Median
		case strings.Contains(name, "P95"):
			label.Target = policy.P95
		case strings.Contains(name, "P99"):
			label.Target = policy.P99
		}
		if label.Completed == 0 {
			label.Coverage = "unavailable"
			label.Reasons = append(label.Reasons, "no_eligible_samples")
		}
		if label.Completed < label.Target {
			label.Reasons = append(label.Reasons, "small_sample")
		}
		out[name] = label
	}
	for name, source := range map[string]string{"RTTSamples": "RTTP95", "VariationSamples": "VariationP95", "VariationGaps": "VariationP95", "CorrectedSamples": "CorrectedP95", "InvalidCorrections": "CorrectedP95"} {
		label := out[source]
		label.Coverage = base.Coverage
		label.Target = 0
		label.Reasons = []string{}
		for _, reason := range out[source].Reasons {
			if reason != "small_sample" {
				label.Reasons = append(label.Reasons, reason)
			}
		}
		out[name] = label
	}
	for _, name := range []string{"Received", "MissingAtReceiver"} {
		label := base
		label.Reasons = []string{}
		label.Basis = "receiver reconciled"
		label.Eligible = s.Attempted
		label.Completed = s.Attempted
		if s.Received == nil || s.MissingAtReceiver == nil {
			label.Coverage = "unavailable"
			label.Basis = "unverified"
			label.Completed = 0
			label.Missing = s.Attempted
			label.Reasons = []string{"missing_remote_evidence"}
		}
		out[name] = label
	}
	out["TransportPacketLoss"] = EvidenceLabel{Coverage: "unavailable", Basis: "unverified", Reasons: []string{"packet_loss_unobserved"}}
	return out
}
