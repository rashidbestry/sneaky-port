package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/profile"
)

const ReportChannelType = "sneaky-port/report/v1"
const reportChunkBytes = 32768

// reportFiles is the only permitted transfer order; peer-supplied paths never reach filesystem operations.
var reportFiles = []string{"manifest.json", "events.jsonl", "summary.json"}

// reportPart binds a fixed file's size and content to the authenticated offer.
type reportPart struct {
	Name   string          `json:"name"`
	Bytes  profile.Decimal `json:"bytes"`
	SHA256 string          `json:"sha256"`
}

// reportOffer supplies a bounded aggregate digest over the three fixed files in canonical order.
type reportOffer struct {
	Hash   string          `json:"plan_hash"`
	Files  []reportPart    `json:"files"`
	Bytes  profile.Decimal `json:"bytes"`
	SHA256 string          `json:"sha256"`
}

// reportChunk carries a numbered contiguous segment; JSON encodes Data as base64, below FrameMax.
type reportChunk struct {
	Hash   string          `json:"plan_hash"`
	Number int             `json:"number"`
	Offset profile.Decimal `json:"offset"`
	Data   []byte          `json:"data_base64"`
	SHA256 string          `json:"sha256"`
}

// reportAck confirms the exact next aggregate offset, including final content verification.
type reportAck struct {
	Hash   string          `json:"plan_hash"`
	Number int             `json:"number"`
	Offset profile.Decimal `json:"next_offset"`
	SHA256 string          `json:"sha256"`
}

// RemoteTransfer records transport provenance separately from endpoint observation completeness.
// A saved receipt is local provenance, not a cryptographic attestation for later arbitrary imports.
type RemoteTransfer struct {
	Version   int             `json:"schema_version"`
	RunID     string          `json:"run_id"`
	SessionID string          `json:"session_id"`
	PlanHash  string          `json:"plan_hash"`
	Status    string          `json:"status"`
	Reason    string          `json:"reason"`
	Source    string          `json:"source"`
	Offer     reportOffer     `json:"offer"`
	Chunks    int             `json:"chunks"`
	Sent      profile.Decimal `json:"sent_application_bytes"`
	Received  profile.Decimal `json:"received_application_bytes"`
}

// digestText returns canonical SHA-256 text without passing exact bytes through numeric JSON values.
func digestText(b []byte) string {
	d := sha256.Sum256(b)
	return hex.EncodeToString(d[:])
}

// transferLimit shares the explicit per-endpoint file budget and offline schema ceiling.
func transferLimit(p profile.Plan) uint64 { return min(uint64(p.Limits.FileBytes), uint64(1<<30)) }

// openReportSnapshot pins regular files, bounds total bytes and hashes with constant-size read buffers.
// The send pass rehashes content; mutation between passes cannot produce a successful final acknowledgment.
func openReportSnapshot(dir string, p profile.Plan, hash string) (reportOffer, []*os.File, error) {
	o := reportOffer{Hash: hash, Files: []reportPart{}}
	files := []*os.File{}
	all := sha256.New()
	for _, name := range reportFiles {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || uint64(info.Size()) > transferLimit(p)-uint64(o.Bytes) {
			closeReportFiles(files)
			return o, nil, errors.New("server report unavailable or exceeds file budget")
		}
		f, err := os.Open(path)
		if err != nil {
			closeReportFiles(files)
			return o, nil, err
		}
		files = append(files, f)
		opened, err := f.Stat()
		if err != nil || !os.SameFile(info, opened) {
			closeReportFiles(files)
			return o, nil, errors.New("server report changed while opening")
		}
		d := sha256.New()
		n, err := io.Copy(io.MultiWriter(d, all), io.LimitReader(f, info.Size()+1))
		if err != nil || n != info.Size() {
			closeReportFiles(files)
			return o, nil, errors.New("server report size changed")
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			closeReportFiles(files)
			return o, nil, err
		}
		o.Files = append(o.Files, reportPart{name, profile.Decimal(n), hex.EncodeToString(d.Sum(nil))})
		o.Bytes += profile.Decimal(n)
	}
	o.SHA256 = hex.EncodeToString(all.Sum(nil))
	return o, files, nil
}

// closeReportFiles releases only handles opened by this report operation.
func closeReportFiles(files []*os.File) {
	for _, f := range files {
		f.Close()
	}
}

