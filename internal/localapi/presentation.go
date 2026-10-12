package localapi

import (
	"errors"
	"net/url"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
	"strconv"
)

// presentationQuery accepts canonical bounded preferences alongside only the route's own selectors.
func presentationQuery(q url.Values, selectors ...string) (measure.EvidencePolicy, error) {
	p := measure.DefaultEvidencePolicy()
	fields := map[string]*profile.Decimal{"median_target": &p.Median, "p95_target": &p.P95, "p99_target": &p.P99, "repeat_target": &p.Repeated}
	allowed := map[string]bool{}
	for _, name := range selectors {
		allowed[name] = true
	}
	for name, values := range q {
		if len(values) != 1 {
			return p, errors.New("duplicate query field")
		}
		if target, ok := fields[name]; ok {
			v, err := strconv.ParseUint(values[0], 10, 64)
			if err != nil || strconv.FormatUint(v, 10) != values[0] {
				return p, errors.New("canonical target required")
			}
			*target = profile.Decimal(v)
		} else if !allowed[name] {
			return p, errors.New("unknown query field")
		}
	}
	return p, p.Validate()
}
