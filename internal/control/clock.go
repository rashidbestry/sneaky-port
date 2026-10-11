package control

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/profile"
)

const CalibrationChannelType = "sneaky-port/calibration/v1"
const clockFrameMax = 4096

// ClockCapture retains application boundaries, including incomplete/invalid exchanges.
// All times are monotonic offsets from the same local finite-start epochs as data.
type ClockCapture struct {
	ID              string          `json:"calibration_id"`
	ClientEpoch     uint32          `json:"client_epoch"`
	ServerEpoch     uint32          `json:"server_epoch"`
	Scheduled       profile.Decimal `json:"scheduled_ns"`
	T1              profile.Decimal `json:"t1_ns"`
	T2              profile.Decimal `json:"t2_ns"`
	T3              profile.Decimal `json:"t3_ns"`
	T4              profile.Decimal `json:"t4_ns"`
	ClientCosts     clockCosts      `json:"client_send"`
	ServerCosts     clockCosts      `json:"server_send"`
	Finished        profile.Decimal `json:"finished_ns"`
	Sent            profile.Decimal `json:"sent_bytes"`
	Received        profile.Decimal `json:"received_bytes"`
	Skipped         profile.Decimal `json:"skipped_cadence_slots"`
	Valid           bool            `json:"valid"`
	Reason          string          `json:"reason"`
	ResponseID      string          `json:"response_id,omitempty"`
	ResponseRun     string          `json:"response_run_id,omitempty"`
	ResponseSession string          `json:"response_session_id,omitempty"`
	ResponseHash    string          `json:"response_plan_hash,omitempty"`
}

// clockCosts separates serialization from channel submission and blocked send-call time.
type clockCosts struct {
	Encoded    profile.Decimal `json:"serialization_end_ns"`
	WriteStart profile.Decimal `json:"send_call_start_ns"`
	WriteEnd   profile.Decimal `json:"send_call_end_ns"`
}

// ClockSnapshot reports a bounded recent window; it is never a calibrated one-way result.
type ClockSnapshot struct {
	At          profile.Decimal     `json:"as_of_ns"`
	Status      string              `json:"status"`
	Reason      string              `json:"reason"`
	Policy      profile.Calibration `json:"policy"`
	Attempts    int                 `json:"attempts"`
	Valid       int                 `json:"valid_total"`
	Invalid     int                 `json:"invalid_total"`
	RecentValid int                 `json:"recent_valid"`
	Skipped     profile.Decimal     `json:"skipped_cadence_slots"`
	Age         *profile.Decimal    `json:"age_ns"`
	Expired     bool                `json:"expired"`
	Recent      []ClockCapture      `json:"recent"`
	Traffic     ControlTraffic      `json:"traffic"`
}

// clockRequest binds each sample to its frozen run/plan and declared client epoch.
type clockRequest struct {
	Hash  string          `json:"plan_hash"`
	Epoch uint32          `json:"client_epoch"`
	T1    profile.Decimal `json:"t1_ns"`
}

// clockResponse captures complete-request arrival and pre-serialization response submission.
type clockResponse struct {
	Hash  string          `json:"plan_hash"`
	Epoch uint32          `json:"server_epoch"`
	T1    profile.Decimal `json:"t1_ns"`
	T2    profile.Decimal `json:"t2_ns"`
	T3    profile.Decimal `json:"t3_ns"`
}

// clockBudget reserves one request per reachable cadence slot, without a startup burst.
func clockBudget(p profile.Plan) int {
	total := uint64(p.Warmup) + p.Duration() + uint64(p.Grace)
	return int(min(uint64(p.Limits.ControlRequests), (total-1)/uint64(p.Calibration.Cadence)+1))
}

// finiteRequestDemand reserves heartbeat/lifecycle and calibration admission before starting sockets.
func finiteRequestDemand(p profile.Plan) uint64 {
	total := uint64(p.Warmup) + p.Duration() + uint64(p.Grace)
	return total/uint64(p.Limits.HeartbeatInterval) + 8 + uint64(clockBudget(p))
}

// clockNow uses only Go's monotonic component; wall labels never enter elapsed timing.
func clockNow(anchor time.Time) profile.Decimal { return profile.Decimal(max(0, time.Since(anchor))) }

