package control

import (
	"encoding/json"
	"math/big"
	"reflect"
	"testing"

	"sneaky-port/internal/calibration"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// clockEvent encodes an already validated synthetic raw exchange for causal report fixtures.
func clockEvent(c ClockCapture) json.RawMessage {
	details, _ := json.Marshal(c)
	b, _ := json.Marshal(EvidenceEvent{Type: "clock_capture", At: c.Finished, Details: details})
	return b
}

// oneWayFixture supplies exact known offset/drift/asymmetry without changing either system clock.
// Data uses authentic origin/receiver identities; separate real SSH fixtures test raw-file validation.
func oneWayFixture(drift uint64) (*ReplayedReport, *ReplayedReport, calibration.Policy) {
	a, b := reconcileFixture()
	c := profile.Calibration{Samples: 2, Window: 4, Cadence: 1000000, Expiry: 10000000, MaxError: 1000000, TimestampError: 1, ModelError: 1, DriftPPB: profile.Decimal(drift + 100), BoundSource: "independently known synthetic clock fixture"}
	a.Manifest.Plan.Calibration, b.Manifest.Plan.Calibration = c, c
	a.Evidence.Entries, b.Evidence.Entries = nil, nil
	a.Evidence.Records, b.Evidence.Records = nil, nil
	a.Evidence.ObservationCutoff, b.Evidence.ObservationCutoff = 20000000, 20000000
	a.Evidence.Receiver.Cutoff, b.Evidence.Receiver.Cutoff = 20000000, 20000000
	scale := uint64(1000000000) + drift
	for i, at := range []uint64{1000000, 2000000, 3000000} {
		t2 := 500000 + at*scale/1000000000 + 10000
		t3 := t2 + 1000
		t4 := ((t3+90000-500000)*1000000000 + scale - 1) / scale
		a.Events = append(a.Events, clockEvent(ClockCapture{ID: decimalInt(i + 1), ClientEpoch: 1, ServerEpoch: 1, T1: profile.Decimal(at), T2: profile.Decimal(t2), T3: profile.Decimal(t3), T4: profile.Decimal(t4), Finished: profile.Decimal(t4 + 2000), Valid: true}))
	}
	addOneWayDatum(a, b, 0, 500000, 500000+500000*scale/1000000000+10000)
	addOneWayDatum(a, b, 1, 3500000, 500000+3500000*scale/1000000000+10000)
	addOneWayDatum(b, a, 0, 500000+3600000*scale/1000000000, 3600000+(90000*1000000000+scale-1)/scale)
	p := calibration.Policy{DriftPPB: uint64(c.DriftPPB), TimestampError: 1, ModelError: 1, Expiry: uint64(c.Expiry), MaxError: uint64(c.MaxError), Source: c.BoundSource, Verified: true}
	return a, b, p
}

// addOneWayDatum preserves an original attempt and complete peer arrival at independent local cutoffs.
func addOneWayDatum(origin, peer *ReplayedReport, sequence, attempt, arrival uint64) {
	id := measure.Identity{Lifecycle: 2, Sequence: profile.Decimal(sequence)}
	at, written := profile.Decimal(attempt), profile.Decimal(attempt+100)
	origin.Evidence.Entries = append(origin.Evidence.Entries, measure.Entry{Identity: id, Attempt: &at, WriteCompleted: &written, State: "written", Admission: "accepted", Deadline: at + 100000})
	peer.Evidence.Receiver.Arrivals[id] = arrival
	raw, _ := json.Marshal(engine.Record{Kind: "arrival", FrameKind: probe.Data, Identity: id, Direction: origin.Evidence.Direction, Epoch: origin.Evidence.Epoch, Attempt: at, At: profile.Decimal(arrival)})
	peer.Evidence.Records = append(peer.Evidence.Records, raw)
}

// assertOneWayTruth checks outward decimal interval bounds contain the independently injected delay.
func assertOneWayTruth(t *testing.T, v OneWayResult, truth uint64) {
	t.Helper()
	if v.Interval.Status != "calibrated" || v.Interval.Lower == nil || v.Interval.Upper == nil || v.Age == nil || v.ModelID == "" || len(v.Interval.CalibrationIDs) < 2 {
		t.Fatalf("missing calibrated interval: %+v", v)
	}
	lo, _ := new(big.Int).SetString(*v.Interval.Lower, 10)
	hi, _ := new(big.Int).SetString(*v.Interval.Upper, 10)
	x := new(big.Int).SetUint64(truth)
	if lo.Cmp(x) > 0 || hi.Cmp(x) < 0 {
		t.Fatalf("truth %d outside %+v", truth, v)
	}
}

// TestOneWayKnownClocks covers both physical directions, nonzero drift and strongly asymmetric paths.
func TestOneWayKnownClocks(t *testing.T) {
	for _, drift := range []uint64{0, 100000} {
		a, b, policy := oneWayFixture(drift)
		r := reconcileReports(a, b)
		if r.Status != "reconciled" || r.OneWay.Estimated != 2 || r.OneWay.Calibrated != 0 || r.OneWay.BoundsVerified {
			t.Fatalf("production source label promoted bounds: %+v", r.OneWay)
		}
		before := localSnapshot(a.Evidence)
		got := deriveOneWay(a, b, r, policy)
		if got.Calibrated != 2 || got.Unavailable != 1 || got.Results[0].Interval.Midpoint != nil || got.Results[0].ModelID != "" {
			t.Fatalf("future samples upgraded early data: %+v", got)
		}
		assertOneWayTruth(t, got.Results[1], 10000)
		assertOneWayTruth(t, got.Results[2], (90000*1000000000+1000000000+drift-1)/(1000000000+drift))
		if !reflect.DeepEqual(before, localSnapshot(a.Evidence)) {
			t.Fatal("directional derivation mutated local RTT/delivery")
		}
		policy.MaxError = 1
		if wide := deriveOneWay(a, b, r, policy); wide.Calibrated != 0 || wide.Estimated != 2 {
			t.Fatal("exceed-policy intervals promoted", wide)
		}
	}
}

// TestOneWayCausalFaults covers invalid/inconsistent/foreign/expired clocks and unavailable remote evidence.
func TestOneWayCausalFaults(t *testing.T) {
	for _, kind := range []string{"inconsistent", "epoch", "invalid", "expired", "partial", "missing", "future"} {
		t.Run(kind, func(t *testing.T) { // Inject evidence faults only, preserving real host/service settings.
			a, b, policy := oneWayFixture(0)
			baseline := reconcileReports(a, b).OneWay
			var e EvidenceEvent
			json.Unmarshal(a.Events[2], &e)
			var c ClockCapture
			json.Unmarshal(e.Details, &c)
			switch kind {
			case "inconsistent":
				c.T2 += 1000000
				c.T3 += 1000000
			case "epoch":
				c.ServerEpoch++
			case "invalid":
				a.Manifest.Plan.Calibration.Window = 2
				c.Valid, c.Reason = false, "incomplete response"
			case "expired":
				a.Manifest.Plan.Calibration.Expiry, policy.Expiry = 100000, 100000
			case "partial":
				b.Status = "partial"
			case "missing":
				b.Evidence.Records = nil
				b.Evidence.Receiver.Arrivals = map[measure.Identity]uint64{}
			case "future":
				c.ID, c.T1, c.T4, c.Finished = "later", 4000000, 4101000, 4103000
				c.T2, c.T3 = 5510000, 5511000
				a.Events = append(a.Events, clockEvent(c))
			}
			if kind == "inconsistent" || kind == "epoch" || kind == "invalid" {
				a.Events[2] = clockEvent(c)
			}
			b.Manifest.Plan = a.Manifest.Plan
			r := reconcileReports(a, b)
			got := deriveOneWay(a, b, r, policy)
			if kind == "future" {
				if !reflect.DeepEqual(baseline.Results, r.OneWay.Results) || got.Models[len(got.Models)-1].Status != "unavailable" {
					t.Fatal("later clock fault changed old intervals")
				}
			} else if kind == "missing" {
				if got.Calibrated != 1 || got.Unavailable != 2 || got.Results[2].Direction != 2 {
					t.Fatal("missing upstream evidence invalidated sound downstream timing")
				}
			} else if got.Calibrated != 0 || got.Estimated != 0 {
				t.Fatalf("%s exposed one-way value: %+v", kind, got)
			}
		})
	}
}

// TestOneWayBoundedDetails verifies exact populations survive deterministic derived-detail exhaustion.
func TestOneWayBoundedDetails(t *testing.T) {
	a, b, _ := oneWayFixture(0)
	r := reconcileReports(a, b)
	a.Manifest.Plan.Limits.EventBytes = 1
	got := productionOneWay(a, b, r)
	if got.DetailBytes > got.DetailBudget || got.OmittedModels != 3 || got.OmittedResults != 3 || got.Estimated != 2 || got.Unavailable != 1 || len(got.Results) != 0 || len(got.Models) != 0 || got.Directions[0].Estimated != 1 || got.Directions[0].Unavailable != 1 || got.Directions[1].Estimated != 1 {
		t.Fatalf("detail cap concealed population: %+v", got)
	}
}

// TestOneWayRecovery preserves failed old revisions while invalid samples age out of the bounded window.
func TestOneWayRecovery(t *testing.T) {
	a, b, policy := oneWayFixture(0)
	a.Manifest.Plan.Calibration.Window = 2
	b.Manifest.Plan = a.Manifest.Plan
	var e EvidenceEvent
	json.Unmarshal(a.Events[2], &e)
	var c ClockCapture
	json.Unmarshal(e.Details, &c)
	c.Valid, c.Reason = false, "blocked SSH response"
	a.Events[2] = clockEvent(c)
	for i, at := range []profile.Decimal{4000000, 5000000} {
		c.ID, c.Valid, c.Reason = decimalInt(i+4), true, ""
		c.T1, c.T2, c.T3, c.T4, c.Finished = at, at+510000, at+511000, at+101000, at+103000
		a.Events = append(a.Events, clockEvent(c))
	}
	addOneWayDatum(a, b, 2, 5500000, 6010000)
	r := reconcileReports(a, b)
	got := deriveOneWay(a, b, r, policy)
	if got.Calibrated != 1 || got.Unavailable != 3 || got.Models[2].Status != "unavailable" || got.Models[3].Status != "unavailable" || got.Models[4].Status != "envelope_available" || got.Results[1].Interval.Midpoint != nil {
		t.Fatalf("bounded recovery changed failed historical values: %+v", got)
	}
	assertOneWayTruth(t, got.Results[3], 10000)
}
