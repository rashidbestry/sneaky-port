// Package extension supplies a test fixture only; production commands never import it.
// It wraps real framed TCP with a distinct authenticated identity and option/frame limits.
package extension

import (
	"context"
	"encoding/json"
	"errors"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

const ID = "test-extension"
const MaxFrame = 1024

// Factory owns test option validation while delegating real socket/framing ownership.
type Factory struct{ base adapter.Factory }

// Register adds the fixture to an isolated startup registry, never the production default.
func Register(r *adapter.Registry) error {
	base, err := r.Get("tcp")
	if err != nil {
		return err
	}
	return r.Register(Factory{base})
}

// Describe retains TCP framing costs but advertises the test identity and smaller frame limit.
func (f Factory) Describe() adapter.Descriptor {
	d := f.base.Describe()
	d.ID, d.MaxFrame = ID, MaxFrame
	d.Options = map[string]string{"enabled": "required boolean"}
	return d
}

// Validate rejects incompatible versions, options and capabilities before any socket opens.
func (f Factory) Validate(p profile.Plan) error {
	if p.Adapter != ID || p.Contract != 1 || len(p.Options) != 1 || string(p.Options["enabled"]) != "true" {
		return errors.New("test extension requires contract 1 and enabled=true")
	}
	for _, ph := range p.Phases {
		if ph.Client.ContentBytes > MaxFrame-probe.Overhead || ph.Server.ContentBytes > MaxFrame-probe.Overhead {
			return errors.New("test extension frame limit exceeded")
		}
	}
	return f.base.Validate(basePlan(p))
}

// basePlan translates only the fixture's private socket options; the frozen public identity stays intact.
func basePlan(p profile.Plan) profile.Plan {
	p.Adapter = "tcp"
	p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
	return p
}

// Prepare validates the extension and creates only its real framed listener.
func (f Factory) Prepare(ctx context.Context, p profile.Plan) (adapter.Prepared, error) {
	if err := f.Validate(p); err != nil {
		return nil, err
	}
	return f.base.Prepare(ctx, basePlan(p))
}

// Dial validates the extension and creates only its real framed connection.
func (f Factory) Dial(ctx context.Context, p profile.Plan) (adapter.Transport, error) {
	if err := f.Validate(p); err != nil {
		return nil, err
	}
	return f.base.Dial(ctx, basePlan(p))
}
