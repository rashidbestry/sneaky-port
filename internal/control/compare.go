package control

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
)

// TrialPair preserves operator-selected A/B order; selection does not prove execution order.
type TrialPair struct {
	A string `json:"a"`
	B string `json:"b"`
}

// ComparisonRequest selects bounded saved trials, predeclared axes and one originating scope.
type ComparisonRequest struct {
	Pairs          []TrialPair             `json:"pairs"`
	Axes           []string                `json:"axes"`
	Endpoint       string                  `json:"endpoint"`
	Scope          string                  `json:"scope"`
	EvidencePolicy *measure.EvidencePolicy `json:"evidence_policy,omitempty"`
}

// ComparisonTrial retains settings/provenance and scalar populations without retaining raw replay heaps.
type ComparisonTrial struct {
	ReportID      string                             `json:"report_id"`
	Manifest      EvidenceManifest                   `json:"manifest"`
	Status        string                             `json:"status"`
	Integrity     string                             `json:"integrity"`
	Reason        string                             `json:"reason"`
	Complete      bool                               `json:"complete"`
	Cutoff        profile.Decimal                    `json:"cutoff_ns"`
	Generation    profile.Decimal                    `json:"generation_cutoff_ns"`
	Markings      []PhaseMarking                     `json:"phase_markings"`
	Metrics       []*string                          `json:"metrics"`
	PhaseMetrics  [][]*string                        `json:"phase_metrics"`
	Limitations   []string                           `json:"limitations"`
	PeerPlatform  []string                           `json:"peer_platform"`
	Faults        map[string]profile.Decimal         `json:"endpoint_faults"`
	Labels        map[string]measure.EvidenceLabel   `json:"evidence_labels"`
	PhaseLabels   []map[string]measure.EvidenceLabel `json:"phase_evidence_labels"`
	ContextLabels map[string]measure.EvidenceLabel   `json:"context_evidence_labels"`
}

// MetricDelta is exact B minus A, with rational fractions and unavailable values kept distinct.
type MetricDelta struct {
	Metric    string                 `json:"metric"`
	Unit      string                 `json:"unit"`
	A         *string                `json:"a"`
	B         *string                `json:"b"`
	Delta     *string                `json:"delta"`
	State     string                 `json:"state"`
	EvidenceA *measure.EvidenceLabel `json:"evidence_a,omitempty"`
	EvidenceB *measure.EvidenceLabel `json:"evidence_b,omitempty"`
	Evidence  measure.EvidenceLabel  `json:"evidence"`
}

// PairComparison preserves incompatible/partial populations rather than dropping unsuccessful trials.
type PairComparison struct {
	A       ComparisonTrial `json:"a"`
	B       ComparisonTrial `json:"b"`
	State   string          `json:"state"`
	Reasons []string        `json:"reasons"`
	Metrics []MetricDelta   `json:"metrics"`
	Phases  [][]MetricDelta `json:"phases"`
}

// RepeatedDelta describes per-trial delta extrema, never a pooled quantile or confidence estimate.
type RepeatedDelta struct {
	Metric   string                `json:"metric"`
	Unit     string                `json:"unit"`
	State    string                `json:"state"`
	Min      *string               `json:"min_delta"`
	Max      *string               `json:"max_delta"`
	Trials   int                   `json:"trials"`
	Evidence measure.EvidenceLabel `json:"evidence"`
}

// Comparison contains no winner; repeated main and phase outcomes remain independently descriptive.
type Comparison struct {
	Version        int                    `json:"schema_version"`
	Selection      ComparisonRequest      `json:"selection"`
	Pairs          []PairComparison       `json:"pairs"`
	Repeated       []RepeatedDelta        `json:"repeated"`
	Phases         [][]RepeatedDelta      `json:"phases"`
	Policy         string                 `json:"policy"`
	EvidencePolicy measure.EvidencePolicy `json:"evidence_policy"`
}

var comparisonNames = []string{"DeadlineMisses", "DeadlineMissFraction", "Planned", "LocalMisses", "AcceptedMisses", "Pending", "OnTime", "Late", "Unresolved", "RTTP95", "RTTP99", "RTTSamples", "VariationP95", "VariationP99", "VariationSamples", "VariationGaps", "Received", "MissingAtReceiver", "Skipped", "Accepted", "Attempted", "Written", "WriteFailed", "ScheduledP95", "ScheduledP99", "TransportPacketLoss", "SpikeEpisodes", "ReceiptStallEpisodes", "SpikePeak", "ReceiptStallMax"}

