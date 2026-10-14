package control

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// Reconciliation derives directional application delivery from compatible endpoint identities.
// Independent cutoffs and unknown packet loss remain; OneWay separately applies explicit clock gates.
type Reconciliation struct {
	Version         int           `json:"schema_version"`
	RunID           string        `json:"run_id"`
	SessionID       string        `json:"session_id"`
	PlanHash        string        `json:"plan_hash"`
	Status          string        `json:"status"`
	Reason          string        `json:"reason"`
	ClientRawSHA256 string        `json:"client_raw_sha256"`
	ServerRawSHA256 string        `json:"server_raw_sha256"`
	Client          *LocalRun     `json:"client_origin"`
	Server          *LocalRun     `json:"server_origin"`
	Warnings        []string      `json:"warnings"`
	OneWay          *OneWayReport `json:"one_way"`
}

// marshalReport preserves exact decimal fields and bounded public JSON without serializing internal ledgers.
func marshalReport(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err == nil && len(b) > FrameMax {
		err = errors.New("report metadata exceeds schema budget")
	}
	return b, err
}

// reconcileReports requires matching frozen identities, settings, epochs and successful endpoint replay.
// Partial/incompatible evidence leaves both directions unknown rather than extrapolating observation windows.
func reconcileReports(client, server *ReplayedReport) *Reconciliation {
	m, s := client.Manifest, server.Manifest
	r := &Reconciliation{Version: 1, RunID: m.RunID, SessionID: m.SessionID, PlanHash: m.PlanHash, Status: "unknown", Reason: "missing, partial or incompatible endpoint evidence", ClientRawSHA256: client.Summary.SHA256, ServerRawSHA256: server.Summary.SHA256, Client: localSnapshot(client.Evidence), Server: localSnapshot(server.Evidence), Warnings: []string{}}
	defer func() { // Derive separately after directional compatibility settles; never alter local RTT/delivery.
		r.OneWay = productionOneWay(client, server, r)
	}()
	a, b := client.Evidence, server.Evidence
	if m.Endpoint != "client" || s.Endpoint != "server" || m.RunID != s.RunID || m.SessionID != s.SessionID || m.PlanHash != s.PlanHash || !reflect.DeepEqual(m.Plan, s.Plan) || !reflect.DeepEqual(m.Settings, s.Settings) || a.Run != b.Run || a.Session != b.Session || a.PlanHash != b.PlanHash || a.Direction != 1 || b.Direction != 2 || a.Epoch != b.PeerEpoch || a.PeerEpoch != b.Epoch || a.Epoch != 1 {
		r.Reason = "endpoint identity, plan, settings or epoch mismatch"
		return r
	}
	if client.Status != "completed" || server.Status != "completed" || client.Verification != "matched" || server.Verification != "matched" || !a.Complete || !b.Complete || !a.Receiver.Complete || !b.Receiver.Complete {
		return r
	}
	r.Client = reconcileOrigin(a, b, &r.Warnings, "client")
	r.Server = reconcileOrigin(b, a, &r.Warnings, "server")
	if r.Client.Summary.Received == nil || r.Server.Summary.Received == nil {
		r.Reason = "unmatched receiver identities; directional delivery remains unknown where incompatible"
		return r
	}
	r.Status, r.Reason = "reconciled", "complete compatible endpoint records at independent local cutoffs; missing application data does not establish network packet loss; no cross-host timing subtraction"
	return r
}

