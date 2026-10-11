// Package profile validates the shared versioned YAML/JSON measurement plan.
package profile

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
	"sneaky-port/internal/schedule"
)

const DocumentMax = 1 << 20

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Decimal keeps unsigned 64-bit quantities exact in browsers, YAML and exports.
type Decimal uint64

// MarshalJSON writes a decimal string instead of a lossy JavaScript number.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(d), 10))
}

// UnmarshalJSON rejects numbers, signs, leading zeroes and unsigned overflow.
func (d *Decimal) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return errors.New("64-bit quantities must be decimal strings")
	}
	v, err := parseDecimal(s)
	if err != nil {
		return err
	}
	*d = Decimal(v)
	return nil
}

// parseDecimal accepts canonical nonnegative decimal text only.
func parseDecimal(s string) (uint64, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || strconv.FormatUint(v, 10) != s {
		return 0, errors.New("noncanonical unsigned decimal string")
	}
	return v, nil
}

// Direction describes physical traffic independently of reverse receipts.
type Direction struct {
	Mode            string  `json:"mode"`
	RateNumerator   Decimal `json:"rate_numerator"`
	RateDenominator Decimal `json:"rate_denominator"`
	ContentBytes    int     `json:"content_bytes"`
	ContentPattern  string  `json:"content_pattern_base64"`
	Active          Decimal `json:"active_ns"`
	Gap             Decimal `json:"gap_ns"`
	DSCP            int     `json:"dscp"`
}

// Pattern exposes scheduling parameters without transport or UI knowledge.
func (d Direction) Pattern() schedule.Pattern {
	return schedule.Pattern{Mode: d.Mode, Numerator: uint64(d.RateNumerator), Denominator: uint64(d.RateDenominator), Active: uint64(d.Active), Gap: uint64(d.Gap)}
}

// Payload expands an already validated repeated-byte content pattern.
func (d Direction) Payload() []byte {
	b, _ := base64.StdEncoding.DecodeString(d.ContentPattern)
	out := make([]byte, d.ContentBytes)
	for i := range out {
		out[i] = b[i%len(b)]
	}
	return out
}

// Phase defines half-open traffic windows and an optional explicit deadline override.
type Phase struct {
	ID       string    `json:"phase_id"`
	Duration Decimal   `json:"duration_ns"`
	Client   Direction `json:"client"`
	Server   Direction `json:"server"`
	Deadline *Decimal  `json:"deadline_ns,omitempty"`
}

// Limits bounds queues, ledgers, persistence, I/O and control leases explicitly.
type Limits struct {
	QueueMessages     int     `json:"queue_messages"`  // Waiting plus active frames, including receipts.
	QueueBytes        Decimal `json:"queue_bytes"`     // Common authenticated bytes; excludes TCP prefix/outer overhead.
	ReceiptReserve    int     `json:"receipt_reserve"` // Data cannot consume these slots or N * 124 receipt bytes.
	Outstanding       int     `json:"outstanding_data"`
	LedgerMessages    int     `json:"ledger_messages"`
	EventBytes        Decimal `json:"event_bytes"`
	FileBytes         Decimal `json:"file_bytes"`
	MaxLag            Decimal `json:"max_schedule_lag_ns"`
	WriteTimeout      Decimal `json:"write_timeout_ns"`
	SetupTimeout      Decimal `json:"setup_timeout_ns"`
	ShutdownTimeout   Decimal `json:"shutdown_timeout_ns"`
	HeartbeatInterval Decimal `json:"heartbeat_interval_ns"`
	Lease             Decimal `json:"lease_ns"`
	ControlRequests   int     `json:"control_requests"`
}

// Calibration retains explicit sample/bound inputs and their provenance.
type Calibration struct {
	Samples        int     `json:"samples"`
	Window         int     `json:"window"`
	Cadence        Decimal `json:"cadence_ns"`
	Expiry         Decimal `json:"expiry_ns"`
	MaxError       Decimal `json:"max_error_ns"`
	TimestampError Decimal `json:"timestamp_error_ns"`
	ModelError     Decimal `json:"model_error_ns"`
	DriftPPB       Decimal `json:"drift_ppb"`
	BoundSource    string  `json:"bound_source"`
}