// routeReportChannels refuses premature/extra/shell channels promptly and admits one report only after stop.
// A finite flood budget prevents unbounded channel work while the control lifecycle owns the connection.
func routeReportChannels(ctx context.Context, incoming <-chan ssh.NewChannel, reports chan<- ssh.NewChannel, ready <-chan struct{}, max int, cancel context.CancelFunc) {
	defer close(reports)
	admitted := false
	for n := 0; n < max; n++ {
		select {
		case <-ctx.Done():
			return
		case ch, ok := <-incoming:
			if !ok {
				return
			}
			allowed := false
			select {
			case <-ready:
				allowed = true
			default:
			}
			if !allowed || admitted || ch.ChannelType() != ReportChannelType || len(ch.ExtraData()) != 0 {
				ch.Reject(ssh.Prohibited, "one post-stop report channel only")
				continue
			}
			admitted = true
			select {
			case reports <- ch:
			case <-ctx.Done():
				ch.Reject(ssh.Prohibited, "control ended")
				return
			}
		}
	}
	cancel()
}

// serveReportChannel accepts one post-stop application channel on the already authenticated connection.
// SetupTimeout bounds the entire transfer; authorization revocation/SSH loss still cancels its ownership.
func serveReportChannel(ctx context.Context, raw net.Conn, channels <-chan ssh.NewChannel, path string, ready ReadyResult, p profile.Plan, requests int) {
	deadline := time.Now().Add(time.Duration(p.Limits.SetupTimeout))
	raw.SetDeadline(deadline)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		return
	case incoming, ok := <-channels:
		if !ok {
			return
		}
		if incoming.ChannelType() != ReportChannelType || len(incoming.ExtraData()) != 0 {
			incoming.Reject(ssh.Prohibited, "post-stop report channel required")
			return
		}
		ch, reqs, err := incoming.Accept()
		if err != nil {
			return
		}
		defer ch.Close()
		child, cancel := context.WithCancel(ctx)
		defer cancel()
		go rejectRequests(child, reqs, p.Limits.ControlRequests, cancel)
		stop := context.AfterFunc(child, func() { // Interrupt channel reads on server shutdown or prohibited request floods.
			ch.Close()
		})
		defer stop()
		serveReport(ch, path, ready, p, requests)
	}
}

// reportEnvelope requires exact run/session/request identities for every report frame.
func reportEnvelope(e Envelope, ready ReadyResult, kind, id string) bool {
	return e.Version == 1 && e.Type == kind && e.RequestID == id && e.RunID == ready.RunID && e.SessionID == ready.SessionID
}

// serveReport streams fixed files with acknowledged offsets, chunk checksums and a final content hash.
// Requests are bounded by the remaining frozen control-request budget; no reconnect/resume is attempted.
func serveReport(ch io.ReadWriter, path string, ready ReadyResult, p profile.Plan, requests int) error {
	e, err := Read(ch)
	var body hashBody
	if err != nil || !reportEnvelope(e, ready, "report", "report") || DecodeObject(e.Body, &body) != nil || body.Hash != ready.PlanHash {
		return errors.New("report request identity mismatch")
	}
	o, files, err := openReportSnapshot(path, p, ready.PlanHash)
	if err != nil {
		Write(ch, message("error", "report", ready.RunID, ready.SessionID, errorBody{err.Error()}))
		return err
	}
	defer closeReportFiles(files)
	chunks := int((uint64(o.Bytes) + reportChunkBytes - 1) / reportChunkBytes)
	if chunks+3 > requests {
		return errors.New("report exceeds remaining control request budget")
	}
	if err = Write(ch, message("report-offer", "report", ready.RunID, ready.SessionID, o)); err != nil {
		return err
	}
	ack := func(id string, expected reportAck) error { // Refuse skipped, repeated or mismatched acknowledgments before advancing the reader.
		e, err := Read(ch)
		var got reportAck
		if err != nil {
			return err
		}
		if !reportEnvelope(e, ready, "report-ack", id) || DecodeObject(e.Body, &got) != nil || got != expected {
			return errors.New("report acknowledgment mismatch")
		}
		return nil
	}
	if err = ack("report", reportAck{ready.PlanHash, 0, 0, o.SHA256}); err != nil {
		return err
	}
	readers := make([]io.Reader, len(files))
	for i, f := range files {
		readers[i] = f
	}
	r := io.MultiReader(readers...)
	all := sha256.New()
	var offset profile.Decimal
	for n := 1; n <= chunks; n++ {
		b := make([]byte, min(uint64(reportChunkBytes), uint64(o.Bytes-offset)))
		if _, err = io.ReadFull(r, b); err != nil {
			return err
		}
		all.Write(b)
		id := fmt.Sprintf("chunk-%d", n)
		if err = Write(ch, message("report-chunk", id, ready.RunID, ready.SessionID, reportChunk{ready.PlanHash, n, offset, b, digestText(b)})); err != nil {
			return err
		}
		offset += profile.Decimal(len(b))
		if err = ack(id, reportAck{ready.PlanHash, n, offset, digestText(b)}); err != nil {
			return err
		}
	}
	var extra [1]byte
	if n, err := r.Read(extra[:]); n != 0 || err != io.EOF || hex.EncodeToString(all.Sum(nil)) != o.SHA256 {
		return errors.New("report changed during transfer")
	}
	end := reportAck{ready.PlanHash, chunks, offset, o.SHA256}
	if err = Write(ch, message("report-end", "report-end", ready.RunID, ready.SessionID, end)); err != nil {
		return err
	}
	return ack("report-end", end)
}

