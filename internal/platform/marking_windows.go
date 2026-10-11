package platform

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var qwave = windows.NewLazySystemDLL("qwave.dll")
var create = qwave.NewProc("QOSCreateHandle")
var add = qwave.NewProc("QOSAddSocketToFlow")
var set = qwave.NewProc("QOSSetFlow")
var closeHandle = qwave.NewProc("QOSCloseHandle")

type Marker struct {
	conn   net.Conn
	handle windows.Handle
	flow   uint32
}

// Set uses qWave's nonadaptive per-socket DSCP operation without bandwidth controls.
func (m *Marker) Set(dscp int) (Marking, error) {
	out := Marking{Requested: dscp, Status: "unknown", Reason: "outer packets unobserved"}
	if dscp < 0 || dscp > 63 {
		return out, errors.New("invalid DSCP")
	}
	// Leave an unmarked baseline untouched when qWave/privileges are not available.
	if dscp == 0 && m.handle == 0 {
		out.Reason = "unmarked baseline requested; OS policy and outer marking unverified"
		return out, nil
	}
	for _, p := range []*windows.LazyProc{create, add, set, closeHandle} {
		if err := p.Find(); err != nil {
			return out, err
		}
	}
	if m.handle == 0 {
		version := struct{ Major, Minor uint16 }{1, 0}
		r, _, err := create.Call(uintptr(unsafe.Pointer(&version)), uintptr(unsafe.Pointer(&m.handle)))
		if r == 0 {
			return out, fmt.Errorf("QOSCreateHandle: %w", err)
		}
		c, ok := m.conn.(syscall.Conn)
		if !ok {
			m.Close()
			return out, errors.New("socket handle unavailable")
		}
		raw, err := c.SyscallConn()
		if err != nil {
			m.Close()
			return out, err
		}
		var callErr error
		err = raw.Control(func(fd uintptr) { // Best-effort nonadaptive flow does not add rate management.
			r, _, e := add.Call(uintptr(m.handle), fd, 0, 0, 2, uintptr(unsafe.Pointer(&m.flow)))
			if r == 0 {
				callErr = e
			}
		})
		if err != nil || callErr != nil {
			m.Close()
			return out, fmt.Errorf("QOSAddSocketToFlow: %v %v", err, callErr)
		}
	}
	value := uint32(dscp)
	r, _, err := set.Call(uintptr(m.handle), uintptr(m.flow), 2, 4, uintptr(unsafe.Pointer(&value)), 0, 0)
	runtime.KeepAlive(m.conn)
	if r == 0 {
		return out, fmt.Errorf("QOSSetOutgoingDSCPValue (requires suitable privileges): %w", err)
	}
	applied := dscp
	out.Applied = &applied
	out.Status = "api_applied"
	out.Reason = "qWave accepted DSCP; outer packets and ECN preservation unobserved"
	return out, nil
}

// Close releases the application-owned qWave handle without editing OS QoS policy.
func (m *Marker) Close() {
	if m.handle != 0 {
		closeHandle.Call(uintptr(m.handle))
		m.handle = 0
		m.flow = 0
	}
}