// clockWrite charges serialization and channel writes separately with a small independent frame cap.
func clockWrite(w io.Writer, e Envelope, anchor time.Time) (clockCosts, error) {
	b, err := json.Marshal(e)
	c := clockCosts{Encoded: clockNow(anchor)}
	if err != nil || len(b) == 0 || len(b) > clockFrameMax {
		return c, errors.New("calibration frame exceeds budget")
	}
	f := make([]byte, len(b)+4)
	binary.BigEndian.PutUint32(f, uint32(len(b)))
	copy(f[4:], b)
	c.WriteStart = clockNow(anchor)
	for len(f) > 0 {
		n, e := w.Write(f)
		if n > 0 {
			f = f[n:]
		}
		if e != nil {
			err = e
			break
		}
		if n == 0 {
			err = io.ErrNoProgress
			break
		}
	}
	c.WriteEnd = clockNow(anchor)
	return c, err
}

// clockRead captures arrival after complete frame bytes, before JSON validation.
// A truncated/oversized frame has no complete-arrival capture; zero is paired with its error.
func clockRead(r io.Reader, anchor time.Time) (Envelope, profile.Decimal, error) {
	var e Envelope
	var prefix [4]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		return e, 0, err
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n == 0 || n > clockFrameMax {
		return e, 0, errors.New("calibration frame exceeds budget")
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	at := clockNow(anchor)
	if err != nil {
		return e, 0, err
	}
	err = DecodeObject(b, &e)
	if err == nil && e.Version != 1 {
		err = errors.New("invalid calibration schema")
	}
	return e, at, err
}

// clockGate permits one calibration channel only during this authenticated finite run.
// Closing it interrupts blocked channel IO without changing the connection's heartbeat deadlines.
type clockGate struct {
	mu               sync.Mutex
	active, admitted bool
	anchor           time.Time
	ready            ReadyResult
	plan             profile.Plan
	channel          ssh.Channel
	done             chan struct{}
	raw              io.Closer
	timeout          time.Duration
	closeOnce        sync.Once
}

// start publishes immutable run identity and the actual server data epoch before start acknowledgment.
func (g *clockGate) start(anchor time.Time, ready ReadyResult, p profile.Plan) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.anchor, g.ready, g.plan, g.active = anchor, ready, p, true
	g.timeout = time.Duration(p.Limits.ShutdownTimeout)
}

// accept rejects premature/duplicate channels and owns the sole bounded server sampler worker.
func (g *clockGate) accept(ctx context.Context, incoming ssh.NewChannel) {
	g.mu.Lock()
	if !g.active || g.admitted || len(incoming.ExtraData()) != 0 {
		g.mu.Unlock()
		incoming.Reject(ssh.Prohibited, "one in-run calibration channel only")
		return
	}
	g.admitted = true
	g.done = make(chan struct{})
	anchor, ready, p, done := g.anchor, g.ready, g.plan, g.done
	g.mu.Unlock()
	go func() { // The worker joins on stop and cancellation; prohibited requests close only calibration.
		defer close(done)
		ch, reqs, err := incoming.Accept()
		if err != nil {
			return
		}
		defer ch.Close()
		g.mu.Lock()
		g.channel = ch
		active := g.active
		g.mu.Unlock()
		if !active {
			return
		}
		child, cancel := context.WithCancel(ctx)
		defer cancel()
		stop := context.AfterFunc(child, func() { // Interrupt reads/writes on loss, revocation or request floods.
			ch.Close()
		})
		defer stop()
		go rejectRequests(child, reqs, p.Limits.ControlRequests, cancel)
		serveClock(ch, anchor, ready, p)
	}()
}

// stop closes calibration and waits for its bounded channel worker before server report transfer.
func (g *clockGate) stop() {
	g.mu.Lock()
	g.active = false
	ch, done, timeout := g.channel, g.done, g.timeout
	g.mu.Unlock()
	if done != nil {
		if ch != nil {
			g.closeOnce.Do(func() { // Close can block behind a shared SSH write; retain one owned close worker.
				go ch.Close()
			})
		}
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			g.raw.Close()
			<-done
		}
	}
}

