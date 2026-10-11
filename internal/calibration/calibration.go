// Package calibration bounds affine clock mappings without assuming symmetric paths.
package calibration

import (
	"errors"
	"math/big"

	"sneaky-port/internal/profile"
)

// Sample retains four host-local captures and their monotonic epoch identities.
type Sample struct {
	ID                       string `json:"calibration_id"`
	ClientEpoch, ServerEpoch uint32
	T1, T2, T3, T4           profile.Decimal
}

// Policy declares bounds; Verified requires independent evidence, not merely a source label.
type Policy struct {
	DriftPPB, TimestampError, ModelError, Expiry, MaxError uint64
	Source                                                 string
	Verified                                               bool
}

// point is one exact feasible affine mapping S(c)=a+b*c.
type point struct{ a, b *big.Rat }

// Model retains the full feasible offset/drift envelope and explicit validity limits.
type Model struct {
	vertices                 []point
	policy                   Policy
	clientEpoch, serverEpoch uint32
	first, last              uint64
	IDs                      []string
}

// integer constructs an exact rational without converting through floating point.
func integer(v uint64) *big.Rat { return new(big.Rat).SetInt(new(big.Int).SetUint64(v)) }

// Build intersects every nonnegative-transit constraint with the declared drift band.
func Build(samples []Sample, p Policy) (*Model, error) {
	if len(samples) < 2 || len(samples) > 4096 || p.DriftPPB >= 1e9 || p.Expiry == 0 || p.MaxError == 0 || p.Source == "" {
		return nil, errors.New("invalid calibration samples/policy")
	}
	m := &Model{policy: p, clientEpoch: samples[0].ClientEpoch, serverEpoch: samples[0].ServerEpoch, first: uint64(samples[0].T1), last: uint64(samples[0].T4)}
	if m.clientEpoch == 0 || m.serverEpoch == 0 {
		return nil, errors.New("missing clock epochs")
	}
	lower := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).SetFrac(new(big.Int).SetUint64(p.DriftPPB), big.NewInt(1e9)))
	upper := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).SetFrac(new(big.Int).SetUint64(p.DriftPPB), big.NewInt(1e9)))
	// Client capture error is scaled by the largest feasible rate; server error is already server-local.
	errBound := new(big.Rat).Add(new(big.Rat).Mul(integer(p.TimestampError), new(big.Rat).Add(big.NewRat(1, 1), upper)), integer(p.ModelError))
	s := samples[0]
	aMin := new(big.Rat).Sub(new(big.Rat).Sub(integer(uint64(s.T3)), new(big.Rat).Mul(upper, integer(uint64(s.T4)))), errBound)
	aMax := new(big.Rat).Add(new(big.Rat).Sub(integer(uint64(s.T2)), new(big.Rat).Mul(lower, integer(uint64(s.T1)))), errBound)
	if aMin.Cmp(aMax) > 0 {
		return nil, errors.New("inconsistent initial clock constraints")
	}
	m.vertices = []point{{new(big.Rat).Set(aMin), new(big.Rat).Set(lower)}, {new(big.Rat).Set(aMax), new(big.Rat).Set(lower)}, {new(big.Rat).Set(aMax), new(big.Rat).Set(upper)}, {new(big.Rat).Set(aMin), new(big.Rat).Set(upper)}}
	seen := map[string]bool{}
	var previous uint64
	for i, s := range samples {
		if s.ID == "" || seen[s.ID] || s.ClientEpoch != m.clientEpoch || s.ServerEpoch != m.serverEpoch || s.T1 > s.T4 || s.T2 > s.T3 || i > 0 && uint64(s.T1) <= previous {
			return nil, errors.New("invalid, reordered or mixed-epoch calibration sample")
		}
		seen[s.ID] = true
		previous = uint64(s.T1)
		m.IDs = append(m.IDs, s.ID)
		if uint64(s.T4) > m.last {
			m.last = uint64(s.T4)
		}
		m.vertices = clip(m.vertices, big.NewRat(1, 1), integer(uint64(s.T1)), new(big.Rat).Add(integer(uint64(s.T2)), errBound))
		m.vertices = clip(m.vertices, big.NewRat(-1, 1), new(big.Rat).Neg(integer(uint64(s.T4))), new(big.Rat).Add(new(big.Rat).Neg(integer(uint64(s.T3))), errBound))
		if len(m.vertices) == 0 {
			return nil, errors.New("inconsistent clock constraints")
		}
	}
	return m, nil
}

// clip intersects a convex polygon with A*a+B*b<=C, preserving exact boundary points.
func clip(vertices []point, a, b, c *big.Rat) []point {
	if len(vertices) == 0 {
		return nil
	}
	var out []point
	for i, current := range vertices {
		previous := vertices[(i+len(vertices)-1)%len(vertices)]
		cv, pv := distance(current, a, b, c), distance(previous, a, b, c)
		cin, pin := cv.Sign() <= 0, pv.Sign() <= 0
		if cin != pin {
			t := new(big.Rat).Quo(pv, new(big.Rat).Sub(pv, cv))
			out = append(out, point{interpolate(previous.a, current.a, t), interpolate(previous.b, current.b, t)})
		}
		if cin {
			out = append(out, current)
		}
	}
	return out
}