// comparisonUnit supplies explicit units for exact counts, durations and scheduled-slot fractions.
func comparisonUnit(name string) string {
	if name == "DeadlineMissFraction" {
		return "misses/planned slots"
	}
	if name == "TransportPacketLoss" {
		return "unavailable"
	}
	if name == "RTTSamples" {
		return "samples"
	}
	if name == "VariationSamples" || name == "VariationGaps" {
		return "pairs"
	}
	if name == "SpikeEpisodes" || name == "ReceiptStallEpisodes" {
		return "episodes"
	}
	if name == "RTTP95" || name == "RTTP99" || name == "VariationP95" || name == "VariationP99" || name == "ScheduledP95" || name == "ScheduledP99" || name == "SpikePeak" || name == "ReceiptStallMax" {
		return "ns"
	}
	return "messages"
}

// comparisonEpisodes uses shared spike/stall rules with exactly the selected lifecycle/origin phase.
func comparisonEpisodes(r *ReplayedReport, lifecycle byte, phase *uint32) []*string {
	a := measure.AnalyzeSnapshot(r.Evidence.Entries, uint64(r.Evidence.ObservationCutoff), uint64(r.Manifest.Plan.SpikeThreshold), lifecycle, phase)
	spikes, stalls := strconv.Itoa(len(a.Spikes)), strconv.Itoa(len(a.Stalls))
	var peak, duration profile.Decimal
	for _, s := range a.Spikes {
		peak = max(peak, s.Peak)
	}
	for _, s := range a.Stalls {
		duration = max(duration, s.Duration)
	}
	p, d := strconv.FormatUint(uint64(peak), 10), strconv.FormatUint(uint64(duration), 10)
	return []*string{&spikes, &stalls, &p, &d}
}

// comparisonValues retains full-population nearest-rank tails and exact denominators per scope.
func comparisonValues(s measure.Summary) []*string {
	count := func(n profile.Decimal) *string { // Preserve uint64 counts beyond browser precision.
		v := strconv.FormatUint(uint64(n), 10)
		return &v
	}
	optional := func(n *profile.Decimal) *string { // Missing receiver evidence stays unavailable.
		if n == nil {
			return nil
		}
		return count(*n)
	}
	var fraction *string
	if s.Planned > 0 {
		v := new(big.Rat).SetFrac(new(big.Int).SetUint64(uint64(s.DeadlineMisses)), new(big.Int).SetUint64(uint64(s.Planned))).RatString()
		fraction = &v
	}
	variation := make([]uint64, len(s.RTTVariation))
	for i, v := range s.RTTVariation {
		variation[i] = uint64(v)
	}
	return []*string{count(s.DeadlineMisses), fraction, count(s.Planned), count(s.LocalMisses), count(s.AcceptedMisses), count(s.Pending), count(s.OnTime), count(s.Late), count(s.Unresolved), s.RTTP95, s.RTTP99, count(s.RTTSamples), measure.Percentile(variation, 95), measure.Percentile(variation, 99), count(profile.Decimal(len(variation))), count(s.VariationGaps), optional(s.Received), optional(s.MissingAtReceiver), count(s.Skipped), count(s.Accepted), count(s.Attempted), count(s.Written), count(s.WriteFailed), s.ScheduledP95, s.ScheduledP99, s.TransportPacketLoss}
}

// Validate rejects ambiguous selections before any report is read; limits are engineering caps.
func (q ComparisonRequest) Validate() error {
	if q.EvidencePolicy != nil {
		if err := q.EvidencePolicy.Validate(); err != nil {
			return err
		}
	}
	if len(q.Pairs) < 1 || len(q.Pairs) > 8 || q.Axes == nil || q.Endpoint != "local" && q.Endpoint != "server" {
		return errors.New("select 1-8 pairs, explicit axes and local/server endpoint")
	}
	if q.Scope != "main" && q.Scope != "warmup" {
		n, err := strconv.ParseUint(q.Scope, 10, 32)
		if err != nil || strconv.FormatUint(n, 10) != q.Scope || n >= 64 {
			return errors.New("invalid originating scope")
		}
	}
	seen := map[string]bool{}
	for _, pair := range q.Pairs {
		for _, id := range []string{pair.A, pair.B} {
			if !profile.SafeID(id) || seen[id] {
				return errors.New("unique safe report IDs required")
			}
			seen[id] = true
		}
	}
	seen = map[string]bool{}
	for _, axis := range q.Axes {
		if axis != "adapter" && axis != "port" && axis != "client_dscp" && axis != "server_dscp" || seen[axis] {
			return errors.New("unsupported or duplicate axis")
		}
		seen[axis] = true
	}
	return nil
}

