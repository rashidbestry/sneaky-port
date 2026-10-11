package control

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// reportFixture gives streaming tests multiple chunks and file boundaries, independently of report schema tests.
func reportFixture(t *testing.T) (string, ReadyResult, profile.Plan, reportOffer) {
	t.Helper()
	p := testutil.Plan()
	h, _ := p.Hash()
	ready := ReadyResult{RunID: strings.Repeat("ab", 16), SessionID: strings.Repeat("cd", 16), PlanHash: digestText(nil)}
	ready.PlanHash = hex.EncodeToString(h[:])
	dir := t.TempDir()
	for i, name := range reportFiles {
		b := []byte("{}")
		if i == 1 {
			b = bytes.Repeat([]byte("exact decimal 9007199254740993\n"), 3000)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	o, files, err := openReportSnapshot(dir, p, ready.PlanHash)
	if err != nil {
		t.Fatal(err)
	}
	closeReportFiles(files)
	return dir, ready, p, o
}

// TestReportStreaming exercises real full-duplex framing, contiguous chunks, acknowledgments and fixed file preservation.
func TestReportStreaming(t *testing.T) {
	dir, ready, p, offer := reportFixture(t)
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	left.SetDeadline(time.Now().Add(3 * time.Second))
	right.SetDeadline(time.Now().Add(3 * time.Second))
	done := make(chan error, 1)
	go func() { // The sender advances only after client acknowledgment; deadlines bound fixture failure.
		done <- serveReport(left, dir, ready, p, p.Limits.ControlRequests)
	}()
	destination := t.TempDir()
	receipt := &RemoteTransfer{}
	if err := receiveReport(right, destination, ready, p, receipt); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if receipt.Chunks < 3 || !reflect.DeepEqual(receipt.Offer, offer) {
		t.Fatal("multi-chunk offer mismatch")
	}
	for _, name := range reportFiles {
		a, _ := os.ReadFile(filepath.Join(dir, name))
		b, _ := os.ReadFile(filepath.Join(destination, name))
		if !bytes.Equal(a, b) {
			t.Fatal("transferred bytes changed")
		}
	}
	if err := receiveReport(&scriptedReport{Reader: bytes.NewReader(nil)}, destination, ready, p, &RemoteTransfer{}); err == nil {
		t.Fatal("truncated transfer accepted")
	}
}

// scriptedReport supplies adversarial incoming frames while retaining emitted acknowledgments for inspection.
type scriptedReport struct {
	io.Reader
	bytes.Buffer
}

// Read disambiguates the incoming script from captured outgoing acknowledgments.
func (s *scriptedReport) Read(b []byte) (int, error) { return s.Reader.Read(b) }

// TestReportTransferRejections covers malicious offsets/numbers/identities/checksums, truncation and budgets.
func TestReportTransferRejections(t *testing.T) {
	dir, ready, p, offer := reportFixture(t)
	var content []byte
	for _, name := range reportFiles {
		b, _ := os.ReadFile(filepath.Join(dir, name))
		content = append(content, b...)
	}
	for _, fault := range []string{"offset", "number", "identity", "digest", "plan", "short", "truncated", "final", "file_hash", "name", "budget", "requests", "repeated", "ack"} {
		t.Run(fault, func(t *testing.T) { // Each fault must prevent successful receipt verification and bounded advancement.
			var input bytes.Buffer
			o := offer
			o.Files = append([]reportPart(nil), offer.Files...)
			plan := p
			if fault == "name" {
				o.Files[0].Name = "../manifest.json"
			}
			if fault == "budget" {
				plan.Limits.FileBytes = 1
			}
			if fault == "requests" {
				plan.Limits.ControlRequests = 1
			}
			if fault == "file_hash" {
				o.Files[0].SHA256 = digestText([]byte("wrong"))
			}
			Write(&input, message("report-offer", "report", ready.RunID, ready.SessionID, o))
			chunks := (len(content) + reportChunkBytes - 1) / reportChunkBytes
			for n, offset := 1, 0; offset < len(content); n++ {
				length := min(reportChunkBytes, len(content)-offset)
				data := bytes.Clone(content[offset : offset+length])
				c := reportChunk{ready.PlanHash, n, profile.Decimal(offset), data, digestText(data)}
				run := ready.RunID
				if n == 1 {
					switch fault {
					case "offset":
						c.Offset++
					case "number":
						c.Number++
					case "identity":
						run = strings.Repeat("ef", 16)
					case "digest":
						c.SHA256 = digestText(nil)
					case "plan":
						c.Hash = digestText(nil)
					case "short":
						c.Data = c.Data[:len(c.Data)-1]
					}
				}
				id := "chunk-" + decimalInt(n)
				if fault == "repeated" && n == 2 {
					c.Number, c.Offset, id = 1, 0, "chunk-1"
				}
				Write(&input, message("report-chunk", id, run, ready.SessionID, c))
				offset += length
				if fault == "truncated" {
					break
				}
			}
			end := reportAck{ready.PlanHash, chunks, offer.Bytes, offer.SHA256}
			if fault == "final" {
				end.SHA256 = digestText(nil)
			}
			Write(&input, message("report-end", "report-end", ready.RunID, ready.SessionID, end))
			if fault == "ack" {
				input.Reset()
				Write(&input, message("report", "report", ready.RunID, ready.SessionID, hashBody{ready.PlanHash}))
				Write(&input, message("report-ack", "report", ready.RunID, ready.SessionID, reportAck{ready.PlanHash, 0, 1, offer.SHA256}))
				if err := serveReport(&scriptedReport{Reader: &input}, dir, ready, p, p.Limits.ControlRequests); err == nil {
					t.Fatal("invalid acknowledgment accepted")
				}
				return
			}
			if err := receiveReport(&scriptedReport{Reader: &input}, t.TempDir(), ready, plan, &RemoteTransfer{}); err == nil {
				t.Fatal("invalid transfer accepted")
			}
		})
	}
}

// decimalInt preserves short fixture request numbers in deterministic text.
func decimalInt(v int) string { b, _ := json.Marshal(v); return string(b) }

// reconcileFixture creates matched endpoint snapshots with one delivered datum lacking its receipt and one missing datum.
// The host-local cutoffs intentionally differ; no timing difference between hosts is interpreted.
func reconcileFixture() (*ReplayedReport, *ReplayedReport) {
	p := testutil.Plan()
	h, _ := p.Hash()
	at, admitted, completed, receipt := profile.Decimal(10), profile.Decimal(5), profile.Decimal(20), profile.Decimal(30)
	client := engine.Result{PlanHash: h, Direction: 1, Epoch: 1, PeerEpoch: 1, Complete: true, ObservationCutoff: 100, Phases: []measure.Summary{{}}, Receiver: measure.ReceiverEvidence{Complete: true, Cutoff: 100, Arrivals: map[measure.Identity]uint64{}}}
	for i := 0; i < 3; i++ {
		e := measure.Entry{Identity: measure.Identity{Lifecycle: 2, Sequence: profile.Decimal(i)}, Scheduled: 0, Deadline: 40, State: "written", Admission: "accepted", Admitted: &admitted, Attempt: &at, WriteCompleted: &completed}
		if i == 0 {
			e.Receipt = &receipt
		}
		client.Entries = append(client.Entries, e)
	}
	client.Summary = measure.SummarizeSnapshot(client.Entries, 100, nil, 2, nil)
	server := engine.Result{PlanHash: h, Direction: 2, Epoch: 1, PeerEpoch: 1, Complete: true, ObservationCutoff: 90, Phases: []measure.Summary{{}}, Receiver: measure.ReceiverEvidence{Complete: true, Cutoff: 90, Arrivals: map[measure.Identity]uint64{}}}
	for i := 0; i < 2; i++ {
		id := client.Entries[i].Identity
		server.Receiver.Arrivals[id] = uint64(50 + i)
		b, _ := json.Marshal(engine.Record{Kind: "arrival", FrameKind: probe.Data, Identity: id, Direction: 1, Epoch: 1, Attempt: at, At: profile.Decimal(50 + i)})
		server.Records = append(server.Records, b)
	}
	makeReport := func(endpoint string, r engine.Result) *ReplayedReport { // Fixtures hold already validated snapshots; raw replay is exercised separately with authentic runs.
		return &ReplayedReport{Status: "completed", Verification: "matched", Manifest: EvidenceManifest{Endpoint: endpoint, RunID: strings.Repeat("ab", 16), SessionID: strings.Repeat("cd", 16), PlanHash: hex.EncodeToString(h[:]), Plan: p}, Evidence: r}
	}
	return makeReport("client", client), makeReport("server", server)
}

// TestReconciledDelivery distinguishes lost receipts, missing application data and incompatible observations exactly.
func TestReconciledDelivery(t *testing.T) {
	a, b := reconcileFixture()
	r := reconcileReports(a, b)
	if r.Status != "reconciled" || *r.Client.Summary.Received != 2 || *r.Client.Summary.MissingAtReceiver != 1 || r.Client.Summary.Unresolved != 2 || r.Client.Summary.TransportPacketLoss != nil || a.Evidence.Summary.Received != nil || a.Evidence.Phases[0].Received != nil {
		t.Fatalf("delivery/receipt accounting or snapshot mutation: %+v", r)
	}
	for _, fault := range []string{"partial", "run", "plan", "epoch", "unmatched", "attempt", "cutoff", "contradiction", "ambiguous"} {
		t.Run(fault, func(t *testing.T) { // Unknown evidence must remain nullable instead of becoming zero missing data.
			a, b := reconcileFixture()
			switch fault {
			case "partial":
				b.Status = "partial"
			case "run":
				b.Manifest.RunID = "different"
			case "plan":
				b.Manifest.PlanHash = "different"
			case "epoch":
				b.Evidence.PeerEpoch = 2
			case "unmatched":
				b.Evidence.Receiver.Arrivals[measure.Identity{Lifecycle: 2, Sequence: 99}] = 40
			case "attempt":
				var record engine.Record
				json.Unmarshal(b.Evidence.Records[0], &record)
				record.Attempt++
				b.Evidence.Records[0], _ = json.Marshal(record)
			case "cutoff":
				b.Evidence.Receiver.Cutoff = 50
			case "contradiction":
				delete(b.Evidence.Receiver.Arrivals, a.Evidence.Entries[0].Identity)
			case "ambiguous":
				a.Evidence.Entries[2].State = "write_failed"
			}
			r := reconcileReports(a, b)
			if fault == "ambiguous" {
				if r.Status != "reconciled" || *r.Client.Summary.MissingAtReceiver != 0 || len(r.Warnings) == 0 {
					t.Fatal("ambiguous write classified as missing")
				}
				return
			}
			if r.Status != "unknown" || r.Client.Summary.Received != nil || r.Client.Summary.MissingAtReceiver != nil {
				t.Fatal("incompatible evidence became known")
			}
		})
	}
}

// TestAuthenticatedReportReplay binds persisted remote files to a real pinned SSH run and rederives exports offline.
func TestAuthenticatedReportReplay(t *testing.T) {
	r := finiteRig(t, "udp")
	result, err := Run(context.Background(), r.client, r.signer, r.plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Remote == nil || result.Remote.Status != "verified" || result.Reconciliation == nil || result.Reconciliation.Status != "reconciled" {
		t.Fatalf("missing authenticated reconciliation: %+v", result.Reconciliation)
	}
	replayed, err := ReplayReport(result.ReportPath, 8<<20)
	if err != nil || !reflect.DeepEqual(replayed.Reconciliation, result.Reconciliation) || replayed.ServerReport == nil {
		t.Fatalf("offline reconciliation mismatch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(result.ReportPath, "reconciliation.json"), []byte("untrusted cached summary"), 0600); err != nil {
		t.Fatal(err)
	}
	replayed, err = ReplayReport(result.ReportPath, 8<<20)
	if err != nil || !reflect.DeepEqual(replayed.Reconciliation, result.Reconciliation) {
		t.Fatal("cached reconciliation trusted")
	}
	for _, format := range []string{"json", "jsonl", "csv"} {
		var output bytes.Buffer
		if err := ExportReport(&output, replayed, format); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(output.Bytes(), []byte("reconcil")) || format != "json" && !bytes.Contains(output.Bytes(), []byte("server_raw_event")) {
			t.Fatal("reconciled export evidence absent")
		}
	}
	if _, err = ReplayReport(result.ReportPath, 1); err == nil {
		t.Fatal("aggregate replay cap ignored")
	}
	path := filepath.Join(result.ReportPath, "remote-server", "events.jsonl")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("tampered"))
	f.Close()
	if _, err = ReplayReport(result.ReportPath, 8<<20); err == nil {
		t.Fatal("tampered remote file accepted")
	}
}

// TestReportBudgetFailure retains complete local data and an incomplete transfer receipt when final evidence cannot fit remaining requests.
func TestReportBudgetFailure(t *testing.T) {
	r := finiteRig(t, "udp")
	r.plan.Limits.HeartbeatInterval = 100e6
	r.plan.Limits.ControlRequests = 10
	result, err := Run(context.Background(), r.client, r.signer, r.plan)
	if err == nil || result.Status != "partial" || result.Local == nil || !result.Local.Complete || result.Remote == nil || result.Remote.Status != "incomplete" || result.Reconciliation != nil || result.Local.Summary.Received != nil {
		t.Fatalf("transfer failure invented delivery or lost local report: %v", err)
	}
	replayed, err := ReplayReport(result.ReportPath, 4<<20)
	if err != nil || replayed.Status != "completed" || replayed.Reconciliation != nil {
		t.Fatalf("local evidence lost after transfer failure: %v", err)
	}
	assertReadyAfterDrain(t, r)
}

// TestReportTransferInterruption bounds a stalled chunk exchange without accepting a final hash or replacing existing files.
func TestReportTransferInterruption(t *testing.T) {
	dir, ready, p, _ := reportFixture(t)
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	left.SetDeadline(time.Now().Add(100 * time.Millisecond))
	right.SetDeadline(time.Now().Add(100 * time.Millisecond))
	done := make(chan error, 1)
	go func() { // Withheld initial acknowledgment must end sender ownership at its explicit fixture deadline.
		done <- serveReport(left, dir, ready, p, p.Limits.ControlRequests)
	}()
	if err := Write(right, message("report", "report", ready.RunID, ready.SessionID, hashBody{ready.PlanHash})); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(right); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("stalled receiver accepted")
	}
}
