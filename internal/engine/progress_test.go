package engine

import (
	"context"
	"encoding/json"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/testutil"
	"testing"
	"time"
)

// TestFiniteProgress runs real UDP/TCP with independent peer timing and a reader that never drives the engine.
func TestFiniteProgress(t *testing.T) {
	for _, carrier := range []string{"udp", "tcp"} {
		t.Run(carrier, func(t *testing.T) { // Each carrier keeps its authentic loopback transport and common count path.
			p := testutil.Plan()
			p.Adapter = carrier
			if carrier == "tcp" {
				p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
			}
			p.Warmup = 150000000
			p.Phases[0].Client.RateNumerator = 10
			p.Phases[0].Server.RateNumerator = 10
			p.Deadline = 100000000
			p.Limits.MaxLag = 50000000
			p.Phases[0].Duration = 650000000
			p.Grace = 150000000
			clientCfg, serverCfg, p := realPair(t, p)
			feed := &measure.ProgressFeed{}
			client, err := NewFinite(measure.WithProgress(context.Background(), feed), clientCfg, p, false)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Cancel()
			server, err := NewFinite(context.Background(), serverCfg, p, true)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Cancel()
			deadline := time.Now().Add(2 * time.Second)
			seen := false
			for time.Now().Before(deadline) {
				snapshot := feed.Snapshot()
				if snapshot != nil && snapshot.Stage == "measurement" && snapshot.Main.Planned > 0 {
					seen = true
					if snapshot.Main.Completed > snapshot.Main.Attempted || snapshot.Cutoff > snapshot.AsOf {
						t.Fatal("invalid progress population", snapshot)
					}
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			r, err := client.Wait(ctx)
			_, peerErr := server.Wait(ctx)
			if err != nil || peerErr != nil || !r.Complete || !seen {
				t.Fatal("finite progress/completion failed", err, peerErr, seen)
			}
		})
	}
}