// comparisonTrial projects one successfully replayed report; server absence remains explicit.
func comparisonTrial(id string, r *ReplayedReport, q ComparisonRequest) (ComparisonTrial, error) {
	if len(r.Manifest.Plan.Phases) > 64 {
		return ComparisonTrial{}, errors.New("comparison phase capacity exceeded")
	}
	t := ComparisonTrial{ReportID: id, Manifest: r.Manifest, Status: r.Status, Integrity: r.Integrity, Reason: r.Reason, Limitations: []string{"Offline consistency is not authentication; environment metadata is operator-declared, not verified.", "Selection order and storage creation timestamps do not establish alternating, sequential or uncontended execution.", "Source/NAT bindings, actual outer overhead, achieved rate and setup timing need raw/export review; no equality or ISP effect is inferred.", "One-way comparison unavailable: production clock bounds are unverified. Priority-effect conclusions require independent outer-packet evidence."}}
	local := r.Summary.Local
	if r.ServerReport != nil {
		m := r.ServerReport.Manifest
		if q.Endpoint == "server" {
			m = r.Manifest
		}
		t.PeerPlatform = comparisonPlatform(m)
	}
	if q.Endpoint == "server" {
		if r.ServerReport == nil {
			t.Status = "unavailable"
			t.Reason = "transferred server evidence unavailable"
			t.Metrics = make([]*string, len(comparisonNames))
			return t, nil
		}
		r = r.ServerReport
		t.Manifest, t.Status, t.Integrity, t.Reason = r.Manifest, r.Status, r.Integrity, r.Reason
		local = r.Summary.Local
	}
	if local == nil {
		return t, errors.New("recomputed local summary unavailable")
	}
	t.Complete = r.Status == "completed" && r.Evidence.Complete && len(r.Accounting) == 0 && r.TailBytes == 0
	t.Cutoff, t.Generation, t.Markings = local.Observation, local.Generation, local.Markings
	t.Faults = map[string]profile.Decimal{"accounting_errors": profile.Decimal(len(r.Accounting)), "discarded_tail_bytes": r.TailBytes, "read_errors": 0, "invalid_frames": 0, "marking_errors": 0}
	for _, raw := range r.Evidence.Records {
		var record struct {
			Kind, Error string
			At          profile.Decimal
		}
		if json.Unmarshal(raw, &record) != nil || record.At >= t.Cutoff {
			continue
		}
		switch record.Kind {
		case "read_error":
			t.Faults["read_errors"]++
		case "invalid":
			t.Faults["invalid_frames"]++
		case "marking":
			if record.Error != "" {
				t.Faults["marking_errors"]++
			}
		}
	}
	// Reconciliation supplies application delivery only when existing independent receiver gates pass.
	// The enclosing report is selected separately below by the caller; endpoint-only tails are unchanged.
	summary := local.Summary
	lifecycle := byte(2)
	var selectedPhase *uint32
	if q.Scope == "warmup" {
		summary = local.Warmup
		lifecycle = 1
	} else if q.Scope != "main" {
		n, _ := strconv.Atoi(q.Scope)
		if n >= len(local.Phases) {
			return t, errors.New("scope outside saved phases")
		}
		summary = local.Phases[n]
		index := uint32(n)
		selectedPhase = &index
	}
	t.Metrics = append(comparisonValues(summary), comparisonEpisodes(r, lifecycle, selectedPhase)...)
	t.Labels = measure.LabelSummary(summary, t.Complete, comparisonEvidencePolicy(q))
	t.ContextLabels = ReportContextLabels(r, comparisonEvidencePolicy(q))
	for i, phase := range local.Phases {
		index := uint32(i)
		t.PhaseMetrics = append(t.PhaseMetrics, append(comparisonValues(phase), comparisonEpisodes(r, 2, &index)...))
		t.PhaseLabels = append(t.PhaseLabels, measure.LabelSummary(phase, t.Complete, comparisonEvidencePolicy(q)))
	}
	if strings.TrimSpace(r.Manifest.Plan.MatchedMetadata["environment"]) == "" {
		t.Limitations = append(t.Limitations, "Missing matched_metadata.environment; descriptive matching remains inconclusive.")
	}
	for _, labels := range append([]map[string]measure.EvidenceLabel{t.Labels}, t.PhaseLabels...) {
		for name, label := range labels {
			label.Reasons = append([]string{}, label.Reasons...)
			if label.Basis == "locally measured" || label.Basis == "estimated" {
				if t.Faults["read_errors"] > 0 {
					label.Reasons = append(label.Reasons, "local_read_faults")
				}
				if t.Faults["invalid_frames"] > 0 {
					label.Reasons = append(label.Reasons, "invalid_frames_observed")
				}
			}
			if len(r.Accounting) > 0 {
				label.Reasons = append(label.Reasons, "accounting_errors")
			}
			if r.TailBytes > 0 {
				label.Reasons = append(label.Reasons, "discarded_tail")
			}
			labels[name] = label
		}
	}
	return t, nil
}

