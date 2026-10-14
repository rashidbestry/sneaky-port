package platform

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestLinuxPacketMarking independently observes owned loopback UDP/TCP packets.
// Explicit opt-in and CAP_NET_RAW are required; this is no NAT or Windows evidence.
func TestLinuxPacketMarking(t *testing.T) {
	if os.Getenv("SNEAKY_PORT_PACKET_CAPTURE") != "1" {
		t.Skip("opt in with SNEAKY_PORT_PACKET_CAPTURE=1 and CAP_NET_RAW")
	}
	for _, network := range []string{"udp4", "udp6", "tcp4", "tcp6"} {
		t.Run(network, func(t *testing.T) { // Capture each actual carrier/family and both physical send directions.
			host := "127.0.0.1"
			if network[3] == '6' {
				host = "::1"
			}
			address := net.JoinHostPort(host, "0")
			var client, server net.Conn
			if network[:3] == "udp" {
				a, err := net.ResolveUDPAddr(network, address)
				if err != nil {
					t.Fatal(err)
				}
				c, err := net.ListenUDP(network, a)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { /* Release this fixture's socket even on capture failure. */ c.Close() })
				s, err := net.ListenUDP(network, a)
				if err != nil {
					t.Fatal(err)
				}
				client, server = c, s
			} else {
				listener, err := net.Listen(network, address)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				client, err = net.DialTimeout(network, listener.Addr().String(), time.Second)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { /* Close the owned stream on every assertion path. */ client.Close() })
				server, err = listener.Accept()
				if err != nil {
					t.Fatal(err)
				}
			}
			defer client.Close()
			defer server.Close()
			for i, conn := range []net.Conn{client, server} {
				ecn := 0
				if network[:3] == "udp" {
					ecn = 3
				}
				seedCaptureECN(t, conn, network[3] == '6', ecn)
				marker := New(conn)
				defer marker.Close()
				requested := []int{46, 12}[i]
				mark, err := marker.Set(requested)
				if err != nil || mark.Applied == nil || *mark.Applied != requested || mark.Observed != nil {
					t.Fatalf("API and captured evidence must remain distinct: %+v %v", mark, err)
				}
			}
			fd := openLoopbackCapture(t)
			defer unix.Close(fd)
			for i, conn := range []net.Conn{client, server} {
				peer := server
				if i == 1 {
					peer = client
				}
				payload := []byte("sneaky-port-owned-capture-" + network + string(rune('a'+i)))
				conn.SetWriteDeadline(time.Now().Add(time.Second))
				if udp, ok := conn.(*net.UDPConn); ok {
					_, err := udp.WriteToUDP(payload, peer.LocalAddr().(*net.UDPAddr))
					if err != nil {
						t.Fatal(err)
					}
				} else if _, err := conn.Write(payload); err != nil {
					t.Fatal(err)
				}
				source, destination := 0, 0
				if network[:3] == "udp" {
					source, destination = conn.LocalAddr().(*net.UDPAddr).Port, peer.LocalAddr().(*net.UDPAddr).Port
				} else {
					source, destination = conn.LocalAddr().(*net.TCPAddr).Port, peer.LocalAddr().(*net.TCPAddr).Port
				}
				capturePayload(t, fd, payload, source, destination, []int{46, 12}[i], network[:3] == "udp", network[3] == '6')
				peer.SetReadDeadline(time.Now().Add(time.Second))
				buf := make([]byte, 256)
				var n int
				var err error
				if network[:3] == "udp" {
					n, err = peer.Read(buf)
				} else {
					n, err = io.ReadFull(peer, buf[:len(payload)])
				}
				if err != nil || !bytes.Equal(buf[:n], payload) {
					t.Fatalf("carrier payload mismatch: %v", err)
				}
			}
			stats, err := unix.GetsockoptTpacketStats(fd, unix.SOL_PACKET, unix.PACKET_STATISTICS)
			if err != nil || stats.Drops != 0 {
				t.Fatalf("capture loss/visibility invalid: %+v %v", stats, err)
			}
			t.Logf("AF_PACKET socket packets=%d drops=%d; loopback ingress headers only, physical offload/NAT remain unobserved", stats.Packets, stats.Drops)
		})
	}
}

