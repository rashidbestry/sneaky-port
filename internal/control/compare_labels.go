package control

import "sneaky-port/internal/measure"
import "sneaky-port/internal/profile"

// comparisonEvidencePolicy returns immutable request preferences or versioned display defaults.
func comparisonEvidencePolicy(q ComparisonRequest) measure.EvidencePolicy {
	if q.EvidencePolicy != nil {
		return *q.EvidencePolicy
	}
	return measure.DefaultEvidencePolicy()
}

// labelPairMetrics retains independent arm populations beside unchanged exact deltas and gates.
func labelPairMetrics(rows []MetricDelta, a, b map[string]measure.EvidenceLabel, state string) {
	for i := range rows {
		label := measure.EvidenceLabel{Coverage: "complete", Basis: "locally measured", Eligible: 2, Completed: 2, Reasons: []string{}}
		if rows[i].Metric == "Received" || rows[i].Metric == "MissingAtReceiver" {
			label.Basis = "receiver reconciled"
		}
		if rows[i].Delta == nil {
			label.Coverage = "unavailable"
			label.Completed = 0
			label.Missing = 2
			label.Reasons = append(label.Reasons, "comparison_unavailable")
		}
		if state == "incompatible" {
			label.Coverage = "incompatible"
			label.Reasons = append(label.Reasons, "incompatible_trials")
		} else if state == "inconclusive" {
			label.Reasons = append(label.Reasons, "incomplete_or_missing_environment")
		}
		rows[i].Evidence = label
		if label, ok := a[rows[i].Metric]; ok {
			copy := label
			rows[i].EvidenceA = &copy
		}
		if label, ok := b[rows[i].Metric]; ok {
			copy := label
			rows[i].EvidenceB = &copy
		}
	}
}

// labelRepeatedMetrics annotates two-pair validity and independent advisory repetition targets.
// Small samples propagate bounded reason codes while repeated quantiles remain unpooled.
func labelRepeatedMetrics(rows []RepeatedDelta, pairs [][]MetricDelta, p measure.EvidencePolicy) {
	for i := range rows {
		r := &rows[i]
		label := measure.EvidenceLabel{Coverage: "complete", Basis: "locally measured", Eligible: profile.Decimal(len(pairs)), Completed: profile.Decimal(r.Trials), Target: p.Repeated, Reasons: []string{}}
		label.Missing = label.Eligible - min(label.Eligible, label.Completed)
		if r.Trials == 0 {
			label.Coverage = "unavailable"
		} else if r.Trials != len(pairs) {
			label.Coverage = "partial"
		}
		if r.State == "inconclusive" {
			label.Reasons = append(label.Reasons, "comparison_inconclusive")
			if r.Trials == len(pairs) && r.Trials >= 2 {
				label.Reasons = append(label.Reasons, "mixed_repetitions")
			}
		}
		if r.Trials < 2 {
			label.Reasons = append(label.Reasons, "two_pairs_required")
		}
		if label.Completed < p.Repeated {
			label.Reasons = append(label.Reasons, "few_repeats")
		}
		seen := map[string]bool{}
		for _, pair := range pairs {
			if i < len(pair) {
				if pair[i].Evidence.Coverage == "incompatible" {
					label.Coverage = "incompatible"
					if !seen["incompatible_trials"] {
						label.Reasons = append(label.Reasons, "incompatible_trials")
						seen["incompatible_trials"] = true
					}
				}
				for _, arm := range []*measure.EvidenceLabel{pair[i].EvidenceA, pair[i].EvidenceB} {
					if arm != nil {
						for _, reason := range arm.Reasons {
							if !seen[reason] {
								seen[reason] = true
								label.Reasons = append(label.Reasons, reason)
							}
						}
					}
				}
			}
		}
		r.Evidence = label
	}
}

// labelComparison derives presentation after existing compatibility and delta decisions are finalized.
func labelComparison(out *Comparison) {
	p := comparisonEvidencePolicy(out.Selection)
	out.EvidencePolicy = p
	main := [][]MetricDelta{}
	for i := range out.Pairs {
		pair := &out.Pairs[i]
		labelPairMetrics(pair.Metrics, pair.A.Labels, pair.B.Labels, pair.State)
		for j := range pair.Phases {
			if j < len(pair.A.PhaseLabels) && j < len(pair.B.PhaseLabels) {
				labelPairMetrics(pair.Phases[j], pair.A.PhaseLabels[j], pair.B.PhaseLabels[j], pair.State)
			}
		}
		main = append(main, pair.Metrics)
	}
	labelRepeatedMetrics(out.Repeated, main, p)
	for j := range out.Phases {
		rows := [][]MetricDelta{}
		for _, pair := range out.Pairs {
			if j < len(pair.Phases) {
				rows = append(rows, pair.Phases[j])
			} else {
				rows = append(rows, nil)
			}
		}
		labelRepeatedMetrics(out.Phases[j], rows, p)
	}
}
