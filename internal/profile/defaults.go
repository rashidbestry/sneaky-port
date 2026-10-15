package profile

import "encoding/json"

// Default returns an independent editable startup plan; these starter values are not FPS tuning or clock-accuracy evidence.
// Endpoint identity/trust still requires pairing or dashboard SSH setup before a measurement can start.
func Default() Plan {
	direction := Direction{Mode: "steady", RateNumerator: 60, RateDenominator: 1, ContentBytes: 32, ContentPattern: "QUJD"}
	return Plan{
		Version: 1, ID: "default-udp", PairingRef: "lab-server", Adapter: "udp", Contract: 1,
		Family: "ipv4", ServerAddress: "127.0.0.1", DestinationPort: 32000, SourcePort: "auto",
		Options: map[string]json.RawMessage{}, Phases: []Phase{{ID: "main", Duration: 10e9, Client: direction, Server: direction}},
		Grace: 100e6, Deadline: 100e6, SpikeThreshold: 10e6,
		Limits: Limits{
			QueueMessages: 64, QueueBytes: 1 << 20, ReceiptReserve: 8, Outstanding: 1024,
			LedgerMessages: 4096, EventBytes: 16 << 20, FileBytes: 64 << 20, MaxLag: 10e6,
			WriteTimeout: 5e9, SetupTimeout: 5e9, ShutdownTimeout: 5e9,
			HeartbeatInterval: 1e9, Lease: 10e9, ControlRequests: 4096,
		},
		Calibration:    Calibration{Samples: 2, Window: 4, Cadence: 1e9, Expiry: 10e9, MaxError: 1e6, BoundSource: "Unverified built-in starter; no calibrated clock evidence"},
		ComparisonAxes: []string{}, MatchedMetadata: map[string]string{},
	}
}
