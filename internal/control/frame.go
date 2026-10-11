// Package control supplies bounded application frames and application-owned SSH identities.
package control

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

const FrameMax = 1 << 20

// Envelope names schema and lifecycle identities without encoding run secrets in URLs/files.
type Envelope struct {
	Version   int             `json:"schema_version"`
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	RunID     string          `json:"run_id"`
	SessionID string          `json:"session_id"`
	Body      json.RawMessage `json:"body"`
}

// Read bounds JSON allocation before decoding; lifecycle validation belongs to coordination.
func Read(r io.Reader) (Envelope, error) {
	var out Envelope
	var prefix [4]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		return out, err
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n == 0 || n > FrameMax {
		return out, errors.New("control frame length unsupported")
	}
	b := make([]byte, int(n))
	if _, err := io.ReadFull(r, b); err != nil {
		return out, err
	}
	if err := uniqueJSON(b); err != nil {
		return out, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return out, err
	}
	if out.Version != 1 || out.Type == "" || out.RequestID == "" || len(out.Body) == 0 {
		return out, errors.New("invalid control envelope")
	}
	return out, nil
}

// Write serializes one complete bounded frame; concurrent channels need separate writers.
func Write(w io.Writer, e Envelope) error {
	if e.Version != 1 || e.Type == "" || e.RequestID == "" || len(e.Body) == 0 {
		return errors.New("invalid control envelope")
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(b) > FrameMax {
		return errors.New("control frame exceeds 1 MiB")
	}
	p := make([]byte, len(b)+4)
	binary.BigEndian.PutUint32(p[:4], uint32(len(b)))
	copy(p[4:], b)
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
		p = p[n:]
	}
	return nil
}

// uniqueJSON rejects duplicate object keys and trailing documents in control frames.
func uniqueJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := jsonValue(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing control JSON")
	}
	return nil
}

// jsonValue recursively checks object-key uniqueness within a bounded document.
func jsonValue(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || keys[s] {
				return errors.New("duplicate or invalid control JSON key")
			}
			keys[s] = true
			if err := jsonValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := jsonValue(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}
