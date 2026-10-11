package worker

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// pipeTransport uses real stream framing over an unbuffered local connection.
// It supplies no marking or network-packet observation capability.
type pipeTransport struct {
	adapter.Transport
	conn net.Conn
}

// Read returns only complete frames; truncation retains transport-byte evidence.
func (p pipeTransport) Read() ([]byte, error) { return probe.ReadTCP(p.conn) }

// Write applies the explicit budget while preserving prefix and partial-body counts.
func (p pipeTransport) Write(b []byte, budget time.Duration) (int, error) {
	p.conn.SetWriteDeadline(time.Now().Add(budget))
	return probe.WriteTCP(p.conn, b)
}

// SetReadDeadline allows bounded complete-frame fixture reads.
func (p pipeTransport) SetReadDeadline(at time.Time) error { return p.conn.SetReadDeadline(at) }

// Close interrupts only this fixture's connection.
func (p pipeTransport) Close() error { return p.conn.Close() }

// tcpPipe provides explicit synthetic window settings without an SSH readiness claim.
func tcpPipe(t *testing.T) (TCPConfig, net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { // Close both owned ends even after failed constructor/fixture assertions.
		a.Close()
		b.Close()
	})
	p := testutil.Plan()
	epoch := time.Now().Add(-time.Second)
	c := TCPConfig{Transport: pipeTransport{conn: a}, SendAuth: probe.Auth{Adapter: "tcp", Plan: [32]byte{3}, Key: [32]byte{1}}, ReceiveAuth: probe.Auth{Adapter: "tcp", Plan: [32]byte{3}, Key: [32]byte{2}}, Run: [16]byte{4}, Session: [16]byte{5}, Direction: 1, Epoch: 11, PeerEpoch: 22, EpochStart: epoch, Start: time.Now().Add(30 * time.Millisecond), Local: fixtureSchedule(0), Peer: fixtureSchedule(5), Limits: p.Limits, EventCapacity: 256}
	c.Limits.MaxLag = profile.Decimal(time.Second)
	return c, b
}

// startTCP transfers a fixture stream and guarantees joined cancellation even on failure.
func startTCP(t *testing.T, c TCPConfig) *TCP {
	t.Helper()
	u, err := NewTCP(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Bound cleanup; transport failures are checked explicitly by the caller.
		u.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		u.Wait(ctx)
	})
	return u
}

