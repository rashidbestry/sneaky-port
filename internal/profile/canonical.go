package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// Canonical implements RFC8785 for this integer/string-only plan domain.
func Canonical(value any) ([]byte, error) {
	if err := validText(reflect.ValueOf(value)); err != nil {
		return nil, err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err = dec.Decode(&v); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = canonicalValue(&out, v)
	return out.Bytes(), err
}

// validText prevents JSON marshaling from silently replacing invalid UTF-8 before hashing.
func validText(value reflect.Value) error {
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if !value.IsNil() {
			return validText(value.Elem())
		}
	case reflect.String:
		if !utf8.ValidString(value.String()) {
			return errors.New("invalid UTF-8 in canonical input")
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).PkgPath == "" {
				if err := validText(value.Field(i)); err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			if err := validText(iterator.Key()); err != nil {
				return err
			}
			if err := validText(iterator.Value()); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return nil
		}
		for i := 0; i < value.Len(); i++ {
			if err := validText(value.Index(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// canonicalValue sorts keys by UTF-16 and rejects floats outside the plan contract.
func canonicalValue(out *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(x))
	case string:
		return canonicalString(out, x)
	case json.Number:
		n, e := strconv.ParseInt(string(x), 10, 64)
		if e != nil || n > 9007199254740991 || n < -9007199254740991 {
			return errors.New("canonical plan numbers must be safe integers")
		}
		out.WriteString(strconv.FormatInt(n, 10))
	case []any:
		out.WriteByte('[')
		for i, a := range x {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := canonicalValue(out, a); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { // Compare RFC8785 property names by UTF-16 code units.
			a, b := utf16.Encode([]rune(keys[i])), utf16.Encode([]rune(keys[j]))
			for k := 0; k < len(a) && k < len(b); k++ {
				if a[k] != b[k] {
					return a[k] < b[k]
				}
			}
			return len(a) < len(b)
		})
		out.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := canonicalString(out, k); err != nil {
				return err
			}
			out.WriteByte(':')
			if err := canonicalValue(out, x[k]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return errors.New("unsupported canonical value")
	}
	return nil
}

// canonicalString emits minimal JSON escapes and preserves valid Unicode scalars.
func canonicalString(out *bytes.Buffer, s string) error {
	if !utf8.ValidString(s) {
		return errors.New("invalid UTF-8 in plan")
	}
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\b':
			out.WriteString(`\b`)
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\f':
			out.WriteString(`\f`)
		case '\r':
			out.WriteString(`\r`)
		default:
			if r < 32 {
				out.WriteString(`\u00`)
				const digits = "0123456789abcdef"
				out.WriteByte(digits[r>>4])
				out.WriteByte(digits[r&15])
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return nil
}