// serveClock validates unique ordered epoch-bound requests and returns send costs in a second frame.
// t3 precedes response serialization; the follow-up costs cannot retroactively alter t4.
func serveClock(ch ssh.Channel, anchor time.Time, ready ReadyResult, p profile.Plan) {
	var previous profile.Decimal
	for i := 0; i < clockBudget(p); i++ {
		e, t2, err := clockRead(ch, anchor)
		var r clockRequest
		if err != nil || e.Type != "clock" || e.RequestID != fmt.Sprintf("clock-%d", i+1) || e.RunID != ready.RunID || e.SessionID != ready.SessionID || DecodeObject(e.Body, &r) != nil || r.Hash != ready.PlanHash || r.Epoch != 1 || i > 0 && r.T1 <= previous {
			return
		}
		previous = r.T1
		t3 := clockNow(anchor)
		cost, err := clockWrite(ch, message("clock", e.RequestID, e.RunID, e.SessionID, clockResponse{r.Hash, 1, r.T1, t2, t3}), anchor)
		if err != nil {
			return
		}
		if _, err = clockWrite(ch, message("clock-costs", e.RequestID, e.RunID, e.SessionID, cost), anchor); err != nil {
			return
		}
	}
}

// clockSampler exclusively owns client calibration IO; snapshots are taken only after joined stop.
type clockSampler struct {
	cancel   context.CancelFunc
	done     chan struct{}
	anchor   time.Time
	p        profile.Plan
	ready    ReadyResult
	writer   *evidenceWriter
	window   clockWindow
	terminal string
	client   *ssh.Client
}

// startClock installs persistence's stop hook before engine finalization can close its event queue.
func startClock(ctx context.Context, client *ssh.Client, anchor time.Time, p profile.Plan, ready ReadyResult, w *evidenceWriter) *clockSampler {
	child, cancel := context.WithCancel(ctx)
	s := &clockSampler{cancel: cancel, done: make(chan struct{}), anchor: anchor, p: p, ready: ready, writer: w}
	s.client = client
	s.window = clockWindow{policy: p.Calibration, recent: make([]ClockCapture, 0, p.Calibration.Window)}
	w.beforeFinalize = func() { // Stop before closing admission; reserve cutoff space for failed calibration retention.
		s.stop()
		if s.terminal == "calibration evidence budget/storage unavailable" {
			w.clockRetention = s.terminal
		}
	}
	go s.run(child, client)
	return s
}

// stop joins the sampler independently of data; an outstanding SSH open wait is released by connection teardown.
func (s *clockSampler) stop() {
	s.cancel()
	timer := time.NewTimer(time.Duration(s.p.Limits.ShutdownTimeout))
	defer timer.Stop()
	select {
	case <-s.done:
	case <-timer.C:
		// A blocked shared SSH write cannot be cancelled at channel scope; close the owned broken connection.
		s.client.Close()
		<-s.done
	}
}

