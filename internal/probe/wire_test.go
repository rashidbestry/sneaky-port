package probe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"testing"
)

// TestWireRoundTrip checks exact offsets, authentication domains, lengths and precision.
func TestWireRoundTrip(t *testing.T) {
	a := Auth{Adapter: "udp"}
	a.Key[0] = 1
	a.Plan[0] = 2
	f := Frame{Kind: Data, Direction: 1, Lifecycle: 2, Phase: 4, Sequence: 1<<63 + 3, OriginEpoch: 8, Attempt: 1<<54 + 9, Content: []byte("content")}
	f.Run[0] = 3
	f.Session[0] = 4
	b, err := a.Encode(f, UDPMax)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 124+len(f.Content) || binary.BigEndian.Uint64(b[48:56]) != f.Sequence || binary.BigEndian.Uint64(b[68:76]) != f.Attempt {
		t.Fatal("wire layout/precision mismatch")
	}
	got, err := a.Decode(b, UDPMax)
	if err != nil || !reflect.DeepEqual(got, f) {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	for i := range b {
		tampered := append([]byte(nil), b...)
		tampered[i] ^= 1
		if _, err = a.Decode(tampered, UDPMax); err == nil {
			t.Fatalf("accepted tampering at %d", i)
		}
	}
	other := a
	other.Adapter = "tcp"
	if _, err = other.Decode(b, UDPMax); err == nil {
		t.Fatal("cross-adapter tag accepted")
	}
	other = a
	other.Plan[0]++
	if _, err = other.Decode(b, UDPMax); err == nil {
		t.Fatal("cross-plan tag accepted")
	}
}

// TestTCPFraming ensures oversized/partial frames cannot allocate or yield samples.
func TestTCPFraming(t *testing.T) {
	a := Auth{Adapter: "tcp"}
	b, err := a.Encode(Frame{Kind: Data, Direction: 2, Lifecycle: 1, OriginEpoch: 1}, TCPMax)
	if err != nil {
		t.Fatal(err)
	}
	var stream bytes.Buffer
	n, err := WriteTCP(&stream, b)
	if err != nil || n != 128 {
		t.Fatalf("TCP overhead %d: %v", n, err)
	}
	copy := append([]byte(nil), stream.Bytes()...)
	got, err := ReadTCP(&stream)
	if err != nil || !bytes.Equal(got, b) {
		t.Fatal("TCP roundtrip")
	}
	for n := 0; n < len(copy); n++ {
		got, err := ReadTCP(bytes.NewReader(copy[:n]))
		var partial *TCPReadError
		if got != nil || !errors.As(err, &partial) || partial.Bytes != n {
			t.Fatalf("accepted partial %d", n)
		}
	}
	var bad [4]byte
	binary.BigEndian.PutUint32(bad[:], TCPMax+1)
	if _, err := ReadTCP(bytes.NewReader(bad[:])); err == nil {
		t.Fatal("accepted oversized length")
	}
	fail := &limitedWriter{remaining: 5}
	n, err = WriteTCP(fail, b)
	if n != 5 || err != io.ErrClosedPipe {
		t.Fatalf("partial evidence %d %v", n, err)
	}
}

// limitedWriter injects a deterministic partial stream write.
type limitedWriter struct{ remaining int }

// Write accepts only the fixture's remaining bytes, then reports a closed stream.
func (w *limitedWriter) Write(b []byte) (int, error) {
	if len(b) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(b)
	return len(b), nil
}

// TestKindInvariants rejects malformed receipts/binds and checks maximum UDP payloads.
func TestKindInvariants(t *testing.T) {
	a := Auth{Adapter: "udp"}
	for _, f := range []Frame{{Kind: Receipt, Direction: 1, Lifecycle: 2, OriginEpoch: 1, PeerEpoch: 1, Arrival: 2, ReceiptAttempt: 1}, {Kind: BindRequest, Direction: 1, Content: make([]byte, 31)}, {Kind: Data, Direction: 3, Lifecycle: 2, OriginEpoch: 1}} {
		if _, err := a.Encode(f, UDPMax); err == nil {
			t.Fatal("invalid structural frame accepted")
		}
	}
	f := Frame{Kind: Data, Direction: 1, Lifecycle: 2, OriginEpoch: 1, Content: make([]byte, UDPMax-Overhead)}
	if _, err := a.Encode(f, UDPMax); err != nil {
		t.Fatal(err)
	}
	f.Content = append(f.Content, 0)
	if _, err := a.Encode(f, UDPMax); err == nil {
		t.Fatal("oversized UDP accepted")
	}
}
