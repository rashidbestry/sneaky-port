package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"sneaky-port/internal/profile"
)

// assertReplayAccounting checks all ledgers, phase populations and timing episodes against saved original records.
// It reuses arbitrary deterministic start offsets; finite persisted runs use the zero local anchor.
func assertReplayAccounting(t *testing.T, p profile.Plan, start uint64, original Result) {
	t.Helper()
	c, err := NewCollector(p, original.Direction, start)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range original.Records {
		var r Record
		if err = json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		if err = c.ReplayRecord(r); err != nil {
			t.Fatal(err)
		}
	}
	replayed := c.ReplayFinish(original.GenerationCutoff, original.ObservationCutoff, original.Complete)
	// Original sockets contribute run/epoch labels after accounting; replay has no transport identity objects.
	replayed.Run, replayed.Session, replayed.Epoch, replayed.PeerEpoch = original.Run, original.Session, original.Epoch, original.PeerEpoch
	if !reflect.DeepEqual(replayed, original) {
		t.Fatalf("replay differs:\n%+v\n%+v", replayed, original)
	}
}