// run keeps an absolute cadence, skips missed slots without catch-up, and bounds attempts and storage.
func (s *clockSampler) run(ctx context.Context, client *ssh.Client) {
	defer close(s.done)
	// OpenChannel has no context API: at most one wait remains owned by the enclosing SSH connection.
	type opened struct {
		ch       ssh.Channel
		requests <-chan *ssh.Request
		err      error
	}
	open := make(chan opened)
	go func() { // Cancellation can abandon the open result; a late accepted channel is immediately closed.
		ch, requests, err := client.OpenChannel(CalibrationChannelType, nil)
		select {
		case open <- opened{ch, requests, err}:
		case <-ctx.Done():
			if ch != nil {
				ch.Close()
			}
		}
	}()
	var ch ssh.Channel
	openTimer := time.NewTimer(time.Duration(s.p.Limits.WriteTimeout))
	defer openTimer.Stop()
	select {
	case r := <-open:
		if r.err != nil {
			s.retain(ClockCapture{ID: "clock-1", ClientEpoch: 1, Finished: clockNow(s.anchor), Reason: "calibration channel rejected"})
			s.terminal = "calibration channel rejected"
			return
		}
		ch = r.ch
		go rejectRequests(ctx, r.requests, s.p.Limits.ControlRequests, s.cancel)
	case <-openTimer.C:
		s.cancel()
		s.retain(ClockCapture{ID: "clock-1", ClientEpoch: 1, Finished: clockNow(s.anchor), Reason: "calibration channel open timed out"})
		s.terminal = "calibration channel open timed out"
		return
	case <-ctx.Done():
		s.retain(ClockCapture{ID: "clock-1", ClientEpoch: 1, Finished: clockNow(s.anchor), Reason: "calibration channel open cancelled"})
		return
	}
	defer ch.Close()
	stop := context.AfterFunc(ctx, func() { // Channel cancellation preserves the shared connection and local RTT run.
		ch.Close()
	})
	defer stop()
	counted := &frameCounter{Channel: ch}
	total := uint64(s.p.Warmup) + s.p.Duration() + uint64(s.p.Grace)
	cadence := uint64(s.p.Calibration.Cadence)
	var slot uint64
	for i := 0; i < clockBudget(s.p); i++ {
		at := slot * cadence
		if at >= total {
			return
		}
		timer := time.NewTimer(max(0, time.Until(s.anchor.Add(time.Duration(at)))))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		now := uint64(clockNow(s.anchor))
		if now >= total {
			return
		}
		actual := now / cadence
		skipped := actual - slot
		slot = actual + 1
		c := ClockCapture{ID: fmt.Sprintf("clock-%d", i+1), ClientEpoch: 1, Scheduled: profile.Decimal(actual * cadence), Skipped: profile.Decimal(skipped)}
		beforeSent, beforeReceived := counted.sent, counted.received
		timedOut := make(chan struct{})
		timeout := time.AfterFunc(time.Duration(s.p.Limits.WriteTimeout), func() { // Close only this sampler on SSH head-of-line blockage.
			ch.Close()
			close(timedOut)
		})
		c.T1 = clockNow(s.anchor)
		var err error
		c.ClientCosts, err = clockWrite(counted, message("clock", c.ID, s.ready.RunID, s.ready.SessionID, clockRequest{s.ready.PlanHash, 1, c.T1}), s.anchor)
		if err == nil {
			err = s.response(counted, &c)
		}
		if !timeout.Stop() {
			<-timedOut
		}
		c.Finished = clockNow(s.anchor)
		c.Sent, c.Received = counted.sent-beforeSent, counted.received-beforeReceived
		if err != nil {
			c.Reason = "calibration exchange IO failed"
			if err.Error() == "invalid calibration response" || err.Error() == "invalid calibration send costs" {
				c.Reason = err.Error()
			}
		} else {
			c.Reason = validateCapture(c)
			c.Valid = c.Reason == ""
		}
		if !s.retain(c) {
			return
		}
		if err != nil {
			s.terminal = c.Reason
			return
		}
	}
}

// response keeps t4 at the first complete response, separately validating correlated send-cost evidence.
func (s *clockSampler) response(r io.Reader, c *ClockCapture) error {
	e, t4, err := clockRead(r, s.anchor)
	c.T4 = t4
	c.ResponseID, c.ResponseRun, c.ResponseSession = e.RequestID, e.RunID, e.SessionID
	var body clockResponse
	decodeErr := DecodeObject(e.Body, &body)
	c.T2, c.T3, c.ServerEpoch, c.ResponseHash = body.T2, body.T3, body.Epoch, body.Hash
	if err != nil || e.Type != "clock" || e.RequestID != c.ID || e.RunID != s.ready.RunID || e.SessionID != s.ready.SessionID || decodeErr != nil || body.Hash != s.ready.PlanHash || body.T1 != c.T1 {
		return errors.New("invalid calibration response")
	}
	e, _, err = clockRead(r, s.anchor)
	if err != nil || e.Type != "clock-costs" || e.RequestID != c.ID || e.RunID != s.ready.RunID || e.SessionID != s.ready.SessionID || DecodeObject(e.Body, &c.ServerCosts) != nil {
		return errors.New("invalid calibration send costs")
	}
	return nil
}

