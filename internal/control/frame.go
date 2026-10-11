// Package control supplies bounded application frames and application-owned SSH identities.
package control

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
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
	if err := DecodeObject(b, &out); err != nil {
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
func jsonValue(d *json.Decoder) error { return jsonDepth(d, 0) }

// jsonDepth caps nesting before walking an untrusted bounded JSON document.
func jsonDepth(d *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeds 64")
	}
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
			if err := jsonDepth(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := jsonDepth(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}

// DecodeObject rejects omitted required fields, nulls, duplicates and unknown fields; public imports may carry a UTF-8 BOM.
func DecodeObject(b []byte, out any) error {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if len(b) == 0 || len(b) > FrameMax || !utf8.Valid(b) {
		return errors.New("invalid bounded UTF-8 JSON")
	}
	if err := uniqueJSON(b); err != nil {
		return err
	}
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return errors.New("JSON destination must be a nonnil pointer")
	}
	if err := requiredFields(b, v.Elem().Type()); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}

// requiredFields checks explicit struct fields recursively while leaving opaque protocol payloads to their own decoder.
func requiredFields(b []byte, typ reflect.Type) error {
	if typ.Kind() == reflect.Pointer && bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return nil
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == reflect.TypeFor[json.RawMessage]() || typ == reflect.TypeFor[time.Time]() {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return errors.New("null JSON fields are unsupported")
	}
	switch typ.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(b, &fields); err != nil {
			return err
		}
		known := map[string]bool{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			name := tag[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			known[name] = true
			value, ok := fields[name]
			if !ok {
				if len(tag) > 1 && tag[1] == "omitempty" {
					continue
				}
				return errors.New("required JSON field missing: " + name)
			}
			if err := requiredFields(value, f.Type); err != nil {
				return err
			}
		}
		for name := range fields {
			if !known[name] {
				return errors.New("unknown JSON field: " + name)
			}
		}
	case reflect.Slice, reflect.Array:
		var items []json.RawMessage
		if err := json.Unmarshal(b, &items); err != nil {
			return err
		}
		for _, item := range items {
			if err := requiredFields(item, typ.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}
