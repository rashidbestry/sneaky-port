package measure

import (
	"context"
	"errors"
	"time"

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
	if duration == 0 || duration > 1<<63-1 || interval == 0 || duration/interval > 1<<20 {
		return nil, errors.New("invalid preflight duration/interval/sample bound")
	}
	p := schedule.Pattern{Mode: "steady", Numerator: 1e9, Denominator: interval}
	count, err := p.Count(duration)
	if err != nil {
		return nil, err
	}
	out := make([]Wake, 0, count)
	anchor := time.Now()
	next := uint64(0)
	for next < count {
		offset, err := p.Offset(next, duration)
		if err != nil {
			return out, err
		}
		if !schedule.Wait(ctx.Done(), anchor, offset) {
			return out, ctx.Err()
		}
		actual := uint64(time.Since(anchor))
		end, _, err := p.Due(next, duration, actual, ^uint64(0))
		if err != nil {
			return out, err
		}
		if end == next {
			continue
		}
		offset, err = p.Offset(end-1, duration)
		if err != nil {
			return out, err
		}
		out = append(out, Wake{profile.Decimal(offset), profile.Decimal(actual), profile.Decimal(actual - offset), profile.Decimal(end - next - 1)})
		next = end
	}
	return out, nil
}
