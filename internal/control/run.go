package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/adapter"
	"sneaky-port/internal/elapsed"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/worker"
)

// startAck establishes fresh run-local epoch identities, without cross-host timestamps or simultaneity claims.
type startAck struct {
	Hash      string `json:"plan_hash"`
	Epoch     uint32 `json:"server_epoch"`
	PeerEpoch uint32 `json:"client_epoch"`
}

// finishAck reports lifecycle status only; it cannot establish remote delivery or reconcile server evidence.
type finishAck struct {
	Hash   string `json:"plan_hash"`
	Status string `json:"status"`
}

// LocalRun holds endpoint-only quantities shared by CLI snapshots and persisted summary envelopes.
// Summary delivery stays unknown until compatible authenticated remote evidence is reconciled.
type LocalRun struct {
	Complete    bool              `json:"complete"`
	Generation  profile.Decimal   `json:"generation_cutoff_ns"`
	Observation profile.Decimal   `json:"observation_cutoff_ns"`
	Summary     measure.Summary   `json:"summary"`
	Warmup      measure.Summary   `json:"warmup"`
	Phases      []measure.Summary `json:"phases"`
	Analysis    measure.Analysis  `json:"analysis"`
	Markings    []PhaseMarking    `json:"phase_markings"`
}

// PhaseMarking exposes the local API application boundary and physical phase, never on-wire observations.
type PhaseMarking = engine.PhaseMarking

// localSnapshot extracts bounded physical-phase API evidence alongside local accounting for CLI review.
// The same local quantities appear in saved/recomputed summaries; remote reconciliation remains separate.
func localSnapshot(r engine.Result) *LocalRun {
	s := &LocalRun{Complete: r.Complete, Generation: r.GenerationCutoff, Observation: r.ObservationCutoff, Summary: r.Summary, Warmup: r.Warmup, Phases: r.Phases, Analysis: r.Analysis, Markings: []PhaseMarking{}}
	s.Markings = append(s.Markings, r.Markings...)
	return s
}

// RunResult contains public lifecycle/settings, local accounting and the newly saved report path.
type RunResult struct {
	Version        int             `json:"schema_version"`
	Status         string          `json:"status"`
	Reason         string          `json:"reason,omitempty"`
	Readiness      ReadyResult     `json:"readiness"`
	ServerStatus   string          `json:"server_lifecycle_status"`
	Local          *LocalRun       `json:"local"`
	Evidence       engine.Result   `json:"-"` // Endpoint-only raw observations; separate reconciliation never mutates them.
	ReportPath     string          `json:"local_report_path,omitempty"`
	Remote         *RemoteTransfer `json:"server_evidence_transfer,omitempty"`
	Reconciliation *Reconciliation `json:"reconciliation,omitempty"`
	Clock          *ClockSnapshot  `json:"clock_sampling,omitempty"`
}

// durationSum saturates composed operation budgets instead of wrapping individually valid nanoseconds.
func durationSum(values ...profile.Decimal) time.Duration {
	var total uint64
	for _, v := range values {
		if uint64(v) > math.MaxInt64-total {
			return time.Duration(math.MaxInt64)
		}
		total += uint64(v)
	}
	return time.Duration(total)
}

// drainJoinBudget includes capped drain, worker join, calibration join and the independent disk-writer join.
func drainJoinBudget(p profile.Plan) time.Duration {
	return durationSum(min(p.Grace, p.Limits.ShutdownTimeout), p.Limits.ShutdownTimeout, p.Limits.ShutdownTimeout, p.Limits.ShutdownTimeout)
}

// Run freezes mutable input and reserves this client store before authenticated finite measurement.
// The exclusive owned lock rejects concurrent processes; a crash leaves an explicit stale lock,
// never automatically removed while another process might still be measuring.
func Run(ctx context.Context, store *Store, signer ssh.Signer, p profile.Plan) (result RunResult, err error) {
	result = RunResult{Version: 1, Status: "not_started", ServerStatus: "unknown"}
	defer func() { // Include final setup/run/cleanup failures in public output without exposing session secrets.
		if err != nil {
			result.Reason = err.Error()
			if result.Readiness.MeasurementStarted {
				result.Status = "partial"
			}
		}
	}()
	b, err := json.Marshal(p)
	if err != nil {
		return result, err
	}
	p, err = profile.Decode(bytes.NewReader(b))
	if err != nil {
		return result, err
	}
	if finiteRequestDemand(p) > uint64(p.Limits.ControlRequests) {
		return result, errors.New("finite run exceeds control/calibration request budget")
	}
	lockPath := filepath.Join(store.Root, "active-run.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return result, errors.New("client run reservation unavailable; inspect active-run.lock before removing a stale reservation")
	}
	defer func() { // Remove only the reservation acquired by this run, reporting failed owned cleanup.
		err = errors.Join(err, os.Remove(lockPath))
	}()
	if err = lock.Close(); err != nil {
		return result, err
	}
	ready, err := clientSession(ctx, store, signer, p, 0, &result)
	if result.Readiness.RunID == "" {
		result.Readiness = ready
	}
	return result, err
}

