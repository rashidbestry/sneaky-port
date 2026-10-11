// Package testutil supplies deliberately synthetic inputs for deterministic tests only.
package testutil

import (
	"encoding/json"

	"sneaky-port/internal/profile"
)

// Plan supplies every explicit input; these arbitrary fixture values are not FPS defaults.
func Plan() profile.Plan {
	d := profile.Direction{Mode: "steady", RateNumerator: 1000, RateDenominator: 1, ContentBytes: 4, ContentPattern: "YWJj", DSCP: 0}
	return profile.Plan{Version: 1, ID: "fixture", PairingRef: "fixture", Adapter: "udp", Contract: 1, Family: "ipv4", ServerAddress: "127.0.0.1", DestinationPort: 32000, SourcePort: "auto", Options: map[string]json.RawMessage{}, Warmup: 0, Phases: []profile.Phase{{ID: "main", Duration: 1e6, Client: d, Server: d}}, Grace: 1e6, Deadline: 1e6, Limits: profile.Limits{QueueMessages: 16, QueueBytes: 1 << 20, ReceiptReserve: 2, Outstanding: 100, LedgerMessages: 1000, EventBytes: 1 << 20, FileBytes: 2 << 20, MaxLag: 1e6, WriteTimeout: 2e9, SetupTimeout: 2e9, ShutdownTimeout: 2e9, HeartbeatInterval: 1e8, Lease: 2e9, ControlRequests: 100}, Calibration: profile.Calibration{Samples: 2, Window: 4, Cadence: 1e8, Expiry: 2e9, MaxError: 1e6, BoundSource: "synthetic unverified test fixture"}, PreflightDuration: 0, PreflightInterval: 0, SpikeThreshold: 1e6, ComparisonAxes: []string{}, MatchedMetadata: map[string]string{}}
}
