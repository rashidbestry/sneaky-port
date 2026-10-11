package profile_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// TestProfilePrecisionAndHash checks normalization and canonical integer-string handling.
func TestProfilePrecisionAndHash(t *testing.T) {
	p := testutil.Plan()
	p.MatchedMetadata["note"] = "<Unicode \u2028>"
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := profile.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	h1, _ := p.Hash()
	h2, _ := got.Hash()
	if h1 != h2 {
		t.Fatal("unstable plan hash")
	}
	var d profile.Decimal
	if err = json.Unmarshal([]byte(`"18446744073709551615"`), &d); err != nil || uint64(d) != ^uint64(0) {
		t.Fatal("lost integer precision")
	}
	if err = json.Unmarshal([]byte(`9007199254740993`), &d); err == nil {
		t.Fatal("numeric 64-bit field accepted")
	}
}

// TestProfileAmbiguity rejects duplicate, aliased, tagged, incomplete and unknown input.
func TestProfileAmbiguity(t *testing.T) {
	p := testutil.Plan()
	b, _ := json.Marshal(p)
	valid := string(b)
	cases := []string{strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1), strings.Replace(valid, `,"dscp":0`, ``, 1), strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"bogus":0`, 1), valid + "\n---\n{}", strings.Replace(valid, `"rate_numerator":"1000"`, `"rate_numerator":1000`, 1), "schema_version: !!int 1", "profile_id: &id fixture\npairing_ref: *id", "profile_id: fixture\n<<: {schema_version: 1}", strings.Repeat("x", profile.DocumentMax+1)}
	cases = append(cases, strings.Replace(valid, `"phase_id":"main"`, `"phase_id":"main","deadline_ns":null`, 1))
	for i, s := range cases {
		if _, err := profile.Decode(strings.NewReader(s)); err == nil {
			t.Fatalf("accepted malformed profile %d", i)
		}
	}
}

// TestProfileBudgets rejects infeasible schedules before allocating ledger entries or sockets.
func TestProfileBudgets(t *testing.T) {
	p := testutil.Plan()
	p.Phases[0].Client.RateNumerator = 1e9
	p.Phases[0].Duration = 1e9
	if err := p.Validate(); err == nil {
		t.Fatal("ledger overflow accepted")
	}
	p = testutil.Plan()
	p.Limits.QueueBytes = 248
	if err := p.Validate(); err == nil {
		t.Fatal("receipt reserve byte demand ignored")
	}
	p = testutil.Plan()
	p.Phases[0].Server.DSCP = 64
	if err := p.Validate(); err == nil {
		t.Fatal("invalid marking accepted")
	}
}

// TestCanonicalKnownVector checks sorting, minimal Unicode escapes and safe-number rejection.
func TestCanonicalKnownVector(t *testing.T) {
	v := map[string]any{"z": "<\u2028>", "a": 1}
	b, err := profile.Canonical(v)
	if err != nil || string(b) != "{\"a\":1,\"z\":\"<\u2028>\"}" {
		t.Fatalf("canonical: %q %v", b, err)
	}
	if _, err = profile.Canonical(map[string]any{"a": 1.5}); err == nil {
		t.Fatal("fractional canonical plan accepted")
	}
	if _, err = profile.Canonical(map[string]any{"a": string([]byte{0xff})}); err == nil {
		t.Fatal("invalid UTF-8 silently replaced in plan hash")
	}
}