// Plan is the normalized, secret-free version-1 request shared across components.
type Plan struct {
	Version           int                        `json:"schema_version"`
	ID                string                     `json:"profile_id"`
	PairingRef        string                     `json:"pairing_ref"`
	Adapter           string                     `json:"adapter_id"`
	Contract          int                        `json:"adapter_contract_version"`
	Family            string                     `json:"address_family"`
	ServerAddress     string                     `json:"server_address"`
	DestinationPort   int                        `json:"destination_port"`
	SourcePort        string                     `json:"source_port"`
	Options           map[string]json.RawMessage `json:"adapter_options"`
	Warmup            Decimal                    `json:"warmup_ns"`
	Phases            []Phase                    `json:"phases"`
	Grace             Decimal                    `json:"grace_ns"`
	Deadline          Decimal                    `json:"receipt_deadline_ns"`
	Limits            Limits                     `json:"limits"`
	Calibration       Calibration                `json:"calibration"`
	PreflightDuration Decimal                    `json:"preflight_duration_ns"`
	PreflightInterval Decimal                    `json:"preflight_interval_ns"`
	SpikeThreshold    Decimal                    `json:"spike_threshold_ns"`
	ComparisonAxes    []string                   `json:"comparison_axes"`
	MatchedMetadata   map[string]string          `json:"matched_metadata"`
}

