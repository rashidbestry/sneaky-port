package worker

import (
	"context"
	"sneaky-port/internal/probe"
)

// UDPConfig retains the existing datagram caller contract through shared configuration.
type UDPConfig = Config

// UDP retains the existing API while all scheduling/accounting workers are shared.
type UDP = Endpoint

// transportContract fixes authenticated frame limits and transport-boundary byte overhead.
// Queue capacity counts common authenticated bytes, separately from framing in events.
type transportContract struct {
	id                     string
	maxFrame, framingBytes int
}

// NewUDP transfers a prepared UDP transport after validating directional authentication.
func NewUDP(parent context.Context, c UDPConfig) (*UDP, error) {
	return newEndpoint(parent, c, transportContract{"udp", probe.UDPMax, 0})
}