// validateCapture checks local order and epochs without subtracting unrelated host clocks.
func validateCapture(c ClockCapture) string {
	if c.ClientEpoch != 1 || c.ServerEpoch != 1 {
		return "invalid clock epoch"
	}
	if c.T1 > c.ClientCosts.Encoded || c.ClientCosts.Encoded > c.ClientCosts.WriteStart || c.ClientCosts.WriteStart > c.ClientCosts.WriteEnd || c.ClientCosts.WriteEnd > c.T4 || c.T1 > c.T4 || c.T4 > c.Finished || c.T2 > c.T3 || c.T3 > c.ServerCosts.Encoded || c.ServerCosts.Encoded > c.ServerCosts.WriteStart || c.ServerCosts.WriteStart > c.ServerCosts.WriteEnd {
		return "invalid local capture order"
	}
	return ""
}

// retain admits raw captures before exposing them; storage failure affects calibration only.
func (s *clockSampler) retain(c ClockCapture) bool {
	if err := s.writer.appendClock(c); err != nil {
		s.terminal = "calibration evidence budget/storage unavailable"
		return false
	}
	s.window.add(c)
	return true
}

// appendClock reserves calibration file bytes independently of data, using the same ordered disk queue.
// Saturation rejects calibration without marking otherwise valid measurement evidence as failed.
func (w *evidenceWriter) appendClock(c ClockCapture) error {
	details, err := json.Marshal(c)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.err != nil {
		return errors.New("calibration storage unavailable")
	}
	b, err := w.event("clock_capture", c.Finished, details)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	n := uint64(len(b))
	if n > clockFrameMax || n > w.clockLimit-w.clockCharged || w.pending >= uint64(cap(w.queue)) || n > w.queueBytes-w.queuedBytes {
		return errors.New("calibration evidence budget exhausted")
	}
	w.queue <- b
	w.sequence++
	w.pending++
	w.clockCharged += n
	w.queuedBytes += n
	return nil
}

// clockWindow retains only Window recent attempts while preserving exact run-wide counters.
type clockWindow struct {
	policy profile.Calibration
	recent []ClockCapture
	totals ClockSnapshot
	last   profile.Decimal
}

// add accounts one persisted attempt and evicts the oldest recent attempt, including invalid samples.
func (w *clockWindow) add(c ClockCapture) {
	w.totals.Attempts++
	if c.Sent > 0 {
		w.totals.Traffic.Requests++
	}
	w.totals.Traffic.Sent += c.Sent
	w.totals.Traffic.Received += c.Received
	w.totals.Skipped += c.Skipped
	if c.Valid {
		w.totals.Valid++
		w.last = c.T4
	} else {
		w.totals.Invalid++
	}
	if len(w.recent) == w.policy.Window {
		copy(w.recent, w.recent[1:])
		w.recent[len(w.recent)-1] = c
	} else {
		w.recent = append(w.recent, c)
	}
}

// snapshot derives expiry at the named local offset without changing old samples or mapping epochs.
// Samples is the minimum recent valid count; Window caps attempts, including invalid ones.
func (w *clockWindow) snapshot(now profile.Decimal) *ClockSnapshot {
	out := w.totals
	out.Status, out.Reason = "unavailable", "insufficient recent valid clock samples"
	out.Policy, out.At = w.policy, now
	out.Traffic.Scope = "calibration application frames only; excludes SSH/network overhead"
	out.Recent = append([]ClockCapture{}, w.recent...)
	if out.Valid > 0 && now >= w.last {
		age := now - w.last
		out.Age = &age
		out.Expired = age > w.policy.Expiry
	}
	for _, c := range out.Recent {
		if c.Valid && now >= c.T4 && now-c.T4 <= w.policy.Expiry {
			out.RecentValid++
		}
	}
	if out.Expired {
		out.Reason = "clock samples expired"
	} else if out.RecentValid >= w.policy.Samples {
		out.Status = "samples_available"
		out.Reason = "raw constraints only; clock model and one-way results not integrated"
	}
	return &out
}

// clockSnapshot replays every retained raw attempt through the same bounded recent-window accounting.
func clockSnapshot(p profile.Plan, captures []ClockCapture, now profile.Decimal) *ClockSnapshot {
	w := clockWindow{policy: p.Calibration, recent: make([]ClockCapture, 0, p.Calibration.Window)}
	for _, c := range captures {
		w.add(c)
	}
	return w.snapshot(now)
}
