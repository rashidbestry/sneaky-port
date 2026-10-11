package control

import (
	"encoding/json"
	"fmt"
	"sort"

	"sneaky-port/internal/calibration"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// OneWayReport is a bounded derived view of matched application captures, never wire latency.
// Exact populations include unretained details; endpoint-only RTT/delivery summaries remain unchanged.
type OneWayReport struct {
	Status          string              `json:"status"`
	Reason          string              `json:"reason"`
	Policy          profile.Calibration `json:"policy"`
	BoundsVerified  bool                `json:"bounds_verified"`
	Boundary        string              `json:"boundary"`
	ClientRawSHA256 string              `json:"client_raw_sha256"`
	ServerRawSHA256 string              `json:"server_raw_sha256"`
	DetailBudget    profile.Decimal     `json:"detail_budget_bytes"`
	DetailBytes     profile.Decimal     `json:"detail_bytes"`
	OmittedModels   int                 `json:"omitted_models"`
	OmittedResults  int                 `json:"omitted_results"`
	Calibrated      int                 `json:"calibrated_count"`
	Estimated       int                 `json:"estimate_count"`
	Unavailable     int                 `json:"unavailable_count"`
	Directions      [2]OneWayCounts     `json:"direction_counts"`
	Models          []ClockRevision     `json:"model_revisions"`
	Results         []OneWayResult      `json:"results"`
}

// OneWayCounts preserves exact per-direction populations even when detail admission is exhausted.
type OneWayCounts struct {
	Direction   byte `json:"original_direction"`
	Calibrated  int  `json:"calibrated_count"`
	Estimated   int  `json:"estimate_count"`
	Unavailable int  `json:"unavailable_count"`
}

// ClockRevision identifies an immutable causal window; a later revision never upgrades old results.
type ClockRevision struct {
	ID             string          `json:"model_id"`
	At             profile.Decimal `json:"available_at_ns"`
	First          profile.Decimal `json:"valid_from_ns"`
	Last           profile.Decimal `json:"last_sample_ns"`
	Expiry         profile.Decimal `json:"expiry_ns"`
	ClientEpoch    uint32          `json:"client_epoch"`
	ServerEpoch    uint32          `json:"server_epoch"`
	Status         string          `json:"status"`
	Reason         string          `json:"reason"`
	CalibrationIDs []string        `json:"calibration_ids"`
}

// OneWayResult ties an interval to original identity and both host-local application captures.
// Selection uses client send time upstream and client arrival downstream, never a cross-host subtraction.
type OneWayResult struct {
	Identity      measure.Identity     `json:"original_identity"`
	Direction     byte                 `json:"original_direction"`
	ClientCapture *profile.Decimal     `json:"client_capture_ns"`
	ServerCapture *profile.Decimal     `json:"server_capture_ns"`
	Units         string               `json:"delay_units"`
	ModelID       string               `json:"model_id"`
	Age           *profile.Decimal     `json:"model_age_ns"`
	Interval      calibration.Interval `json:"delay"`
}

// oneWayQuery holds matched immutable endpoint captures until ordered by client-local time.
type oneWayQuery struct {
	result  OneWayResult
	matched bool
	at      profile.Decimal
}

// productionOneWay fixes verification to false: a profile source label cannot attest clock bounds.
func productionOneWay(client, server *ReplayedReport, r *Reconciliation) *OneWayReport {
	c := client.Manifest.Plan.Calibration
	p := calibration.Policy{DriftPPB: uint64(c.DriftPPB), TimestampError: uint64(c.TimestampError), ModelError: uint64(c.ModelError), Expiry: uint64(c.Expiry), MaxError: uint64(c.MaxError), Source: c.BoundSource}
	return deriveOneWay(client, server, r, p)
}

// retainOneWay charges exact serialized detail bytes against the smaller explicit event/metadata cap.
// Raw inputs remain available for replay; exhaustion is visible and never changes exact populations.
func retainOneWay(r *OneWayReport, v any) bool {
	b, err := json.Marshal(v)
	n := profile.Decimal(len(b) + 1)
	if err != nil || n > r.DetailBudget-r.DetailBytes {
		return false
	}
	r.DetailBytes += n
	return true
}

// oneWayQueries reuses reconciliation's complete directional gates and exact attempt/arrival matches.
// Missing/ambiguous data creates unavailable timing, never an inferred arrival or transport-loss claim.
func oneWayQueries(origin, peer engine.Result, compatible bool, upstream bool) []oneWayQuery {
	arrivals := make(map[measure.Identity]engine.Record)
	for _, raw := range peer.Records {
		var record engine.Record
		if json.Unmarshal(raw, &record) == nil && record.Kind == "arrival" && record.FrameKind == probe.Data && record.At < peer.ObservationCutoff {
			arrivals[record.Identity] = record
		}
	}
	var out []oneWayQuery
	for _, e := range origin.Entries {
		if e.Attempt == nil || *e.Attempt >= origin.ObservationCutoff {
			continue
		}
		q := oneWayQuery{result: OneWayResult{Identity: e.Identity, Direction: origin.Direction, Interval: calibration.Interval{Status: "unavailable", Reason: "missing or incompatible matched receiver capture", CalibrationIDs: []string{}}}}
		a, ok := arrivals[e.Identity]
		arrival, observed := peer.Receiver.Arrivals[e.Identity]
		q.matched = compatible && ok && observed && a.Attempt == *e.Attempt && a.Direction == origin.Direction && a.Epoch == origin.Epoch && arrival == uint64(a.At)
		if upstream {
			q.result.ClientCapture, q.at = e.Attempt, *e.Attempt
			q.result.Units = "server monotonic nanoseconds"
			if q.matched {
				q.result.ServerCapture = &a.At
			}
		} else {
			q.result.ServerCapture = e.Attempt
			q.result.Units = "client monotonic nanoseconds"
			if q.matched {
				q.result.ClientCapture, q.at = &a.At, a.At
			}
		}
		out = append(out, q)
	}
	return out
}

// deriveOneWay reconstructs causal revisions from validated raw captures and matched data only.
// Production calls supply unverified bounds; Verified is reserved for independently justified fixtures.
func deriveOneWay(client, server *ReplayedReport, reconciliation *Reconciliation, policy calibration.Policy) *OneWayReport {
	p := client.Manifest.Plan
	out := &OneWayReport{Status: "unavailable", Reason: "no eligible matched captures with a valid clock envelope", Policy: p.Calibration, BoundsVerified: policy.Verified, Boundary: "origin pre-encoding attempt to authenticated complete application-frame arrival; includes serialization, local submission and carrier buffering; not packet transit", ClientRawSHA256: client.Summary.SHA256, ServerRawSHA256: server.Summary.SHA256, DetailBudget: min(p.Limits.EventBytes, profile.Decimal(FrameMax/4)), Models: []ClockRevision{}, Results: []OneWayResult{}}
	out.Directions[0].Direction, out.Directions[1].Direction = 1, 2
	var captures []ClockCapture
	for _, raw := range client.Events {
		var e EvidenceEvent
		if json.Unmarshal(raw, &e) == nil && e.Type == "clock_capture" {
			var c ClockCapture
			if json.Unmarshal(e.Details, &c) == nil {
				captures = append(captures, c)
			}
		}
	}
	compatible := reconciliation.Status == "reconciled"
	queries := oneWayQueries(client.Evidence, server.Evidence, compatible && reconciliation.Client.Summary.Received != nil, true)
	queries = append(queries, oneWayQueries(server.Evidence, client.Evidence, compatible && reconciliation.Server.Summary.Received != nil, false)...)
	sort.SliceStable(queries, func(i, j int) bool { // Client-local ordering prevents future samples correcting earlier data.
		return queries[i].at < queries[j].at
	})
	window := clockWindow{policy: p.Calibration, recent: make([]ClockCapture, 0, p.Calibration.Window)}
	var model *calibration.Model
	var revision ClockRevision
	retained := false
	next := 0
	advance := func(c ClockCapture) { // Freeze every capture-triggered window, including invalid attempts/evictions.
		window.add(c)
		revision = ClockRevision{ID: fmt.Sprintf("model-%d", window.totals.Attempts), At: c.Finished, Expiry: p.Calibration.Expiry, Status: "unavailable", Reason: "insufficient recent valid clock samples", CalibrationIDs: []string{}}
		model = nil
		var samples []calibration.Sample
		for _, item := range window.recent {
			if item.Valid && item.Finished <= c.Finished && item.T4 <= c.Finished && c.Finished-item.T4 <= p.Calibration.Expiry {
				samples = append(samples, calibration.Sample{ID: item.ID, ClientEpoch: item.ClientEpoch, ServerEpoch: item.ServerEpoch, T1: item.T1, T2: item.T2, T3: item.T3, T4: item.T4})
				revision.CalibrationIDs = append(revision.CalibrationIDs, item.ID)
			}
		}
		if len(samples) >= p.Calibration.Samples {
			revision.First, revision.Last = samples[0].T1, samples[len(samples)-1].T4
			revision.ClientEpoch, revision.ServerEpoch = samples[0].ClientEpoch, samples[0].ServerEpoch
			var err error
			model, err = calibration.Build(samples, policy)
			if err != nil {
				revision.Reason = err.Error()
			} else {
				revision.Status, revision.Reason = "envelope_available", "all retained valid constraints intersected; verification and error gates apply per result"
			}
		}
		retained = retainOneWay(out, revision)
		if retained {
			out.Models = append(out.Models, revision)
		} else {
			out.OmittedModels++
		}
	}
	for _, q := range queries {
		now := q.at
		for next < len(captures) && captures[next].Finished <= now {
			advance(captures[next])
			next++
		}
		v := q.result
		if q.matched {
			v.Interval.Reason = "no causally available clock model"
			if revision.ID != "" {
				v.ModelID, v.Interval.CalibrationIDs = revision.ID, append([]string{}, revision.CalibrationIDs...)
				v.Interval.Reason = revision.Reason
			}
			if model != nil {
				age := now - revision.Last
				v.Age = &age
				if window.snapshot(now).Status != "samples_available" {
					v.Interval.Reason = "insufficient unexpired clock samples"
				} else {
					v.Interval = model.Delay(uint64(*v.ClientCapture), uint64(*v.ServerCapture), uint64(now), client.Evidence.Epoch, server.Evidence.Epoch, v.Direction == 1)
				}
			}
		}
		switch v.Interval.Status {
		case "calibrated":
			out.Calibrated++
		case "estimate":
			out.Estimated++
		default:
			out.Unavailable++
		}
		for i := range out.Directions {
			if out.Directions[i].Direction == v.Direction {
				switch v.Interval.Status {
				case "calibrated":
					out.Directions[i].Calibrated++
				case "estimate":
					out.Directions[i].Estimated++
				default:
					out.Directions[i].Unavailable++
				}
			}
		}
		if (v.ModelID == "" || retained) && retainOneWay(out, v) {
			out.Results = append(out.Results, v)
		} else {
			out.OmittedResults++
		}
	}
	for next < len(captures) {
		advance(captures[next])
		next++
	}
	if out.Calibrated > 0 || out.Estimated > 0 {
		out.Status, out.Reason = "derived", "per-result uncertainty states; estimates excluded from directional timeliness/comparison claims"
	}
	return out
}