// validateReportOffer rejects path substitutions, overflow, oversized files and unsupported digest encodings.
func validateReportOffer(o reportOffer, ready ReadyResult, p profile.Plan) error {
	if o.Hash != ready.PlanHash || len(o.Files) != len(reportFiles) || o.Bytes == 0 || uint64(o.Bytes) > transferLimit(p) {
		return errors.New("report offer plan/files/budget mismatch")
	}
	if _, err := parse32(o.SHA256); err != nil {
		return err
	}
	var total uint64
	for i, f := range o.Files {
		if f.Name != reportFiles[i] || uint64(f.Bytes) > uint64(o.Bytes)-total || i != 1 && uint64(f.Bytes) > profile.DocumentMax {
			return errors.New("report file size/name mismatch")
		}
		if _, err := parse32(f.SHA256); err != nil {
			return err
		}
		total += uint64(f.Bytes)
	}
	if total != uint64(o.Bytes) || (total+reportChunkBytes-1)/reportChunkBytes+3 > uint64(p.Limits.ControlRequests) {
		return errors.New("report aggregate/request budget mismatch")
	}
	return nil
}

// receiveServerReport uses a separate channel after finite stop, preserving endpoint-only local files.
// Transfer failure yields an incomplete receipt; it never manufactures remote arrivals or retries.
func receiveServerReport(ctx context.Context, raw net.Conn, client *ssh.Client, dir string, ready ReadyResult, p profile.Plan) (*RemoteTransfer, *Reconciliation, error) {
	t := &RemoteTransfer{Version: 1, RunID: ready.RunID, SessionID: ready.SessionID, PlanHash: ready.PlanHash, Status: "incomplete", Reason: "transfer not finalized", Source: "same pinned-host, enrolled-client SSH connection; local receipt is not a transferable attestation"}
	deadline := time.Now().Add(time.Duration(p.Limits.SetupTimeout))
	raw.SetDeadline(deadline)
	ch, reqs, err := client.OpenChannel(ReportChannelType, nil)
	if err != nil {
		t.Reason = err.Error()
		return t, nil, err
	}
	defer ch.Close()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	go rejectRequests(child, reqs, p.Limits.ControlRequests, cancel)
	stop := context.AfterFunc(child, func() { // Join transfer cancellation to the owned report channel without touching saved local evidence.
		ch.Close()
	})
	defer stop()
	counted := &frameCounter{Channel: ch}
	remoteDir := filepath.Join(dir, "remote-server")
	if err = os.Mkdir(remoteDir, 0700); err != nil {
		return t, nil, err
	}
	if err = restrictDirectory(remoteDir); err != nil {
		return t, nil, err
	}
	err = receiveReport(counted, remoteDir, ready, p, t)
	t.Sent, t.Received = counted.sent, counted.received
	if err != nil {
		t.Reason = err.Error()
	}
	var reconciliation *Reconciliation
	if err == nil {
		local, e := ReplayReport(dir, transferLimit(p))
		if e == nil {
			var remote *ReplayedReport
			remote, e = ReplayReport(remoteDir, transferLimit(p))
			if e == nil {
				if remote.Manifest.RunID != ready.RunID || remote.Manifest.SessionID != ready.SessionID || remote.Manifest.PlanHash != ready.PlanHash || remote.Manifest.Endpoint != "server" {
					e = errors.New("server report content identity mismatch")
				} else {
					reconciliation = reconcileReports(local, remote)
				}
			}
		}
		err = e
		if err == nil {
			t.Status, t.Reason = "verified", "fixed files, chunks, final hash and endpoint replay verified"
		} else {
			t.Reason = err.Error()
		}
	}
	b, marshalErr := marshalReport(t)
	if marshalErr == nil {
		marshalErr = publishEvidence(dir, "server-transfer.json", b, false)
	}
	err = errors.Join(err, marshalErr)
	if err == nil && reconciliation != nil {
		b, err = marshalReport(reconciliation)
		if err == nil {
			err = publishEvidence(dir, "reconciliation.json", b, false)
		}
	}
	return t, reconciliation, err
}

