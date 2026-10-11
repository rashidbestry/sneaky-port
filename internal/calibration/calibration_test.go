package calibration

import (
	"math/big"
	"testing"

	"sneaky-port/internal/profile"
)

// samplesFor creates known-offset/drift asymmetric transit captures using exact rationals.
func samplesFor(a int64, b *big.Rat, up, down uint64) []Sample {
	var out []Sample
	for i, c := range []uint64{1000000000, 2000000000, 3000000000} {
		s := new(big.Rat).Add(big.NewRat(a, 1), new(big.Rat).Mul(b, integer(c)))
		s.Add(s, integer(up))
		t2 := ceil(s).Uint64()
		t3 := t2 + 1000
		c4 := new(big.Rat).Quo(new(big.Rat).Sub(integer(t3+down), big.NewRat(a, 1)), b)
		out = append(out, Sample{ID: string(rune('a' + i)), ClientEpoch: 1, ServerEpoch: 2, T1: profile.Decimal(c), T2: profile.Decimal(t2), T3: profile.Decimal(t3), T4: profile.Decimal(ceil(c4).Uint64())})
	}
	return out
}

// TestAsymmetricEnvelope checks injected directional truth lies inside retained uncertainty.
func TestAsymmetricEnvelope(t *testing.T) {
	for _, b := range []*big.Rat{big.NewRat(1, 1), big.NewRat(100000001, 100000000)} {
		samples := samplesFor(5000000, b, 10000, 90000)
		m, err := Build(samples, Policy{DriftPPB: 100, TimestampError: 1, Expiry: 1e9, MaxError: 1e6, Source: "known synthetic clock fixture", Verified: true})
		if err != nil {
			t.Fatal(err)
		}
		client := uint64(samples[2].T1)
		server := ceil(new(big.Rat).Add(big.NewRat(5010000, 1), new(big.Rat).Mul(b, integer(client)))).Uint64()
		got := m.Delay(client, server, uint64(samples[2].T4), 1, 2, true)
		if got.Status != "calibrated" || got.Lower == nil {
			t.Fatalf("interval %+v", got)
		}
		lo, _ := new(big.Int).SetString(*got.Lower, 10)
		hi, _ := new(big.Int).SetString(*got.Upper, 10)
		if lo.Cmp(big.NewInt(10000)) > 0 || hi.Cmp(big.NewInt(10001)) < 0 {
			t.Fatalf("truth excluded %+v", got)
		}
		if got = m.Delay(client, server, uint64(samples[2].T4)+1e9+1, 1, 2, true); got.Status != "unavailable" || got.Midpoint != nil {
			t.Fatal("expired value exposed")
		}
		if got = m.Delay(client, server, uint64(samples[2].T4), 9, 2, true); got.Status != "unavailable" {
			t.Fatal("foreign epoch exposed")
		}
	}
}

// TestEstimateAndInconsistent ensures assumptions cannot silently become calibrated evidence.
func TestEstimateAndInconsistent(t *testing.T) {
	s := samplesFor(1000, big.NewRat(1, 1), 10, 90)
	p := Policy{Expiry: 1e9, MaxError: 1, Source: "unverified assumptions"}
	m, err := Build(s, p)
	if err != nil {
		t.Fatal(err)
	}
	got := m.Delay(uint64(s[2].T1), uint64(s[2].T2), uint64(s[2].T4), 1, 2, true)
	if got.Status != "estimate" {
		t.Fatalf("assumptions promoted %+v", got)
	}
	s[1].T2 += 1000000
	s[1].T3 += 1000000
	if _, err = Build(s, p); err == nil {
		t.Fatal("inconsistent retained sample ignored")
	}
}

// TestDelayCaptureBounds verifies rate-scaled timestamp error, signed intervals and both capture lifetimes.
// A single exact synthetic mapping isolates propagation from uncertainty in the fitted envelope.
func TestDelayCaptureBounds(t *testing.T) {
	m := &Model{vertices: []point{{big.NewRat(500, 1), big.NewRat(2, 1)}}, first: 1, last: 1000, clientEpoch: 1, serverEpoch: 2, IDs: []string{"known"}, policy: Policy{TimestampError: 10, ModelError: 5, Expiry: 1000, MaxError: 100, Source: "exact fixture", Verified: true}}
	up := m.Delay(1100, 2700, 1100, 1, 2, true)
	down := m.Delay(1100, 2700, 1100, 1, 2, false)
	if up.Status != "calibrated" || *up.Lower != "-35" || *up.Upper != "35" || *down.Lower != "-18" || *down.Upper != "18" || *down.Uncertainty != "18" {
		t.Fatalf("rate-scaled error or signed interval lost: %+v %+v", up, down)
	}
	negative := m.Delay(1100, 2000, 1100, 1, 2, true)
	if negative.Status != "unavailable" || negative.Midpoint != nil || negative.Reason != "wholly negative delay envelope" {
		t.Fatal("impossible negative value exposed", negative)
	}
	if remote := m.Delay(1100, 5100, 1100, 1, 2, true); remote.Status != "unavailable" || remote.Midpoint != nil || remote.Reason != "remote capture outside clock validity envelope" {
		t.Fatal("remote capture outlived clock envelope", remote)
	}
	if boundary := m.Delay(1900, 4300, 2000, 1, 2, true); boundary.Status != "calibrated" {
		t.Fatal("exact expiry boundary rejected", boundary)
	}
	if expired := m.Delay(1900, 4300, 2001, 1, 2, true); expired.Status != "unavailable" || expired.Lower != nil {
		t.Fatal("strict expiry failed", expired)
	}
}

// TestDelayExactLargeOffsets preserves nanoseconds beyond browser precision without float conversion.
func TestDelayExactLargeOffsets(t *testing.T) {
	const base uint64 = 9007199254740993
	m := &Model{vertices: []point{{big.NewRat(500, 1), big.NewRat(1, 1)}}, first: base, last: base + 1000, clientEpoch: 1, serverEpoch: 1, IDs: []string{"large"}, policy: Policy{Expiry: 1000, MaxError: 1, Source: "exact fixture", Verified: true}}
	got := m.Delay(base+1100, base+1610, base+1100, 1, 1, true)
	if got.Status != "calibrated" || *got.Lower != "10" || *got.Upper != "10" || *got.Uncertainty != "0" {
		t.Fatal("large-offset precision lost", got)
	}
}
