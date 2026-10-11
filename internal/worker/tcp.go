package worker

import (
	"context"
	"sneaky-port/internal/probe"
)

// TCPConfig uses the same local windows, budgets and caller-owned anchors as UDP.
type TCPConfig = Config

// TCP runs shared scheduler/reader/FIFO workers over an already authenticated stream.
type TCP = Endpoint

// NewTCP transfers a prepared framed TCP transport; complete arrivals include its prefix.
// Any failed/partial write ends the segment without retry, reconnect or substitution.
func NewTCP(parent context.Context, c TCPConfig) (*TCP, error) {
	return newEndpoint(parent, c, transportContract{"tcp", probe.TCPMax, 4})
}
