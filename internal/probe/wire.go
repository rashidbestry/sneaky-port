// Package probe implements the authenticated common UDP/TCP wire contract.
package probe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const HeaderSize = 92
const Overhead = HeaderSize + sha256.Size
const TCPMax = 1 << 20
const UDPMax = 65507
const (
	BindRequest byte = iota + 1
	BindAck
	Data
	Receipt
)

// Frame carries original data identity; receipts preserve that identity across directions.
type Frame struct {
	Kind, Direction, Lifecycle       byte
	Run, Session                     [16]byte
	Phase                            uint32
	Sequence                         uint64
	OriginEpoch, PeerEpoch           uint32
	Attempt, Arrival, ReceiptAttempt uint64
	Content                          []byte
}

// Auth binds a frame to a frozen plan, adapter and physical-direction key.
type Auth struct {
	Plan    [32]byte
	Adapter string
	Key     [32]byte
}

// ID decodes only canonical lowercase 128-bit identifiers.
func ID(s string) ([16]byte, error) {
	var id [16]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(id) || hex.EncodeToString(b) != s {
		return id, errors.New("ID must be 32 lowercase hexadecimal characters")
	}
	copy(id[:], b)
	return id, nil
}

// tag authenticates the exact domain-separated plan/adapter/header/content bytes.
func (a Auth) tag(b []byte) []byte {
	m := hmac.New(sha256.New, a.Key[:])
	m.Write([]byte("sneaky-port/probe/v1\x00"))
	m.Write(a.Plan[:])
	var size [2]byte
	binary.BigEndian.PutUint16(size[:], uint16(len(a.Adapter)))
	m.Write(size[:])
	m.Write([]byte(a.Adapter))
	m.Write(b)
	return m.Sum(nil)
}

// Encode validates structural invariants before encoding and signing a complete frame.
func (a Auth) Encode(f Frame, max int) ([]byte, error) {
	if len(a.Adapter) == 0 || len(a.Adapter) > 65535 || max < Overhead || len(f.Content) > max-Overhead {
		return nil, errors.New("invalid adapter or frame length")
	}
	if err := f.validate(); err != nil {
		return nil, err
	}
	b := make([]byte, Overhead+len(f.Content))
	copy(b[:4], "SPV1")
	binary.BigEndian.PutUint16(b[4:6], 1)
	b[6], b[7], b[8] = f.Kind, f.Direction, f.Lifecycle
	copy(b[12:28], f.Run[:])
	copy(b[28:44], f.Session[:])
	binary.BigEndian.PutUint32(b[44:48], f.Phase)
	binary.BigEndian.PutUint64(b[48:56], f.Sequence)
	binary.BigEndian.PutUint32(b[56:60], uint32(len(f.Content)))
	binary.BigEndian.PutUint32(b[60:64], f.OriginEpoch)
	binary.BigEndian.PutUint32(b[64:68], f.PeerEpoch)
	binary.BigEndian.PutUint64(b[68:76], f.Attempt)
	binary.BigEndian.PutUint64(b[76:84], f.Arrival)
	binary.BigEndian.PutUint64(b[84:92], f.ReceiptAttempt)
	copy(b[HeaderSize:], f.Content)
	copy(b[len(b)-sha256.Size:], a.tag(b[:len(b)-sha256.Size]))
	return b, nil
}

