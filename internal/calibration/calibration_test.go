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