// reconcileOrigin applies the shared ledger to authenticated receiver arrivals and preserves write ambiguity.
// Receiver identities absent from attempted origin work invalidate that direction instead of inflating delivery.
func reconcileOrigin(origin engine.Result, peer engine.Result, warnings *[]string, role string) *LocalRun {
	receiver := peer.Receiver
	out := localSnapshot(origin)
	out.Phases = append([]measure.Summary(nil), origin.Phases...)
	entries := make(map[measure.Identity]measure.Entry, len(origin.Entries))
	for _, e := range origin.Entries {
		entries[e.Identity] = e
	}
	arrivals := make(map[measure.Identity]engine.Record, len(receiver.Arrivals))
	for _, raw := range peer.Records {
		var record engine.Record
		if json.Unmarshal(raw, &record) == nil && record.Kind == "arrival" && record.FrameKind == probe.Data && record.At < peer.ObservationCutoff {
			arrivals[record.Identity] = record
		}
	}
	for id, at := range receiver.Arrivals {
		// A complete read racing endpoint stop remains raw evidence, but cannot
		// invalidate or join the receiver's half-open eligible population.
		// The origin-receipt check below still rejects contradictory claims.
		if at >= receiver.Cutoff {
			continue
		}
		e, ok := entries[id]
		arrival, recorded := arrivals[id]
		if !ok || e.Attempt == nil || uint64(*e.Attempt) >= uint64(origin.ObservationCutoff) || !recorded || arrival.At != profile.Decimal(at) || arrival.Attempt != *e.Attempt || arrival.Epoch != origin.Epoch || arrival.Direction != origin.Direction {
			*warnings = append(*warnings, role+": receiver contains unmatched or outside-cutoff identity")
			return out
		}
	}
	ambiguous := false
	for _, e := range origin.Entries {
		if e.Receipt != nil && *e.Receipt < origin.ObservationCutoff {
			if at, exists := receiver.Arrivals[e.Identity]; !exists || at >= receiver.Cutoff {
				*warnings = append(*warnings, role+": origin receipt contradicts complete receiver ledger")
				return out
			}
		}
		if e.Attempt != nil && (e.State != "written" || e.WriteCompleted == nil || *e.WriteCompleted >= origin.ObservationCutoff) {
			ambiguous = true
		}
	}
	if ambiguous {
		*warnings = append(*warnings, role+": failed/pending writes are excluded from missing-data claims; observed arrivals remain distinct")
	}
	out.Summary = measure.SummarizeSnapshot(origin.Entries, uint64(origin.ObservationCutoff), &receiver, 2, nil)
	out.Warmup = measure.SummarizeSnapshot(origin.Entries, uint64(origin.ObservationCutoff), &receiver, 1, nil)
	groups := make([][]measure.Entry, len(out.Phases))
	for _, e := range origin.Entries {
		if e.Identity.Lifecycle == 2 && uint64(e.Identity.Phase) < uint64(len(groups)) {
			groups[e.Identity.Phase] = append(groups[e.Identity.Phase], e)
		}
	}
	for i := range out.Phases {
		out.Phases[i] = measure.SummarizeSnapshot(groups[i], uint64(origin.ObservationCutoff), &receiver, 2, nil)
	}
	return out
}

// replayTransferred rechecks a saved transfer receipt and its fixed files before deriving delivery offline.
// Cached reconciliation JSON is never trusted; absent/incomplete receipts retain unknown delivery.
func replayTransferred(dir string, local *ReplayedReport, remaining *uint64, registries ...*adapter.Registry) error {
	path := filepath.Join(dir, "server-transfer.json")
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	}
	b, err := reportFile(dir, "server-transfer.json", remaining)
	if err != nil {
		return err
	}
	if len(b) > FrameMax {
		return errors.New("transfer receipt exceeds schema budget")
	}
	var t RemoteTransfer
	if err = decodeReportObject(b, &t); err != nil {
		return err
	}
	m := local.Manifest
	if t.Version != 1 || t.RunID != m.RunID || t.SessionID != m.SessionID || t.PlanHash != m.PlanHash || t.Status != "verified" && t.Status != "incomplete" {
		return errors.New("transfer receipt identity/status mismatch")
	}
	local.TransferReceipt = &t
	if t.Status != "verified" {
		return nil
	}
	ready := ReadyResult{RunID: m.RunID, SessionID: m.SessionID, PlanHash: m.PlanHash}
	if err = validateReportOffer(t.Offer, ready, m.Plan); err != nil {
		return err
	}
	remoteDir := filepath.Join(dir, "remote-server")
	// Hash verification and replay read the same files twice; both passes consume the aggregate inspection budget.
	before := *remaining
	if err = verifyTransferredFiles(remoteDir, t.Offer, before); err != nil {
		return err
	}
	*remaining -= uint64(t.Offer.Bytes)
	remote, err := replayReport(remoteDir, remaining, false, registries...)
	if err != nil {
		return err
	}
	local.Reconciliation = reconcileReports(local, remote)
	local.ServerReport = remote
	return nil
}
