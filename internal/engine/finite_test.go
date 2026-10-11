package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/platform"
	"sneaky-port/internal/testutil"
	"sneaky-port/internal/worker"
)

// markingFixture replaces only the marking API to test phase selection without pretending to capture packets.
type markingFixture struct {
	adapter.Transport
	values []int
	fail   bool
}

// Mark retains exact requests, including nonzero-to-zero transitions, with deterministic API-only evidence.
func (t *markingFixture) Mark(value int) (platform.Marking, error) {
	t.values = append(t.values, value)
	m := platform.Marking{Requested: value, Applied: &value, Status: "synthetic_api", Reason: "fixture only; outer packets unobserved"}
	if t.fail {
		m.Applied = nil
		return m, errors.New("injected marking rejection")
	}
	return m, nil
}

// Interrupt forwards real socket cutoff interruption while preserving control-owned transport lifetime.
func (t *markingFixture) Interrupt() error {
	return t.Transport.(interface{ Interrupt() error }).Interrupt()
}

// TestFinitePhaseWrites connects changing API markings to actual FIFO frames/receipts and finite cutoffs.
// Nonzero marking success is synthetic here; the carrier I/O is real IPv4 loopback.
func TestFinitePhaseWrites(t *testing.T) {
	for _, kind := range []string{"udp", "tcp"} {
		for _, reject := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/phases", true: "/rejection"}[reject], func(t *testing.T) { // Validate transitions and explicit marking failure before attempt timestamps.
				p := testutil.Plan()
				p.Adapter = kind
				if kind == "tcp" {
					p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
				}
				p.Warmup, p.Grace, p.Deadline = 40e6, 150e6, 100e6
				p.Phases[0].Duration = 80e6
				p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 50, 25
				p.Phases[0].Client.DSCP, p.Phases[0].Server.DSCP = 46, 12
				ph := p.Phases[0]
				ph.ID, ph.Client.DSCP, ph.Server.DSCP = "second", 0, 34
				p.Phases = append(p.Phases, ph)
				p.Limits.MaxLag = 30e6
				c, s, p := realPair(t, p)
				clientMark := &markingFixture{Transport: c.Transport, fail: reject}
				serverMark := &markingFixture{Transport: s.Transport}
				c.Transport, s.Transport = clientMark, serverMark
				client, err := NewFinite(context.Background(), c, p, false)
				if err != nil {
					t.Fatal(err)
				}
				server, err := NewFinite(context.Background(), s, p, true)
				if err != nil {
					client.Cancel()
					t.Fatal(err)
				}
				t.Cleanup(func() { // Join both endpoints before assertions release the fixture sockets.
					client.Cancel()
					server.Cancel()
					client.Wait(context.Background())
					server.Wait(context.Background())
				})
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				local, err := client.Wait(ctx)
				if reject {
					if err == nil || local.Complete || local.Summary.Attempted != 0 {
						t.Fatalf("mark rejection hidden: %v %+v", err, local.Summary)
					}
					return
				}
				peer, peerErr := server.Wait(ctx)
				if err != nil || peerErr != nil || !local.Complete || !peer.Complete {
					t.Fatalf("finite writes: %v %v", err, peerErr)
				}
				if len(clientMark.values) != 3 || len(serverMark.values) != 3 || clientMark.values[2] != 0 || serverMark.values[2] != 34 {
					t.Fatalf("missing role/phase transitions: %v %v", clientMark.values, serverMark.values)
				}
				var changes int
				for _, data := range local.Records {
					var r Record
					json.Unmarshal(data, &r)
					if r.Kind == "marking" {
						changes++
						if r.Marking == nil || r.Marking.Observed != nil {
							t.Fatal("invalid marking evidence")
						}
					}
				}
				if changes != 3 {
					t.Fatal("phase API evidence missing")
				}
			})
		}
	}
}

// TestPhysicalPhaseMarking covers half-open warmup/main/quiet/grace and original-identity independence.
func TestPhysicalPhaseMarking(t *testing.T) {
	p := testutil.Plan()
	p.Warmup = 10
	p.Phases[0].Duration = 20
	p.Phases[0].Client.DSCP = 46
	ph := p.Phases[0]
	ph.ID, ph.Duration, ph.Client.DSCP = "second", 30, 0
	p.Phases = append(p.Phases, ph)
	ws, err := worker.PlanWindows(p, 1)
	if err != nil {
		t.Fatal(err)
	}
	tpt := &markingFixture{}
	mark := phaseMarker(tpt, ws, 100)
	for i, at := range []uint64{100, 109, 110, 129, 130, 159, 160} {
		m, life, phase, err := mark(context.Background(), at)
		if err != nil {
			t.Fatal(err)
		}
		wantLife, wantPhase := byte(1), uint32(0)
		if at >= 110 {
			wantLife = 2
		}
		if at >= 130 {
			wantPhase = 1
		}
		if life != wantLife || phase != wantPhase || (i == 0 || i == 2 || i == 4) != (m != nil) {
			t.Fatalf("incorrect physical phase at %d: marking=%+v life=%d phase=%d", at, m, life, phase)
		}
	}
	if len(tpt.values) != 3 || tpt.values[0] != 46 || tpt.values[1] != 46 || tpt.values[2] != 0 {
		t.Fatal("marking transition missing:", tpt.values)
	}
	tpt.fail = true
	mark = phaseMarker(tpt, ws, 100)
	if m, _, _, err := mark(context.Background(), 130); err == nil || m == nil || m.Applied != nil {
		t.Fatal("phase marking rejection hidden")
	}
}

// TestRoleSpecificMarking verifies independent outgoing server phase settings with no clock subtraction.
func TestRoleSpecificMarking(t *testing.T) {
	p := testutil.Plan()
	p.Phases[0].Client.DSCP, p.Phases[0].Server.DSCP = 12, 34
	for _, role := range []byte{1, 2} {
		ws, err := worker.PlanWindows(p, role)
		if err != nil {
			t.Fatal(err)
		}
		tpt := &markingFixture{}
		mark := phaseMarker(tpt, ws, 0)
		m, _, _, err := mark(context.Background(), 0)
		want := p.Phases[0].Client.DSCP
		if role == 2 {
			want = p.Phases[0].Server.DSCP
		}
		if err != nil || m.Requested != want || m.Observed != nil {
			t.Fatal("directional marking substituted:", m, err)
		}
	}
}

// TestStopAfterElapsedCutoff preserves a full elapsed observation when stop beats a delayed timer callback.
// The past-start fixture deliberately skips all overdue slots; completeness is evidence, not successful traffic.
func TestStopAfterElapsedCutoff(t *testing.T) {
	p := testutil.Plan()
	c, _, p := realPair(t, p)
	anchor := time.Now().Add(-time.Duration(uint64(p.Warmup)+p.Duration()+uint64(p.Grace)) - 20*time.Millisecond)
	c.Start, c.EpochStart = anchor, anchor
	finite, err := NewFinite(context.Background(), c, p, false)
	if err != nil {
		t.Fatal(err)
	}
	finite.Cancel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := finite.Wait(ctx)
	if err != nil || !result.Complete || !result.Receiver.Complete || result.Summary.Skipped != result.Summary.Planned {
		t.Fatalf("elapsed cutoff relabeled interruption: %v %+v", err, result.Summary)
	}
}