// comparisonDelivery replaces only delivery populations using recomputed compatible reconciliation.
func comparisonDelivery(t *ComparisonTrial, r *ReplayedReport, q ComparisonRequest) {
	if r.Reconciliation == nil {
		return
	}
	l := r.Reconciliation.Client
	if q.Endpoint == "server" {
		l = r.Reconciliation.Server
	}
	if l == nil {
		return
	}
	s := l.Summary
	if q.Scope == "warmup" {
		s = l.Warmup
	} else if q.Scope != "main" {
		n, _ := strconv.Atoi(q.Scope)
		if n >= len(l.Phases) {
			return
		}
		s = l.Phases[n]
	}
	v := comparisonValues(s)
	t.Metrics[16], t.Metrics[17] = v[16], v[17]
	labels := measure.LabelSummary(s, t.Complete, comparisonEvidencePolicy(q))
	t.Labels["Received"], t.Labels["MissingAtReceiver"] = labels["Received"], labels["MissingAtReceiver"]
	for i, p := range l.Phases {
		if i < len(t.PhaseMetrics) {
			v = comparisonValues(p)
			t.PhaseMetrics[i][16], t.PhaseMetrics[i][17] = v[16], v[17]
			labels := measure.LabelSummary(p, t.Complete, comparisonEvidencePolicy(q))
			t.PhaseLabels[i]["Received"], t.PhaseLabels[i]["MissingAtReceiver"] = labels["Received"], labels["MissingAtReceiver"]
		}
	}
}

// normalizedComparison strips only selected axes and profile labels; workload/environment stays exact.
func normalizedComparison(p profile.Plan, axes []string, differentAdapter bool) map[string]any {
	b, _ := json.Marshal(p)
	out := map[string]any{}
	json.Unmarshal(b, &out)
	delete(out, "profile_id")
	delete(out, "comparison_axes")
	for _, axis := range axes {
		switch axis {
		case "adapter":
			if differentAdapter {
				delete(out, "adapter_id")
				delete(out, "adapter_contract_version")
				delete(out, "adapter_options")
			}
		case "port":
			delete(out, "destination_port")
			delete(out, "source_port")
		case "client_dscp", "server_dscp":
			role := "client"
			if axis == "server_dscp" {
				role = "server"
			}
			for _, phase := range out["phases"].([]any) {
				delete(phase.(map[string]any)[role].(map[string]any), "dscp")
			}
		}
	}
	return out
}

// comparisonPlatform preserves declared build/host/clock identity independently of variable bindings.
func comparisonPlatform(m EvidenceManifest) []string {
	details := ""
	if m.ClockDetails != nil {
		b, _ := json.Marshal(m.ClockDetails) // The fixed scalar provenance structure cannot fail JSON encoding.
		details = string(b)
	}
	return []string{m.OS, m.Arch, m.Go, m.Revision, m.Modified, m.Clock, details}
}