// Decode bounds input, rejects ambiguous YAML and uses one strict JSON validator.
func Decode(r io.Reader) (Plan, error) {
	var p Plan
	b, err := io.ReadAll(io.LimitReader(r, DocumentMax+1))
	if err != nil {
		return p, err
	}
	if len(b) > DocumentMax {
		return p, errors.New("profile exceeds 1 MiB")
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var node yaml.Node
	if err = dec.Decode(&node); err != nil {
		return p, err
	}
	var extra yaml.Node
	if err = dec.Decode(&extra); err != io.EOF {
		return p, errors.New("exactly one document required")
	}
	if len(node.Content) != 1 {
		return p, errors.New("empty profile")
	}
	value, err := yamlValue(node.Content[0])
	if err != nil {
		return p, err
	}
	if err = requiredFields(value, reflect.TypeOf(p), "profile"); err != nil {
		return p, err
	}
	b, err = json.Marshal(value)
	if err != nil {
		return p, err
	}
	jd := json.NewDecoder(bytes.NewReader(b))
	jd.DisallowUnknownFields()
	err = jd.Decode(&p)
	if err != nil {
		return p, err
	}
	return p, p.Validate()
}

// requiredFields distinguishes explicit zero/quiet settings from omitted required inputs.
func requiredFields(value any, t reflect.Type, path string) error {
	if t.Kind() == reflect.Pointer {
		if value == nil {
			return fmt.Errorf("%s must be omitted or explicit and non-null", path)
		}
		return requiredFields(value, t.Elem(), path)
	}
	if value == nil {
		return fmt.Errorf("%s must be explicit and non-null", path)
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be a mapping", path)
		}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			tag := strings.Split(field.Tag.Get("json"), ",")
			name := tag[0]
			if name == "" {
				continue
			}
			v, exists := m[name]
			optional := len(tag) > 1 && tag[1] == "omitempty"
			if !exists {
				if optional {
					continue
				}
				return fmt.Errorf("%s.%s is required", path, name)
			}
			if err := requiredFields(v, field.Type, path+"."+name); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		values, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be a sequence", path)
		}
		for i, v := range values {
			if err := requiredFields(v, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// yamlValue rejects aliases, custom/explicit tags, merge and duplicate mapping keys.
func yamlValue(n *yaml.Node) (any, error) {
	if n.Kind == yaml.AliasNode || n.Style&yaml.TaggedStyle != 0 {
		return nil, errors.New("aliases and explicit tags are unsupported")
	}
	switch n.Kind {
	case yaml.MappingNode:
		m := map[string]any{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || k.Value == "<<" {
				return nil, errors.New("mapping keys must be strings; merge unsupported")
			}
			if _, exists := m[k.Value]; exists {
				return nil, fmt.Errorf("duplicate key %s", k.Value)
			}
			v, err := yamlValue(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m[k.Value] = v
		}
		return m, nil
	case yaml.SequenceNode:
		out := make([]any, len(n.Content))
		for i, c := range n.Content {
			v, err := yamlValue(c)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!str":
			return n.Value, nil
		case "!!null":
			return nil, nil
		case "!!bool":
			v, err := strconv.ParseBool(n.Value)
			return v, err
		case "!!int":
			v, err := strconv.ParseInt(n.Value, 10, 32)
			if err != nil {
				return nil, errors.New("small fields require base-10 32-bit integers; quote 64-bit quantities")
			}
			return v, nil
		default:
			return nil, errors.New("unsupported scalar tag or floating point value")
		}
	}
	return nil, errors.New("unsupported YAML node")
}

// Validate applies shared schema, workload and resource rules before sockets open.
func (p Plan) Validate() error {
	if p.Warmup > math.MaxInt64 || p.Grace > math.MaxInt64 || p.Deadline > math.MaxInt64 {
		return errors.New("run timing exceeds monotonic range")
	}
	if p.Version != 1 || p.Contract != 1 {
		return errors.New("unsupported schema/adapter contract")
	}
	if !SafeID(p.ID) || !SafeID(p.PairingRef) || !SafeID(p.Adapter) {
		return errors.New("invalid profile/pairing/adapter ID")
	}
	ip := net.ParseIP(p.ServerAddress)
	if ip == nil || p.Family != "ipv4" && p.Family != "ipv6" || (ip.To4() != nil) != (p.Family == "ipv4") {
		return errors.New("explicit server IP must match address_family")
	}
	if p.DestinationPort < 1 || p.DestinationPort > 65535 {
		return errors.New("invalid destination_port")
	}
	if p.SourcePort != "auto" {
		n, e := strconv.Atoi(p.SourcePort)
		if e != nil || n < 1 || n > 65535 || strconv.Itoa(n) != p.SourcePort {
			return errors.New("source_port must be auto or canonical port string")
		}
	}
	if p.Options == nil {
		return errors.New("adapter_options must be explicit")
	}
	if len(p.Phases) == 0 || len(p.Phases) > 1024 {
		return errors.New("1-1024 phases required")
	}
	if p.Deadline == 0 || p.SpikeThreshold == 0 {
		return errors.New("deadline and spike threshold must be positive")
	}
	l := p.Limits
	if l.QueueMessages < 2 || l.QueueMessages > 1<<20 || l.ReceiptReserve < 1 || l.ReceiptReserve >= l.QueueMessages || l.Outstanding < 1 || l.LedgerMessages < 1 || l.LedgerMessages > 1<<20 || l.ControlRequests < 1 || l.ControlRequests > 1<<20 || l.QueueBytes < 248 || l.QueueBytes > 1<<30 || l.EventBytes < 1024 || l.EventBytes > 1<<30 || l.FileBytes < l.EventBytes || l.FileBytes > 1<<40 {
		return errors.New("invalid or infeasible resource limits")
	}
	for _, v := range []Decimal{l.WriteTimeout, l.SetupTimeout, l.ShutdownTimeout, l.HeartbeatInterval, l.Lease} {
		if v == 0 || v > math.MaxInt64 {
			return errors.New("timeouts require positive monotonic durations")
		}
	}
	if l.Lease <= l.HeartbeatInterval || l.MaxLag > math.MaxInt64 {
		return errors.New("invalid heartbeat lease/scheduling allowance")
	}
	total := uint64(p.Warmup)
	slots := [2]uint64{}
	phases := map[string]bool{}
	for index, ph := range p.Phases {
		if !SafeID(ph.ID) || phases[ph.ID] || ph.Duration == 0 || uint64(ph.Duration) > math.MaxInt64-total {
			return errors.New("invalid, duplicate or overflowing phase")
		}
		phases[ph.ID] = true
		total += uint64(ph.Duration)
		if ph.Deadline != nil && (*ph.Deadline == 0 || *ph.Deadline > math.MaxInt64) {
			return errors.New("invalid phase deadline")
		}
		for role, d := range []Direction{ph.Client, ph.Server} {
			if d.DSCP < 0 || d.DSCP > 63 || d.ContentBytes < 0 || d.ContentBytes > (1<<20)-124 {
				return errors.New("invalid DSCP/content length")
			}
			b, err := base64.StdEncoding.DecodeString(d.ContentPattern)
			if err != nil || base64.StdEncoding.EncodeToString(b) != d.ContentPattern || d.ContentBytes > 0 && len(b) == 0 {
				return errors.New("invalid content pattern")
			}
			n, err := d.Pattern().Count(uint64(ph.Duration))
			if err != nil {
				return fmt.Errorf("phase %s: %w", ph.ID, err)
			}
			if n > uint64(l.LedgerMessages)-slots[role] {
				return errors.New("planned slots exceed ledger budget")
			}
			slots[role] += n
			if index == 0 {
				n, err = d.Pattern().Count(uint64(p.Warmup))
				if err != nil {
					return err
				}
				if n > uint64(l.LedgerMessages)-slots[role] {
					return errors.New("warmup exceeds ledger budget")
				}
				slots[role] += n
			}
			if uint64(d.ContentBytes+124) > uint64(l.QueueBytes)/(uint64(l.ReceiptReserve)+1) {
				return errors.New("queue bytes cannot hold data plus reserved receipts")
			}
		}
	}
	if p.Grace > math.MaxInt64 || total > math.MaxInt64-uint64(p.Grace) || p.Deadline > math.MaxInt64 {
		return errors.New("run duration overflow")
	}
	// Each slot creates bounded origin/receiver/receipt evidence; storage must cover both directions.
	if slots[0]+slots[1] > uint64(l.LedgerMessages) || (slots[0]+slots[1])*4096+16384 > uint64(l.EventBytes) {
		return errors.New("predicted ledger/event demand exceeds explicit budgets")
	}
	c := p.Calibration
	if c.Samples < 2 || c.Samples > 4096 || c.Window < c.Samples || c.Window > 4096 || c.Cadence == 0 || c.Expiry <= c.Cadence || c.MaxError == 0 || c.DriftPPB >= 1e9 || strings.TrimSpace(c.BoundSource) == "" || !utf8.ValidString(c.BoundSource) {
		return errors.New("explicit calibration window/cadence/expiry/error/source required")
	}
	for _, v := range []Decimal{c.Cadence, c.Expiry, c.MaxError, c.TimestampError, c.ModelError, c.DriftPPB, p.PreflightDuration, p.PreflightInterval} {
		if v > math.MaxInt64 {
			return errors.New("calibration/preflight quantity overflow")
		}
	}
	if (p.PreflightDuration == 0) != (p.PreflightInterval == 0) || p.PreflightDuration > 0 && (p.PreflightInterval == 0 || p.PreflightDuration/p.PreflightInterval > 1<<20) {
		return errors.New("invalid preflight sample budget")
	}
	if p.ComparisonAxes == nil || p.MatchedMetadata == nil {
		return errors.New("comparison axes and metadata must be explicit")
	}
	for k, v := range p.MatchedMetadata {
		if !SafeID(k) || !utf8.ValidString(v) {
			return errors.New("metadata keys require ASCII IDs")
		}
	}
	seenAxes := map[string]bool{}
	for _, axis := range p.ComparisonAxes {
		if axis != "adapter" && axis != "port" && axis != "client_dscp" && axis != "server_dscp" || seenAxes[axis] {
			return errors.New("unsupported or duplicate comparison axis")
		}
		seenAxes[axis] = true
	}
	return nil
}

// SafeID prevents browser-controlled path traversal and bounds persistent names.
func SafeID(s string) bool { return safeID.MatchString(s) }

// Duration returns the measured duration; warmup and grace are separate boundaries.
func (p Plan) Duration() uint64 {
	var n uint64
	for _, ph := range p.Phases {
		n += uint64(ph.Duration)
	}
	return n
}

// Hash freezes the canonical public normalized plan without secrets or floats.
func (p Plan) Hash() ([32]byte, error) {
	b, err := Canonical(p)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(b), nil
}