// executeRun binds start and stop to the frozen hash and services bounded heartbeats independently of data.
// Each endpoint starts on processing its own start exchange; the anchors are not synchronized.
func executeRun(ctx context.Context, store *Store, raw net.Conn, exchange func(string, any) (Envelope, error), sshClient *ssh.Client, t adapter.Transport, ca, sa probe.Auth, run, session [16]byte, p profile.Plan, result *RunResult) error {
	persistence, err := newEvidence(store, p, result.Readiness, "client")
	if err != nil {
		return err
	}
	result.ReportPath = persistence.path
	owned := true
	defer func() { // Preserve an incomplete setup record if start/engine construction fails before ownership transfer.
		if owned {
			persistence.Finalize(engine.Result{})
		}
	}()
	hash, _ := p.Hash()
	body := hashBody{result.Readiness.PlanHash}
	raw.SetDeadline(time.Now().Add(time.Duration(p.Limits.WriteTimeout)))
	e, err := exchange("start", body)
	if err != nil {
		return err
	}
	var started startAck
	if e.Type != "started" || DecodeObject(e.Body, &started) != nil || started.Hash != body.Hash || started.Epoch != 1 || started.PeerEpoch != 1 || ca.Plan != hash || sa.Plan != hash {
		return errors.New("invalid frozen-plan start acknowledgment")
	}
	result.Readiness.MeasurementStarted, result.Readiness.Status = true, "started"
	result.Status = "running"
	anchor, err := elapsed.New()
	if err != nil {
		return err
	}
	if err = persistence.setClock(anchor); err != nil {
		return err
	}
	clock := startClock(ctx, sshClient, anchor, p, result.Readiness, persistence)
	defer clock.stop()
	finite, err := engine.NewFiniteWithEvidence(ctx, worker.Config{Registry: store.AdapterRegistry(), Transport: t, SendAuth: ca, ReceiveAuth: sa, Run: run, Session: session, Direction: 1, Epoch: started.PeerEpoch, PeerEpoch: started.Epoch, Clock: anchor, StartNS: 0}, p, false, persistence)
	if err != nil {
		return err
	}
	owned = false
	defer finite.Cancel()
	raw.SetDeadline(time.Time{})
	ticker := time.NewTicker(time.Duration(p.Limits.HeartbeatInterval))
	defer ticker.Stop()
	var controlErr error
loop:
	for {
		select {
		case <-finite.Done():
			break loop
		case <-ctx.Done():
			controlErr = ctx.Err()
			break loop
		case <-ticker.C:
			raw.SetDeadline(time.Now().Add(time.Duration(p.Limits.WriteTimeout)))
			e, controlErr = exchange("heartbeat", body)
			var ack finishAck
			if controlErr == nil && (e.Type != "run-alive" || DecodeObject(e.Body, &ack) != nil || ack.Hash != body.Hash || ack.Status != "running" && ack.Status != "completed" && ack.Status != "partial") {
				controlErr = errors.New("heartbeat plan acknowledgment mismatch")
			}
			if controlErr == nil && ack.Status == "partial" {
				result.ServerStatus = "partial"
				controlErr = errors.New("server reported partial finite endpoint evidence")
			}
			if controlErr != nil {
				break loop
			}
			raw.SetDeadline(time.Time{})
		}
	}
	if controlErr != nil {
		finite.Cancel()
	}
	join, cancel := context.WithTimeout(context.Background(), drainJoinBudget(p))
	local, runErr := finite.Wait(join)
	cancel()
	clock.stop()
	asOf := local.ObservationCutoff
	if len(clock.window.recent) > 0 {
		asOf = max(asOf, clock.window.recent[len(clock.window.recent)-1].Finished)
	}
	result.Clock = clock.window.snapshot(asOf)
	if clock.terminal == "calibration evidence budget/storage unavailable" {
		result.Clock.Reason = clock.terminal
	}
	result.Evidence = local
	result.Local = localSnapshot(local)
	result.Status = "partial"
	if controlErr != nil {
		return controlErr
	}
	// A successful local cutoff is independent of the server's anchor/cutoff and final lifecycle status.
	raw.SetDeadline(time.Now().Add(durationSum(min(p.Grace, p.Limits.ShutdownTimeout), p.Limits.ShutdownTimeout, p.Limits.ShutdownTimeout, p.Limits.ShutdownTimeout, p.Limits.WriteTimeout)))
	e, err = exchange("stop", body)
	if err != nil {
		return err
	}
	var finished finishAck
	if e.Type != "finished" || DecodeObject(e.Body, &finished) != nil || finished.Hash != body.Hash || finished.Status != "completed" && finished.Status != "partial" {
		return errors.New("invalid finite stop acknowledgment")
	}
	result.ServerStatus = finished.Status
	result.Remote, result.Reconciliation, err = receiveServerReport(ctx, raw, sshClient, result.ReportPath, result.Readiness, p, store.AdapterRegistry())
	if err != nil {
		return errors.Join(runErr, err)
	}
	if runErr != nil {
		return runErr
	}
	if !local.Complete || finished.Status != "completed" {
		return errors.New("finite run retained partial endpoint evidence")
	}
	result.Status, result.Readiness.Status = "completed", "finished"
	return nil
}