// comparisonReasons names non-axis mismatches without using floating-point measurement quantities.
func comparisonReasons(a, b ComparisonTrial, axes []string) []string {
	reasons := []string{}
	x, y := normalizedComparison(a.Manifest.Plan, axes, a.Manifest.Plan.Adapter != b.Manifest.Plan.Adapter), normalizedComparison(b.Manifest.Plan, axes, a.Manifest.Plan.Adapter != b.Manifest.Plan.Adapter)
	for key, value := range x {
		if !reflect.DeepEqual(value, y[key]) {
			reasons = append(reasons, "requested_plan."+key+" differs outside selected axes")
		}
	}
	for _, t := range []ComparisonTrial{a, b} {
		declared := append([]string{}, t.Manifest.Plan.ComparisonAxes...)
		selected := append([]string{}, axes...)
		sort.Strings(declared)
		sort.Strings(selected)
		if !reflect.DeepEqual(declared, selected) {
			reasons = append(reasons, "selected axes differ from frozen comparison_axes")
		}
	}
	if a.Status != "unavailable" && b.Status != "unavailable" && a.Manifest.Endpoint != b.Manifest.Endpoint {
		reasons = append(reasons, "originating endpoints differ")
	}
	if a.Status != "unavailable" && b.Status != "unavailable" && !reflect.DeepEqual(comparisonPlatform(a.Manifest), comparisonPlatform(b.Manifest)) {
		reasons = append(reasons, "endpoint platform/tool/clock provenance differs")
	}
	if len(a.PeerPlatform) > 0 && len(b.PeerPlatform) > 0 && !reflect.DeepEqual(a.PeerPlatform, b.PeerPlatform) {
		reasons = append(reasons, "peer platform/tool/clock provenance differs")
	}
	sort.Strings(reasons)
	return reasons
}

// metricDeltas computes signed exact B-A values only for compatible complete trial pairs.
func metricDeltas(a, b []*string, eligible bool) []MetricDelta {
	out := []MetricDelta{}
	for i, name := range comparisonNames {
		m := MetricDelta{Metric: name, Unit: comparisonUnit(name), State: "inconclusive"}
		if i < len(a) {
			m.A = a[i]
		}
		if i < len(b) {
			m.B = b[i]
		}
		if eligible && m.A != nil && m.B != nil {
			x, ok := new(big.Rat).SetString(*m.A)
			y, valid := new(big.Rat).SetString(*m.B)
			if ok && valid {
				d := new(big.Rat).Sub(y, x)
				v := d.RatString()
				m.Delta = &v
				m.State = "descriptive"
				if d.Sign() == 0 {
					m.State = "tie"
				}
			}
		}
		out = append(out, m)
	}
	return out
}

// repeatDeltas retains exact trial delta ranges; missing trials and opposite signs stay inconclusive.
func repeatDeltas(rows [][]MetricDelta) []RepeatedDelta {
	out := []RepeatedDelta{}
	for i, name := range comparisonNames {
		r := RepeatedDelta{Metric: name, Unit: comparisonUnit(name), State: "inconclusive"}
		var low, high *big.Rat
		for _, row := range rows {
			if i >= len(row) || row[i].Delta == nil {
				continue
			}
			d, ok := new(big.Rat).SetString(*row[i].Delta)
			if !ok {
				continue
			}
			r.Trials++
			if low == nil || d.Cmp(low) < 0 {
				low = d
			}
			if high == nil || d.Cmp(high) > 0 {
				high = d
			}
		}
		if low != nil {
			a, b := low.RatString(), high.RatString()
			r.Min, r.Max = &a, &b
		}
		if len(rows) >= 2 && r.Trials == len(rows) {
			if low.Sign() == 0 && high.Sign() == 0 {
				r.State = "tie"
			} else if low.Sign() >= 0 || high.Sign() <= 0 {
				r.State = "descriptive"
			}
		}
		out = append(out, r)
	}
	return out
}

