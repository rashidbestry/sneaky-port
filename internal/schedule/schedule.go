// Package schedule computes rational, half-open workload slots without wall clocks.
package schedule

import (
	"errors"
	"math/big"
	"time"
)

// Pattern represents explicit messages/second and optional active/gap intervals.
type Pattern struct {
	Mode                                string
	Numerator, Denominator, Active, Gap uint64
}

// Validate rejects unrepresentable subnanosecond spacing and overflowing burst cycles.
func (p Pattern) Validate() error {
	if p.Mode == "quiet" {
		if p.Numerator != 0 || p.Denominator == 0 || p.Active != 0 || p.Gap != 0 {
			return errors.New("quiet requires zero rate, positive denominator and no burst")
		}
		return nil
	}
	if p.Mode != "steady" && p.Mode != "burst" {
		return errors.New("unsupported traffic mode")
	}
	if p.Numerator == 0 || p.Denominator == 0 {
		return errors.New("rate requires positive numerator and denominator")
	}
	step := new(big.Int).Mul(new(big.Int).SetUint64(p.Denominator), big.NewInt(1e9))
	if step.Cmp(new(big.Int).SetUint64(p.Numerator)) < 0 {
		return errors.New("rate exceeds one slot per nanosecond")
	}
	if p.Mode == "burst" {
		if p.Active == 0 || p.Gap == 0 || p.Active > 1<<63-1 || p.Gap > 1<<63-1 || p.Active > uint64(1<<63-1)-p.Gap {
			return errors.New("invalid burst active/gap")
		}
	} else if p.Active != 0 || p.Gap != 0 {
		return errors.New("steady has no active/gap")
	}
	return nil
}

// countActive calculates ceil(duration*n/(d*1e9)) without allocating planned slots.
func (p Pattern) countActive(duration uint64) *big.Int {
	n := new(big.Int).Mul(new(big.Int).SetUint64(duration), new(big.Int).SetUint64(p.Numerator))
	d := new(big.Int).Mul(new(big.Int).SetUint64(p.Denominator), big.NewInt(1e9))
	n.Add(n, new(big.Int).Sub(d, big.NewInt(1)))
	return n.Div(n, d)
}

// Count checks the exact half-open slot count including a truncated final burst.
func (p Pattern) Count(duration uint64) (uint64, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if duration > 1<<63-1 {
		return 0, errors.New("duration exceeds monotonic range")
	}
	if p.Mode == "quiet" || duration == 0 {
		return 0, nil
	}
	n := p.countActive(duration)
	if p.Mode == "burst" {
		cycle := p.Active + p.Gap
		full, remainder := duration/cycle, duration%cycle
		if remainder > p.Active {
			remainder = p.Active
		}
		n = new(big.Int).Mul(p.countActive(p.Active), new(big.Int).SetUint64(full))
		n.Add(n, p.countActive(remainder))
	}
	if !n.IsUint64() {
		return 0, errors.New("slot count overflow")
	}
	return n.Uint64(), nil
}

// Offset locates a slot using exact rational division, never repeated rounded intervals.
func (p Pattern) Offset(index, duration uint64) (uint64, error) {
	n, err := p.Count(duration)
	if err != nil {
		return 0, err
	}
	if index >= n {
		return 0, errors.New("slot outside phase")
	}
	base := uint64(0)
	if p.Mode == "burst" {
		per := p.countActive(p.Active).Uint64()
		base = (index / per) * (p.Active + p.Gap)
		index %= per
	}
	value := new(big.Int).Mul(new(big.Int).SetUint64(index), new(big.Int).SetUint64(p.Denominator))
	value.Mul(value, big.NewInt(1e9))
	value.Div(value, new(big.Int).SetUint64(p.Numerator))
	if !value.IsUint64() || value.Uint64() > duration-base {
		return 0, errors.New("slot offset overflow")
	}
	return base + value.Uint64(), nil
}

// Due returns all due indices and only the latest admissible slot at a delayed wake.
func (p Pattern) Due(next, duration, elapsed, allowance uint64) (end uint64, send bool, err error) {
	n, err := p.Count(duration)
	if err != nil {
		return next, false, err
	}
	if next >= n {
		return next, false, nil
	}
	lo, hi := next, n
	for lo < hi {
		mid := lo + (hi-lo)/2
		offset, e := p.Offset(mid, duration)
		if e != nil {
			return next, false, e
		}
		if offset <= elapsed {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == next {
		return next, false, nil
	}
	offset, err := p.Offset(lo-1, duration)
	return lo, elapsed-offset <= allowance && elapsed < duration, err
}

// Wait sleeps against an absolute monotonic anchor; cancellation never depends on UI.
func Wait(done <-chan struct{}, anchor time.Time, offset uint64) bool {
	delay := time.Until(anchor.Add(time.Duration(offset)))
	if delay <= 0 {
		select {
		case <-done:
			return false
		default:
			return true
		}
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-done:
		return false
	case <-t.C:
		return true
	}
}