// TestTCPCompleteFrameArrival requires authentication of the final byte before any arrival/receipt.
// Prefix splitting and body splitting model stream reads, not IP fragmentation or packet loss.
func TestTCPCompleteFrameArrival(t *testing.T) {
	c, peer := tcpPipe(t)
	u := startTCP(t, c)
	l := collect(u)
	f := probe.Frame{Kind: probe.Data, Direction: 2, Run: c.Run, Session: c.Session, Lifecycle: c.Peer.Lifecycle, Phase: c.Peer.Phase, Sequence: c.Peer.FirstSequence, OriginEpoch: c.PeerEpoch, Content: c.Peer.Traffic.Payload()}
	b, err := c.ReceiveAuth.Encode(f, probe.TCPMax)
	if err != nil {
		t.Fatal(err)
	}
	stream := make([]byte, len(b)+4)
	binary.BigEndian.PutUint32(stream, uint32(len(b)))
	copy(stream[4:], b)
	peer.SetWriteDeadline(time.Now().Add(time.Second))
	for _, chunk := range [][]byte{stream[:2], stream[2:4], stream[4 : len(stream)-1]} {
		if _, err := peer.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if count(l.snapshot(), "arrival", probe.Data) != 0 || accepted(l.snapshot(), probe.Receipt) != 0 {
		t.Fatal("incomplete frame became arrival")
	}
	if _, err := peer.Write(stream[len(stream)-1:]); err != nil {
		t.Fatal(err)
	}
	peer.SetReadDeadline(time.Now().Add(time.Second))
	raw, err := probe.ReadTCP(peer)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := c.SendAuth.Decode(raw, probe.TCPMax)
	if err != nil || receipt.Kind != probe.Receipt || receipt.Sequence != f.Sequence || receipt.ReceiptAttempt < receipt.Arrival {
		t.Fatal("invalid reverse receipt", receipt, err)
	}
	until(t, func() bool { // Wait for complete-frame and completed reverse-write evidence.
		return count(l.snapshot(), "arrival", probe.Data) == 1 && count(l.snapshot(), "write", probe.Receipt) == 1
	})
	for _, e := range l.snapshot() {
		if e.Kind == "arrival" && e.Bytes != len(stream) {
			t.Fatal("arrival prefix bytes lost")
		}
		if e.Kind == "write" && e.Bytes != probe.Overhead+4 {
			t.Fatal("receipt prefix bytes lost")
		}
	}
}

// TestTCPTruncatedRead never creates an arrival and retains exact consumed bytes on EOF/cancel.
func TestTCPTruncatedRead(t *testing.T) {
	for _, consumed := range []int{2, 4, 9} {
		for _, cancelRead := range []bool{false, true} {
			t.Run(strconv.Itoa(consumed)+map[bool]string{true: "cancel", false: "eof"}[cancelRead], func(t *testing.T) { // Check prefix/body interruption without interpreting it as network loss.
				c, peer := tcpPipe(t)
				u := startTCP(t, c)
				l := collect(u)
				b := make([]byte, consumed)
				if consumed >= 4 {
					binary.BigEndian.PutUint32(b, probe.Overhead)
				}
				peer.SetWriteDeadline(time.Now().Add(time.Second))
				if _, err := peer.Write(b); err != nil {
					t.Fatal(err)
				}
				if cancelRead {
					u.Close()
				} else {
					peer.Close()
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err := u.Wait(ctx)
				if cancelRead && err != nil || !cancelRead && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
					t.Fatal("termination", err)
				}
				<-l.done
				if count(l.snapshot(), "arrival", probe.Data) != 0 {
					t.Fatal("partial frame counted")
				}
				n := 0
				for _, e := range l.snapshot() {
					if e.Kind == "read_error" {
						n += e.Bytes
					}
				}
				if n != consumed {
					t.Fatalf("partial read bytes %d want %d", n, consumed)
				}
			})
		}
	}
}

// TestTCPBlockedWriteKeepsScheduleAndReader proves independent progress under actual stream backpressure.
func TestTCPBlockedWriteKeepsScheduleAndReader(t *testing.T) {
	c, peer := tcpPipe(t)
	c.Local = fixtureSchedule(100)
	c.Limits.QueueMessages = 3
	c.Limits.ReceiptReserve = 1
	c.Limits.WriteTimeout = profile.Decimal(400 * time.Millisecond)
	u := startTCP(t, c)
	l := collect(u)
	until(t, func() bool { // Ensure the first unbuffered stream write is blocked before injecting peer data.
		return count(l.snapshot(), "attempt", probe.Data) == 1
	})
	f := probe.Frame{Kind: probe.Data, Direction: 2, Run: c.Run, Session: c.Session, Lifecycle: c.Peer.Lifecycle, Phase: c.Peer.Phase, Sequence: c.Peer.FirstSequence, OriginEpoch: c.PeerEpoch, Content: c.Peer.Traffic.Payload()}
	b, err := c.ReceiveAuth.Encode(f, probe.TCPMax)
	if err != nil {
		t.Fatal(err)
	}
	peer.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err = probe.WriteTCP(peer, b); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-u.Scheduled():
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err = u.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("blocked writer timeout", err)
	}
	<-l.done
	events := l.snapshot()
	if reached(events) != u.localCount || accepted(events, probe.Data) < 1 || accepted(events, probe.Data) > 2 || count(events, "attempt", probe.Data) != 1 || count(events, "arrival", probe.Data) != 1 || accepted(events, probe.Receipt) != 1 {
		t.Fatalf("independence/caps: %+v", events)
	}
	for _, e := range events {
		if e.Kind == "admission" && e.Admission.Done != nil {
			result := <-e.Admission.Done
			if e.Frame.Kind == probe.Receipt && result.Attempted {
				t.Fatal("receipt bypassed blocked FIFO")
			}
		}
	}
}