// compareTrials checks every pair and consistent A/B configurations across repeated selected trials.
func compareTrials(q ComparisonRequest, trials []ComparisonTrial) Comparison {
	out := Comparison{Version: 1, Selection: q, Pairs: []PairComparison{}, Policy: "Exact B minus A; fractions are rational misses/planned slots. Per-trial tails are never pooled. At least two complete compatible pairs are needed for a repeated description; opposite signs or missing evidence remain inconclusive. Ties are exact, with no material-difference threshold, confidence score, winner, clock or priority-effect claim. Phase rows use originating main phases independently of selected scope. Retain failed/incompatible trials and review raw/export evidence and limitations."}
	for i := range q.Pairs {
		a, b := trials[2*i], trials[2*i+1]
		p := PairComparison{A: a, B: b, State: "descriptive", Reasons: comparisonReasons(a, b, q.Axes), Phases: [][]MetricDelta{}}
		if i > 0 {
			for j, t := range []ComparisonTrial{a, b} {
				x, _ := json.Marshal(normalizedComparison(trials[j].Manifest.Plan, nil, false))
				y, _ := json.Marshal(normalizedComparison(t.Manifest.Plan, nil, false))
				if string(x) != string(y) {
					p.Reasons = append(p.Reasons, "repeated arm configuration differs from first pair")
				}
				if trials[j].Status != "unavailable" && t.Status != "unavailable" && (!reflect.DeepEqual(comparisonPlatform(trials[j].Manifest), comparisonPlatform(t.Manifest)) || len(trials[j].PeerPlatform) > 0 && len(t.PeerPlatform) > 0 && !reflect.DeepEqual(trials[j].PeerPlatform, t.PeerPlatform)) {
					p.Reasons = append(p.Reasons, "repeated platform/tool/clock provenance differs")
				}
			}
		}
		if len(p.Reasons) > 0 {
			p.State = "incompatible"
		} else if !a.Complete || !b.Complete || strings.TrimSpace(a.Manifest.Plan.MatchedMetadata["environment"]) == "" || strings.TrimSpace(b.Manifest.Plan.MatchedMetadata["environment"]) == "" {
			p.State = "inconclusive"
			p.Reasons = append(p.Reasons, "incomplete observation or missing declared environment")
		}
		eligible := p.State == "descriptive"
		p.Metrics = metricDeltas(a.Metrics, b.Metrics, eligible)
		for phase := range a.Manifest.Plan.Phases {
			var x, y []*string
			if phase < len(a.PhaseMetrics) {
				x = a.PhaseMetrics[phase]
			}
			if phase < len(b.PhaseMetrics) {
				y = b.PhaseMetrics[phase]
			}
			p.Phases = append(p.Phases, metricDeltas(x, y, eligible))
		}
		if eligible {
			allTie := true
			for _, row := range append([][]MetricDelta{p.Metrics}, p.Phases...) {
				for _, m := range row {
					if m.Delta != nil && m.State != "tie" {
						allTie = false
					}
				}
			}
			if allTie {
				p.State = "tie"
			}
		}
		out.Pairs = append(out.Pairs, p)
	}
	rows := [][]MetricDelta{}
	for _, p := range out.Pairs {
		rows = append(rows, p.Metrics)
	}
	out.Repeated = repeatDeltas(rows)
	for phase := range trials[0].Manifest.Plan.Phases {
		rows = nil
		for _, p := range out.Pairs {
			if phase < len(p.Phases) {
				rows = append(rows, p.Phases[phase])
			} else {
				rows = append(rows, nil)
			}
		}
		out.Phases = append(out.Phases, repeatDeltas(rows))
	}
	return out
}

// CompareSavedReports replays sequentially under one aggregate 8 MiB read budget and discards raw heaps.
// Cancellation is checked between bounded file replays; no sockets, runs or saved files are changed.
func (s *Store) CompareSavedReports(ctx context.Context, q ComparisonRequest) (Comparison, error) {
	if err := q.Validate(); err != nil {
		return Comparison{}, err
	}
	dir, err := s.ownedDir("reports", false)
	if err != nil {
		return Comparison{}, err
	}
	remaining := uint64(APIReportBytes)
	trials := []ComparisonTrial{}
	seen := map[string]bool{}
	for _, pair := range q.Pairs {
		for _, id := range []string{pair.A, pair.B} {
			if err := ctx.Err(); err != nil {
				return Comparison{}, err
			}
			path := filepath.Join(dir, id)
			if _, err := os.Lstat(path); err != nil {
				return Comparison{}, err
			}
			r, err := replayReport(path, &remaining, true, s.AdapterRegistry())
			if err != nil {
				return Comparison{}, err
			}
			if seen[r.Manifest.RunID] {
				return Comparison{}, errors.New("duplicate measurement run identity; imports are not repetitions")
			}
			seen[r.Manifest.RunID] = true
			t, err := comparisonTrial(id, r, q)
			if err != nil {
				return Comparison{}, err
			}
			comparisonDelivery(&t, r, q)
			trials = append(trials, t)
		}
	}
	out := compareTrials(q, trials)
	labelComparison(&out)
	b, err := json.Marshal(out)
	if err != nil || len(b) > 16<<20 {
		return Comparison{}, errors.New("comparison exceeds serialized response budget")
	}
	return out, ctx.Err()
}
