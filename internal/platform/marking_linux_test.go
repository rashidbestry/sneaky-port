package platform

import (
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

// TestLinuxDSCPReadback checks per-socket DSCP/ECN bits without claiming packet observation.
func TestLinuxDSCPReadback(t *testing.T) {
	for _, family := range []string{"udp4", "udp6"} {
		t.Run(family, func(t *testing.T) { // Exercise both family-specific socket options using owned loopback fixtures.
			address := "127.0.0.1:0"
			level, opt := unix.IPPROTO_IP, unix.IP_TOS
			if family == "udp6" {
				address = "[::1]:0"
				level, opt = unix.IPPROTO_IPV6, unix.IPV6_TCLASS
			}
			a, err := net.ResolveUDPAddr(family, address)
			if err != nil {
				t.Fatal(err)
			}
			conn, err := net.ListenUDP(family, a)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			raw, err := conn.SyscallConn()
			if err != nil {
				t.Fatal(err)
			}
			var sockErr error
			if err = raw.Control(func(fd uintptr) { // Seed controlled ECN bits for socket-option preservation, not on-wire evidence.
				sockErr = unix.SetsockoptInt(int(fd), level, opt, 3)
			}); err != nil || sockErr != nil {
				t.Fatalf("seed %v %v", err, sockErr)
			}
			marker := New(conn)
			defer marker.Close()
			mark, err := marker.Set(26)
			if err != nil || mark.Applied == nil || *mark.Applied != 26 || mark.Observed != nil || mark.Status != "api_applied" {
				t.Fatalf("mark %+v %v", mark, err)
			}
			var value int
			if err = raw.Control(func(fd uintptr) { // Verify exact socket bits while retaining the unobserved-packet limitation.
				value, sockErr = unix.GetsockoptInt(int(fd), level, opt)
			}); err != nil || sockErr != nil || value != (26<<2)|3 {
				t.Fatalf("readback bits %d %v %v", value, err, sockErr)
			}
			if _, err = marker.Set(64); err == nil {
				t.Fatal("invalid DSCP accepted")
			}
		})
	}
}