// seedCaptureECN seeds only the owned socket: UDP CE tests bit preservation,
// while TCP keeps ECN zero because TCP ECN negotiation is protocol-native.
func seedCaptureECN(t *testing.T, conn net.Conn, ipv6 bool, value int) {
	t.Helper()
	raw, err := conn.(syscall.Conn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var socketErr error
	err = raw.Control(func(fd uintptr) { // Set controlled ECN before the production DSCP backend runs.
		level, opt := unix.IPPROTO_IP, unix.IP_TOS
		if ipv6 {
			level, opt = unix.IPPROTO_IPV6, unix.IPV6_TCLASS
		}
		socketErr = unix.SetsockoptInt(int(fd), level, opt, value)
	})
	if err != nil || socketErr != nil {
		t.Fatalf("seed ECN: %v %v", err, socketErr)
	}
}

// openLoopbackCapture opens one independent bounded AF_PACKET capture socket;
// it changes no interface, filter, routing, capture session or product result.
func openLoopbackCapture(t *testing.T) int {
	t.Helper()
	lo, err := net.InterfaceByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, int(binary.NativeEndian.Uint16([]byte{0, 3})))
	if err != nil {
		t.Fatalf("CAP_NET_RAW capture prerequisite: %v", err)
	}
	if err = unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: lo.Index, Protocol: binary.NativeEndian.Uint16([]byte{0, 3})}); err != nil {
		unix.Close(fd)
		t.Fatal(err)
	}
	if err = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 2}); err != nil {
		unix.Close(fd)
		t.Fatal(err)
	}
	return fd
}

// capturePayload matches exact disposable content independently of socket
// readback, checking DSCP/ECN, protocol and actual ports at loopback ingress.
// Outgoing duplicates are excluded; matching is not a capture-loss guarantee.
func capturePayload(t *testing.T, fd int, payload []byte, source, destination, dscp int, udp, ipv6 bool) {
	t.Helper()
	buf := make([]byte, 65536)
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		n, from, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("matching ingress packet unavailable: %v", err)
		}
		if link, ok := from.(*unix.SockaddrLinklayer); ok && link.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		packet := buf[:n]
		if n < 40 {
			continue
		}
		var traffic, protocol byte
		var offset int
		switch packet[0] >> 4 {
		case 4:
			traffic, protocol, offset = packet[1], packet[9], int(packet[0]&15)*4
		case 6:
			traffic, protocol, offset = (packet[0]&15)<<4|packet[1]>>4, packet[6], 40
		default:
			continue
		}
		if n < offset+8 {
			continue
		}
		transport := packet[offset:]
		header := 8
		if udp && protocol != unix.IPPROTO_UDP {
			continue
		}
		if !udp {
			if protocol != unix.IPPROTO_TCP || len(transport) < 20 {
				continue
			}
			header = int(transport[12]>>4) * 4
		}
		if len(transport) < header || !bytes.Equal(transport[header:], payload) {
			continue
		}
		if (packet[0]>>4 == 6) != ipv6 {
			t.Fatal("captured IP family differs from the requested owned carrier family")
		}
		if int(binary.BigEndian.Uint16(transport)) != source || int(binary.BigEndian.Uint16(transport[2:])) != destination {
			t.Fatal("captured ports differ from actual owned socket bindings")
		}
		ecn := 0
		if udp {
			ecn = 3
		}
		if int(traffic>>2) != dscp || int(traffic&3) != ecn {
			t.Fatalf("captured DSCP/ECN=%d/%d, requested=%d/%d", traffic>>2, traffic&3, dscp, ecn)
		}
		t.Logf("loopback ingress IPv%d protocol=%d source_port=%d destination_port=%d DSCP=%d ECN=%d bytes=%d; no NAT, physical-path or capture-loss claim", packet[0]>>4, protocol, binary.BigEndian.Uint16(transport), binary.BigEndian.Uint16(transport[2:]), traffic>>2, traffic&3, n)
		return
	}
	t.Fatal("capture observation window expired")
}