// Decode authenticates before exposing a frame and rejects noncanonical flags/lengths.
func (a Auth) Decode(b []byte, max int) (Frame, error) {
	var f Frame
	if len(a.Adapter) == 0 || len(a.Adapter) > 65535 || len(b) < Overhead || len(b) > max || string(b[:4]) != "SPV1" || binary.BigEndian.Uint16(b[4:6]) != 1 || b[9] != 0 || binary.BigEndian.Uint16(b[10:12]) != 0 {
		return f, errors.New("invalid wire header or length")
	}
	if uint64(binary.BigEndian.Uint32(b[56:60])) != uint64(len(b)-Overhead) || !hmac.Equal(b[len(b)-32:], a.tag(b[:len(b)-32])) {
		return f, errors.New("invalid content length or authentication tag")
	}
	f.Kind, f.Direction, f.Lifecycle = b[6], b[7], b[8]
	copy(f.Run[:], b[12:28])
	copy(f.Session[:], b[28:44])
	f.Phase = binary.BigEndian.Uint32(b[44:48])
	f.Sequence = binary.BigEndian.Uint64(b[48:56])
	f.OriginEpoch = binary.BigEndian.Uint32(b[60:64])
	f.PeerEpoch = binary.BigEndian.Uint32(b[64:68])
	f.Attempt = binary.BigEndian.Uint64(b[68:76])
	f.Arrival = binary.BigEndian.Uint64(b[76:84])
	f.ReceiptAttempt = binary.BigEndian.Uint64(b[84:92])
	f.Content = append([]byte(nil), b[HeaderSize:len(b)-32]...)
	return f, f.validate()
}

// validate enforces kind-specific fields without interpreting cross-host timestamps.
func (f Frame) validate() error {
	if f.Direction != 1 && f.Direction != 2 {
		return errors.New("invalid origin direction")
	}
	switch f.Kind {
	case BindRequest, BindAck:
		if f.Lifecycle != 0 || f.Phase != 0 || f.Sequence != 0 || len(f.Content) != 32 || f.Attempt != 0 || f.Arrival != 0 || f.ReceiptAttempt != 0 || f.OriginEpoch != 0 || f.PeerEpoch != 0 {
			return errors.New("invalid binding fields")
		}
	case Data:
		if f.Lifecycle < 1 || f.Lifecycle > 2 || f.OriginEpoch == 0 || f.PeerEpoch != 0 || f.Arrival != 0 || f.ReceiptAttempt != 0 {
			return errors.New("invalid data fields")
		}
	case Receipt:
		if f.Lifecycle < 1 || f.Lifecycle > 2 || f.OriginEpoch == 0 || f.PeerEpoch == 0 || f.ReceiptAttempt < f.Arrival || len(f.Content) != 0 {
			return errors.New("invalid receipt fields")
		}
	default:
		return fmt.Errorf("unsupported frame kind %d", f.Kind)
	}
	return nil
}

// TCPReadError retains consumed stream bytes, including prefix, without a complete arrival.
type TCPReadError struct {
	Bytes int
	Err   error
}

// Error describes the underlying framing/read failure without exposing partial content.
func (e *TCPReadError) Error() string { return e.Err.Error() }

// Unwrap preserves EOF/timeout/cancellation classification by callers.
func (e *TCPReadError) Unwrap() error { return e.Err }

// ReadBytes reports consumed prefix/body bytes; they do not prove logical delivery.
func (e *TCPReadError) ReadBytes() int { return e.Bytes }

// ReadTCP rejects oversized lengths before allocation; partial frames expose only byte evidence.
func ReadTCP(r io.Reader) ([]byte, error) {
	var prefix [4]byte
	read, err := io.ReadFull(r, prefix[:])
	if err != nil {
		return nil, &TCPReadError{read, err}
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n < Overhead || n > TCPMax {
		return nil, &TCPReadError{4, errors.New("invalid TCP frame length")}
	}
	b := make([]byte, int(n))
	read, err = io.ReadFull(r, b)
	if err != nil {
		return nil, &TCPReadError{4 + read, err}
	}
	return b, nil
}

// WriteTCP reports actual bytes including the prefix; callers terminate on partial writes.
func WriteTCP(w io.Writer, b []byte) (int, error) {
	if len(b) < Overhead || len(b) > TCPMax {
		return 0, errors.New("invalid TCP frame length")
	}
	p := make([]byte, len(b)+4)
	binary.BigEndian.PutUint32(p[:4], uint32(len(b)))
	copy(p[4:], b)
	total := 0
	for total < len(p) {
		n, err := w.Write(p[total:])
		total += n
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrNoProgress
		}
	}
	return total, nil
}