// receiveReport writes bounded contiguous chunks directly to exclusive fixed files and verifies every digest.
// Files left by interruption stay unusable without a verified transfer receipt; none are overwritten.
func receiveReport(ch io.ReadWriter, dir string, ready ReadyResult, p profile.Plan, t *RemoteTransfer) error {
	if err := Write(ch, message("report", "report", ready.RunID, ready.SessionID, hashBody{ready.PlanHash})); err != nil {
		return err
	}
	e, err := Read(ch)
	if err != nil {
		return err
	}
	if !reportEnvelope(e, ready, "report-offer", "report") || DecodeObject(e.Body, &t.Offer) != nil {
		return errors.New("invalid server report offer")
	}
	if err = validateReportOffer(t.Offer, ready, p); err != nil {
		return err
	}
	files := []*os.File{}
	for _, name := range reportFiles {
		f, err := createPrivate(filepath.Join(dir, name))
		if err != nil {
			closeReportFiles(files)
			return err
		}
		files = append(files, f)
	}
	defer closeReportFiles(files)
	ack := func(id string, a reportAck) error { // Acknowledge only already validated content at the exact next offset.
		return Write(ch, message("report-ack", id, ready.RunID, ready.SessionID, a))
	}
	if err = ack("report", reportAck{ready.PlanHash, 0, 0, t.Offer.SHA256}); err != nil {
		return err
	}
	all := sha256.New()
	var offset profile.Decimal
	fileIndex := 0
	var fileOffset uint64
	chunks := int((uint64(t.Offer.Bytes) + reportChunkBytes - 1) / reportChunkBytes)
	for n := 1; n <= chunks; n++ {
		e, err = Read(ch)
		if err != nil {
			return err
		}
		var c reportChunk
		id := fmt.Sprintf("chunk-%d", n)
		if !reportEnvelope(e, ready, "report-chunk", id) || DecodeObject(e.Body, &c) != nil || c.Hash != ready.PlanHash || c.Number != n || c.Offset != offset || len(c.Data) != int(min(uint64(reportChunkBytes), uint64(t.Offer.Bytes-offset))) || digestText(c.Data) != c.SHA256 {
			return errors.New("report chunk identity/offset/checksum mismatch")
		}
		all.Write(c.Data)
		b := c.Data
		for len(b) > 0 {
			for fileIndex < len(files) && fileOffset == uint64(t.Offer.Files[fileIndex].Bytes) {
				fileIndex++
				fileOffset = 0
			}
			if fileIndex >= len(files) {
				return errors.New("report file boundary overflow")
			}
			length := min(uint64(len(b)), uint64(t.Offer.Files[fileIndex].Bytes)-fileOffset)
			written, err := files[fileIndex].Write(b[:length])
			if err != nil {
				return err
			}
			if written != int(length) {
				return io.ErrShortWrite
			}
			fileOffset += length
			b = b[length:]
		}
		offset += profile.Decimal(len(c.Data))
		t.Chunks = n
		if err = ack(id, reportAck{ready.PlanHash, n, offset, c.SHA256}); err != nil {
			return err
		}
	}
	e, err = Read(ch)
	if err != nil {
		return err
	}
	var end reportAck
	if !reportEnvelope(e, ready, "report-end", "report-end") || DecodeObject(e.Body, &end) != nil || end != (reportAck{ready.PlanHash, chunks, offset, t.Offer.SHA256}) || hex.EncodeToString(all.Sum(nil)) != t.Offer.SHA256 {
		return errors.New("report final hash/offset mismatch")
	}
	for _, f := range files {
		if err = f.Sync(); err != nil {
			return err
		}
	}
	closeReportFiles(files)
	if err = verifyTransferredFiles(dir, t.Offer, transferLimit(p)); err != nil {
		return err
	}
	return ack("report-end", end)
}

// verifyTransferredFiles checks the original ordered offer against saved files; SHA-256 alone is local consistency.
func verifyTransferredFiles(dir string, o reportOffer, limit uint64) error {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("transferred report must be a real directory")
	}
	all := sha256.New()
	for _, part := range o.Files {
		b, err := reportFile(dir, part.Name, &limit)
		if err != nil {
			return err
		}
		if profile.Decimal(len(b)) != part.Bytes || digestText(b) != part.SHA256 {
			return errors.New("transferred file hash/size mismatch")
		}
		all.Write(b)
	}
	if hex.EncodeToString(all.Sum(nil)) != o.SHA256 {
		return errors.New("transferred content hash mismatch")
	}
	return nil
}
