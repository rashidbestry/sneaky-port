package profile_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"sneaky-port/internal/profile"
)

// TestDefaultProfile validates a fully materialized starter and its exact persisted/replayed representation.
func TestDefaultProfile(t *testing.T) {
	p := profile.Default()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := profile.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	before, err := p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	after, err := decoded.Hash()
	if err != nil || before != after {
		t.Fatal("starter plan changed across strict serialization", err)
	}
	p.Phases[0].Client.DSCP = 42
	p.Options["changed"] = json.RawMessage("true")
	p.MatchedMetadata["changed"] = "edited"
	p.ComparisonAxes = append(p.ComparisonAxes, "port")
	next, err := profile.Default().Hash()
	if err != nil || next != before {
		t.Fatal("editing one starter mutated later startup settings", err)
	}
}
