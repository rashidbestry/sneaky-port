// Package platform implements per-owned-socket marking with honest observation limits.
package platform

import "net"

// Marking distinguishes OS API application from independent outer-packet observation.
type Marking struct {
	Requested int    `json:"requested"`
	Applied   *int   `json:"applied"`
	Status    string `json:"status"`
	Observed  *int   `json:"observed"`
	Reason    string `json:"reason"`
}

// New creates marking state for one owned socket without changing global settings.
func New(conn net.Conn) *Marker { return &Marker{conn: conn} }
