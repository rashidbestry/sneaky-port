package engine

import (
	"context"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/worker"
)

// TCP exposes shared results, cutoffs and bounded cleanup.
type TCP = Endpoint

// NewTCP connects a caller-prepared authenticated transport to shared phase accounting.
// Start negotiation, phase markings and observation stop remain the caller's responsibility.
func NewTCP(parent context.Context, cfg worker.TCPConfig, p profile.Plan) (*TCP, error) {
	return newEndpoint(parent, cfg, p, "tcp", worker.NewTCP)
}
