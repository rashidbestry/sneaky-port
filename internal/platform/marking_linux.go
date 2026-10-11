package platform

import (
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

type Marker struct{ conn net.Conn }

// Set preserves ECN bits when applying DSCP to a socket; readback is not packet evidence.
func (m *Marker) Set(dscp int) (Marking, error) {
	out := Marking{Requested: dscp, Status: "unsupported", Reason: "socket marking unavailable"}
	if dscp < 0 || dscp > 63 {
		return out, errors.New("invalid DSCP")
	}
	c, ok := m.conn.(syscall.Conn)
	if !ok {
		return out, errors.New(out.Reason)
	}
	raw, err := c.SyscallConn()
	if err != nil {
		return out, err
	}
	var socketErr error
	err = raw.Control(func(fd uintptr) { // Modify only DSCP bits on the owned IPv4/IPv6 socket.
		level, opt := unix.IPPROTO_IP, unix.IP_TOS
		host, _, _ := net.SplitHostPort(m.conn.LocalAddr().String())
		if net.ParseIP(host).To4() == nil {
			level, opt = unix.IPPROTO_IPV6, unix.IPV6_TCLASS
		}
		current, e := unix.GetsockoptInt(int(fd), level, opt)
		if e != nil {
			socketErr = e
			return
		}
		socketErr = unix.SetsockoptInt(int(fd), level, opt, (current&3)|(dscp<<2))
		if socketErr != nil {
			return
		}
		value, e := unix.GetsockoptInt(int(fd), level, opt)
		if e != nil {
			socketErr = e
			return
		}
		applied := value >> 2
		out.Applied = &applied
		out.Status = "api_applied"
		out.Reason = "socket readback only; outer packets unobserved"
	})
	if err != nil {
		return out, err
	}
	if socketErr != nil {
		return out, socketErr
	}
	if out.Applied == nil || *out.Applied != dscp {
		return out, errors.New("DSCP readback mismatch")
	}
	return out, nil
}

// Close has no extra Linux resource; socket ownership stays with the adapter.
func (m *Marker) Close() {}