// TestTCPFrameLimitAndAdapterIdentity checks carrier-specific bounds before ownership transfer.
func TestTCPFrameLimitAndAdapterIdentity(t *testing.T) {
	c, _ := tcpPipe(t)
	c.Local.Traffic.ContentBytes = probe.UDPMax
	if _, _, _, err := prepareWindows(c, probe.TCPMax); err != nil {
		t.Fatal("TCP content restricted to UDP", err)
	}
	if _, _, _, err := prepareWindows(c, probe.UDPMax); err == nil {
		t.Fatal("oversized UDP allowed")
	}
	c.SendAuth.Adapter = "udp"
	if _, err := NewTCP(context.Background(), c); err == nil {
		t.Fatal("wrong adapter accepted")
	}
	c.SendAuth.Adapter = "tcp"
	c.Local.Traffic.ContentBytes = probe.TCPMax - probe.Overhead + 1
	if _, err := NewTCP(context.Background(), c); err == nil {
		t.Fatal("oversized TCP allowed")
	}
}

// TestTCPPartialWriteTerminatesFraming records actual prefix/body bytes before peer interruption.
func TestTCPPartialWriteTerminatesFraming(t *testing.T) {
	c, peer := tcpPipe(t)
	c.Local = fixtureSchedule(100)
	u := startTCP(t, c)
	l := collect(u)
	peer.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 7)
	if _, err := io.ReadFull(peer, b); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(b) != uint32(probe.Overhead+c.Local.Traffic.ContentBytes) {
		t.Fatal("missing stream prefix")
	}
	peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := u.Wait(ctx); err == nil {
		t.Fatal("partial stream treated as successful")
	}
	<-l.done
	if count(l.snapshot(), "attempt", probe.Data) != 1 || count(l.snapshot(), "write", probe.Data) != 1 || count(l.snapshot(), "arrival", probe.Receipt) != 0 {
		t.Fatal("framing resumed after partial write")
	}
	for _, e := range l.snapshot() {
		if e.Kind == "write" && (e.Bytes != 7 || e.Err == nil) {
			t.Fatal("partial bytes lost", e)
		}
	}
}

// closingRead models a complete frame already returning from an adapter when stop is observed.
type closingRead struct {
	adapter.Transport
	started, stopped chan struct{}
	frame            []byte
}

// Read returns the complete buffered frame only when cancellation closes the fixture.
func (r *closingRead) Read() ([]byte, error) { close(r.started); <-r.stopped; return r.frame, nil }

// Close unblocks the single in-progress read; the owned worker calls this once during join.
func (r *closingRead) Close() error { close(r.stopped); return r.Transport.Close() }

// TestTCPCompleteReadRacingStop keeps outside-cutoff evidence without sending a new receipt.
func TestTCPCompleteReadRacingStop(t *testing.T) {
	c, _ := tcpPipe(t)
	f := probe.Frame{Kind: probe.Data, Direction: 2, Run: c.Run, Session: c.Session, Lifecycle: c.Peer.Lifecycle, Phase: c.Peer.Phase, Sequence: c.Peer.FirstSequence, OriginEpoch: c.PeerEpoch, Content: c.Peer.Traffic.Payload()}
	b, err := c.ReceiveAuth.Encode(f, probe.TCPMax)
	if err != nil {
		t.Fatal(err)
	}
	r := &closingRead{Transport: c.Transport, started: make(chan struct{}), stopped: make(chan struct{}), frame: b}
	c.Transport = r
	u := startTCP(t, c)
	l := collect(u)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-r.started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	u.Close()
	if err := u.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	<-l.done
	if count(l.snapshot(), "arrival", probe.Data) != 1 || accepted(l.snapshot(), probe.Receipt) != 0 {
		t.Fatal("complete read discarded or receipt generated after stop")
	}
	for _, e := range l.snapshot() {
		if e.Kind == "arrival" && e.At < u.stoppedAt() {
			t.Fatal("outside-cutoff fixture became early arrival")
		}
	}
}
