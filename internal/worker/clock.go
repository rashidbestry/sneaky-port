package worker

import (
	"errors"
	"math"
	"time"

	"sneaky-port/internal/elapsed"
)

// NormalizeClock installs a single domain before accounting and worker construction.
// Legacy component fixtures deliberately retain their supplied Go anchor and offsets.
func (c *Config) NormalizeClock() error {
	if c.Clock != nil {
		if c.StartNS > math.MaxInt64 || c.ObservationNS > math.MaxInt64 || c.ObservationNS != 0 && c.ObservationNS < c.StartNS {
			return errors.New("invalid elapsed timeline")
		}
		_, err := c.Clock.Now()
		return err
	}
	if c.Start.IsZero() || c.Start.Before(c.EpochStart) {
		return errors.New("invalid legacy measurement start")
	}
	clock, err := elapsed.FromTime(c.EpochStart)
	if err != nil {
		return err
	}
	c.Clock, c.StartNS = clock, uint64(c.Start.Sub(c.EpochStart))
	if !c.ObservationEnd.IsZero() {
		if c.ObservationEnd.Before(c.Start) {
			return errors.New("observation cutoff precedes local start")
		}
		c.ObservationNS = uint64(c.ObservationEnd.Sub(c.EpochStart))
	}
	return nil
}

// Remaining uses only chosen-domain offsets; Go timers provide an operational wait budget.
func Remaining(clock *elapsed.Epoch, end uint64) (time.Duration, error) {
	if clock == nil || end > math.MaxInt64 {
		return 0, errors.New("invalid elapsed deadline")
	}
	now, err := clock.Now()
	if err != nil {
		return 0, err
	}
	if now >= end {
		return 0, nil
	}
	return time.Duration(end - now), nil
}