// distance computes a signed constraint residual without mutating retained vertices.
func distance(p point, a, b, c *big.Rat) *big.Rat {
	return new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(a, p.a), new(big.Rat).Mul(b, p.b)), c)
}

// interpolate computes one exact edge intersection coordinate.
func interpolate(a, b, t *big.Rat) *big.Rat {
	return new(big.Rat).Add(a, new(big.Rat).Mul(t, new(big.Rat).Sub(b, a)))
}

// Interval preserves signed nanosecond bounds; unavailable values remain null.
type Interval struct {
	Status         string   `json:"status"`
	Reason         string   `json:"reason"`
	Lower          *string  `json:"lower_ns"`
	Upper          *string  `json:"upper_ns"`
	Midpoint       *string  `json:"midpoint_ns"`
	Uncertainty    *string  `json:"uncertainty_ns"`
	CalibrationIDs []string `json:"calibration_ids"`
}

// Delay evaluates directional delay over every feasible mapping, never a symmetry shortcut.
func (m *Model) Delay(client, server, now uint64, clientEpoch, serverEpoch uint32, upstream bool) Interval {
	out := Interval{Status: "unavailable", Reason: "invalid/expired clock epoch", CalibrationIDs: append([]string(nil), m.IDs...)}
	if clientEpoch != m.clientEpoch || serverEpoch != m.serverEpoch || now < m.last || now-m.last > m.policy.Expiry || client < m.first || client > now || len(m.vertices) == 0 {
		return out
	}
	var lo, hi *big.Rat
	for _, p := range m.vertices {
		// Both actual captures must fit the declared mapping lifetime under every feasible clock.
		remoteClient := new(big.Rat).Quo(new(big.Rat).Sub(integer(server), p.a), p.b)
		validEnd := new(big.Rat).Add(integer(m.last), integer(m.policy.Expiry))
		if remoteClient.Cmp(integer(m.first)) < 0 || remoteClient.Cmp(validEnd) > 0 {
			out.Reason = "remote capture outside clock validity envelope"
			return out
		}
		var value *big.Rat
		if upstream {
			value = new(big.Rat).Sub(integer(server), new(big.Rat).Add(p.a, new(big.Rat).Mul(p.b, integer(client))))
		} else {
			mapped := new(big.Rat).Quo(new(big.Rat).Sub(integer(server), p.a), p.b)
			value = new(big.Rat).Sub(integer(client), mapped)
		}
		if lo == nil || value.Cmp(lo) < 0 {
			lo = new(big.Rat).Set(value)
		}
		if hi == nil || value.Cmp(hi) > 0 {
			hi = new(big.Rat).Set(value)
		}
	}
	// Propagate endpoint capture and declared between-sample model uncertainty in each direction.
	minB, maxB := m.vertices[0].b, m.vertices[0].b
	for _, p := range m.vertices {
		if p.b.Cmp(minB) < 0 {
			minB = p.b
		}
		if p.b.Cmp(maxB) > 0 {
			maxB = p.b
		}
	}
	errorBound := new(big.Rat).Add(new(big.Rat).Mul(integer(m.policy.TimestampError), new(big.Rat).Add(big.NewRat(1, 1), maxB)), integer(m.policy.ModelError))
	if !upstream {
		errorBound.Quo(errorBound, minB)
	}
	lo.Sub(lo, errorBound)
	hi.Add(hi, errorBound)
	if hi.Sign() < 0 {
		out.Reason = "wholly negative delay envelope"
		return out
	}
	mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
	width := new(big.Rat).Quo(new(big.Rat).Sub(hi, lo), big.NewRat(2, 1))
	l, h, v, u := floor(lo).String(), ceil(hi).String(), floor(mid).String(), ceil(width).String()
	out.Lower, out.Upper, out.Midpoint, out.Uncertainty = &l, &h, &v, &u
	out.Status = "estimate"
	out.Reason = "clock/timestamp/model bounds require independent validation"
	if m.policy.Verified && width.Cmp(integer(m.policy.MaxError)) <= 0 {
		out.Status = "calibrated"
		out.Reason = "independently verified bounds meet explicit error policy"
	} else if width.Cmp(integer(m.policy.MaxError)) > 0 {
		out.Reason = "uncertainty exceeds explicit maximum"
	}
	return out
}

// floor rounds a signed rational outward toward negative infinity.
func floor(r *big.Rat) *big.Int {
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if r.Sign() < 0 && rem.Sign() != 0 {
		q.Sub(q, big.NewInt(1))
	}
	return q
}

// ceil rounds a signed rational outward toward positive infinity.
func ceil(r *big.Rat) *big.Int {
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if r.Sign() > 0 && rem.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}
