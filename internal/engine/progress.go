package engine

import (
	"errors"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/worker"
)

// trackProgress updates optional count contributions after successful authoritative accounting only.
// Peer receipts/data retain their original identity; final-cutoff equality remains excluded.
func (c *Collector) trackProgress(e worker.Event) {
	if c.live == nil || e.At >= c.finalEnd {
		return
	}
	cutoff := min(c.finalEnd, e.At+1)
	count := uint64(1)
	if e.Kind == "schedule_skip" {
		count = e.Count
	}
	for i := uint64(0); i < count; i++ {
		id := identity(e.Frame)
		id.Sequence += profile.Decimal(i)
		c.ledger.TrackProgress(c.live, id, cutoff)
	}
	if e.Kind == "arrival" && e.Frame.Kind == probe.Data {
		c.peerReceived++
	}
}

// publishProgress publishes at most four times per second plus a terminal update; final reports supersede it.
// Cutoff/elapsed are host-local; no synchronized server progress or complete remote delivery is inferred.
func (c *Collector) publishProgress(feed *measure.ProgressFeed, cfg worker.Config, terminal bool) {
	if feed == nil {
		return
	}
	now, err := cfg.Clock.Now()
	if err != nil {
		c.err = errors.Join(c.err, err)
	}
	cutoff := min(now, c.finalEnd)
	if c.observation > 0 {
		c.live.Limit(c.observation)
		cutoff = min(cutoff, c.observation)
	}
	if !terminal && c.progressPublished && now-c.lastProgress < 250000000 {
		return
	}
	c.progressPublished = true
	c.lastProgress = now
	main, warmup := c.live.Counts(cutoff)
	elapsed := uint64(0)
	if now > c.start {
		elapsed = min(now-c.start, c.finalEnd-c.start)
	}
	p := measure.Progress{AsOf: profile.Decimal(now), Cutoff: profile.Decimal(cutoff), AccountingValid: c.err == nil, Elapsed: profile.Decimal(elapsed), Total: profile.Decimal(c.finalEnd - c.start), Stage: "setup", Main: main, Warmup: warmup, PeerReceived: c.peerReceived}
	p.RTTTime = c.live.RTTRanges(c.start)
	if now >= c.start {
		p.Stage = "grace"
		for _, w := range c.windows {
			if now >= c.start+w.Offset && now < c.start+w.Offset+w.Local.Duration {
				p.Stage = "measurement"
				if w.Local.Lifecycle == 1 {
					p.Stage = "warmup"
				} else {
					p.Phase = c.plan.Phases[w.Local.Phase].ID
				}
				break
			}
		}
	}
	if terminal || now >= c.finalEnd {
		p.Stage = "finalizing"
	}
	feed.Publish(p)
}
