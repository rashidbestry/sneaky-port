package control

import (
	"context"
	"encoding/json"
	"testing"
)

// TestFiniteLinuxMarkingTransitions uses actual owned Linux sockets and API readback, not packet capture.
func TestFiniteLinuxMarkingTransitions(t *testing.T) {
	for _, kind := range []string{"udp", "tcp", "udp6", "tcp6"} {
		t.Run(kind, func(t *testing.T) { // Apply independent physical directions and restore baseline in the second phase.
			r := finiteRig(t, kind)
			r.plan.Phases[0].Client.DSCP, r.plan.Phases[0].Server.DSCP = 46, 12
			ph := r.plan.Phases[0]
			ph.ID, ph.Client.DSCP, ph.Server.DSCP = "baseline", 0, 0
			r.plan.Phases = append(r.plan.Phases, ph)
			result, err := Run(context.Background(), r.client, r.signer, r.plan)
			if err != nil || result.Local == nil || !result.Local.Complete {
				t.Fatalf("Linux marking run: %v", err)
			}
			marks := []int{}
			for _, data := range result.Evidence.Records {
				var record struct {
					Kind    string
					Marking struct {
						Requested int
						Applied   *int
						Observed  *int
					}
				}
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				if record.Kind == "marking" {
					if record.Marking.Applied == nil || *record.Marking.Applied != record.Marking.Requested || record.Marking.Observed != nil {
						t.Fatal("API/observation evidence conflated")
					}
					marks = append(marks, record.Marking.Requested)
				}
			}
			if len(marks) != 3 || marks[0] != 46 || marks[1] != 46 || marks[2] != 0 {
				t.Fatal("Linux phase/baseline transitions missing:", marks)
			}
		})
	}
}
