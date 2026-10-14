package measure

import (
	"context"
	"errors"
	"sneaky-port/internal/elapsed"

	"sneaky-port/internal/profile"
	"sneaky-port/internal/schedule"
)

// Wake records local timer evidence only; it is not network RTT or clock calibration.
type Wake struct {
	Scheduled profile.Decimal `json:"scheduled_ns"`
	Actual    profile.Decimal `json:"actual_ns"`
	Lag       profile.Decimal `json:"lag_ns"`
	Missed    profile.Decimal `json:"earlier_due_slots"`
}

// Preflight measures absolute monotonic timer lag with explicit duration/interval/bounds.
func Preflight(ctx context.Context, duration, interval uint64) ([]Wake, error) {
	wakes, _, err := PreflightObserved(ctx, duration, interval)
	return wakes, err
}

// PreflightObserved retains the actual clock provenance from the same observation epoch.
func PreflightObserved(ctx context.Context, duration, interval uint64) ([]Wake, *elapsed.Metadata, error) {
	if duration == 0 || duration > 1<<63-1 || interval == 0 || duration/interval > 1<<20 {
		return nil, nil, errors.New("invalid preflight duration/interval/sample bound")
	}
	p := schedule.Pattern{Mode: "steady", Numerator: 1e9, Denominator: interval}
	count, err := p.Count(duration)
	if err != nil {
		return nil, nil, err
	}
	out := make([]Wake, 0, count)
	anchor, err := elapsed.New()
	if err != nil {
		return nil, nil, err
	}
	metadata := anchor.Metadata()
	next := uint64(0)
	for next < count {
		offset, err := p.Offset(next, duration)
		if err != nil {
			return out, &metadata, err
		}
		waiting, err := schedule.WaitEpoch(ctx.Done(), anchor, offset)
		if err != nil {
			return out, &metadata, err
		}
		if !waiting {
			return out, &metadata, ctx.Err()
		}
		actual, err := anchor.Now()
		if err != nil {
			return out, &metadata, err
		}
		end, _, err := p.Due(next, duration, actual, ^uint64(0))
		if err != nil {
			return out, &metadata, err
		}
		if end == next {
			continue
		}
		offset, err = p.Offset(end-1, duration)
		if err != nil {
			return out, &metadata, err
		}
		out = append(out, Wake{profile.Decimal(offset), profile.Decimal(actual), profile.Decimal(actual - offset), profile.Decimal(end - next - 1)})
		next = end
	}
	return out, &metadata, nil
}
