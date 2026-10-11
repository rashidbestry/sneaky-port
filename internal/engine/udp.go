package engine

import (
	"context"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/worker"
)

// UDP exposes shared results, cutoffs and bounded cleanup.
type UDP = Endpoint

// NewUDP connects a caller-prepared authenticated transport to shared phase accounting.
// Start negotiation, phase markings and observation stop remain the caller's responsibility.
func NewUDP(parent context.Context, cfg worker.UDPConfig, p profile.Plan) (*UDP, error) {
	return newEndpoint(parent, cfg, p, "udp", worker.NewUDP)
}
