// Package validation exercises bounded application-transport faults around real
// UDP sockets and authenticated control/report workflows. It does not model IP
// packet loss, path MTU, native Linux deployment or independently calibrated clocks.
package validation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/adapter"
	"sneaky-port/internal/control"
	"sneaky-port/internal/elapsed"
	"sneaky-port/internal/engine"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

const faultSlots = 12

// truthEvent records the fixture's exact transformation and local release floor;
// parsed headers remain untrusted until the product independently authenticates them.
type truthEvent struct {
	Action, Role, Digest string
	RunID, SessionID     string
	Kind, Direction      byte
	Identity             measure.Identity
	Attempt              uint64
	Hold                 time.Duration
	Bytes                int
}

// faultScenario owns one finite seeded transformation ledger outside the product.
// The stale frame is retained only to verify a later run rejects an earlier run's bytes.
type faultScenario struct {
	mu                      sync.Mutex
	clock                   faultClock
	mode                    string
	dataDrops, receiptDrops map[uint64]bool
	truth                   []truthEvent
	stale, previous         []byte
	transports              []*faultTransport
}

// faultClock keeps fixture release floors in the production platform's elapsed
// source; tests can freeze that source independently of operating-system timers.
type faultClock interface {
	Now() (uint64, error)
}

// newScenario freezes reproducible random and contiguous burst drops before any traffic.
func newScenario(mode string, clock faultClock) *faultScenario {
	s := &faultScenario{clock: clock, mode: mode, dataDrops: map[uint64]bool{2: true, 3: true}, receiptDrops: map[uint64]bool{}}
	r := rand.New(rand.NewSource(20261013))
	// A fixed-size random subset stays reproducible and cannot accidentally contain no random drops.
	for _, i := range r.Perm(faultSlots) {
		if len(s.dataDrops) == 4 {
			break
		}
		s.dataDrops[uint64(i)] = true
	}
	for i := uint64(0); i < faultSlots && len(s.receiptDrops) < 2; i++ {
		if !s.dataDrops[i] {
			s.receiptDrops[i] = true
		}
	}
	return s
}

// header extracts only enough public framing to choose a fixture action, never authentication.
func header(b []byte) (truthEvent, bool) {
	if len(b) < probe.Overhead || string(b[:4]) != "SPV1" {
		return truthEvent{}, false
	}
	h := sha256.Sum256(b)
	return truthEvent{Digest: hex.EncodeToString(h[:]), RunID: hex.EncodeToString(b[12:28]), SessionID: hex.EncodeToString(b[28:44]), Kind: b[6], Direction: b[7], Identity: measure.Identity{Lifecycle: b[8], Phase: binary.BigEndian.Uint32(b[44:48]), Sequence: profile.Decimal(binary.BigEndian.Uint64(b[48:56]))}, Attempt: binary.BigEndian.Uint64(b[68:76])}, true
}

// record appends bounded fixture truth for this deliberately small workload.
func (s *faultScenario) record(b []byte, role, action string, hold time.Duration) {
	e, _ := header(b)
	e.Action, e.Role, e.Hold, e.Bytes = action, role, hold, len(b)
	s.mu.Lock()
	s.truth = append(s.truth, e)
	s.mu.Unlock()
}

// release is one copied datagram scheduled for the fixture consumer, with its captured local origin.
type release struct {
	bytes     []byte
	seen, due uint64
	action    string
}

// transform applies faults only to main traffic; readiness and real socket addresses remain intact.
func (s *faultScenario) transform(b []byte, role string, now uint64) []release {
	f, ok := header(b)
	out := []release{{bytes: bytes.Clone(b), seen: now, due: now, action: "deliver"}}
	if !ok || f.Identity.Lifecycle != 2 {
		return out
	}
	s.record(b, role, "seen", 0)
	seq := uint64(f.Identity.Sequence)
	if s.mode == "loss" && (role == "server" && f.Kind == probe.Data && f.Direction == 1 && s.dataDrops[seq] || role == "client" && f.Kind == probe.Receipt && f.Direction == 1 && s.receiptDrops[seq]) {
		s.record(b, role, "drop", 0)
		return nil
	}
	if s.mode == "delay" && role == "server" && f.Kind == probe.Data && f.Direction == 1 {
		hold := time.Duration(9+seq%3*2) * time.Millisecond
		if seq == 3 || seq == 8 {
			hold = 140 * time.Millisecond
		}
		out[0].due = now + uint64(hold)
	}
	if s.mode == "duplicate" {
		if role == "server" && f.Kind == probe.Data && f.Direction == 1 {
			if seq == 1 {
				out[0].due = now + uint64(140*time.Millisecond)
			}
			if seq == 4 {
				out = append(out, release{bytes: bytes.Clone(b), seen: now, due: now + uint64(2*time.Millisecond), action: "duplicate"})
			}
			if seq == 0 {
				s.mu.Lock()
				s.previous = bytes.Clone(b)
				stale := bytes.Clone(s.stale)
				s.mu.Unlock()
				if len(stale) > 0 {
					out = append(out, release{bytes: stale, seen: now, due: now + uint64(time.Millisecond), action: "stale-replay"})
				}
			}
			if seq == 6 {
				foreign := bytes.Clone(b)
				foreign[12] ^= 1 // Preserve the original MAC so the tampered foreign run cannot authenticate.
				out = append(out, release{bytes: foreign, seen: now, due: now + uint64(time.Millisecond), action: "foreign-tamper"}, release{bytes: []byte("bad"), seen: now, due: now + uint64(2*time.Millisecond), action: "malformed"})
			}
		}
		if role == "client" && f.Kind == probe.Receipt && f.Direction == 1 && seq == 5 {
			out = append(out, release{bytes: bytes.Clone(b), seen: now, due: now + uint64(2*time.Millisecond), action: "duplicate"})
		}
	}
	return out
}

// receiveResult is one genuine datagram or the terminal socket error from the independent pump.
type receiveResult struct {
	bytes []byte
	err   error
}

// faultTransport retains authentic socket writes/marking/bindings while a bounded
// independent receive pump applies application-level delivery faults before worker capture.
type faultTransport struct {
	adapter.Transport
	scenario *faultScenario
	role     string
	stop     chan struct{}
	output   chan receiveResult
	once     sync.Once
	wg       sync.WaitGroup
}

// wrap starts bounded independent receive/release loops only for a fixture-owned session.
func (s *faultScenario) wrap(t adapter.Transport, role string) adapter.Transport {
	w := &faultTransport{Transport: t, scenario: s, role: role, stop: make(chan struct{}), output: make(chan receiveResult, 256)}
	s.mu.Lock()
	s.transports = append(s.transports, w)
	s.mu.Unlock()
	input := make(chan receiveResult, 16)
	w.wg.Add(2)
	go w.pump(input)
	go w.dispatch(input)
	return w
}

// pump reads real UDP independently of delayed consumer delivery and joins on owned I/O interruption.
func (w *faultTransport) pump(input chan<- receiveResult) {
	defer w.wg.Done()
	defer close(input)
	for {
		b, err := w.Transport.Read()
		select {
		case input <- receiveResult{b, err}:
		case <-w.stop:
			return
		}
		if err != nil {
			return
		}
	}
}

// dispatch maintains at most 256 copied releases; timers only sleep, and the selected
// elapsed source must reach each release floor before delivery, including on early wakes.
// Faults are local application delivery actions after a genuine socket read, not IP packet impairment.
func (w *faultTransport) dispatch(input <-chan receiveResult) {
	defer w.wg.Done()
	var pending []release
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		sort.SliceStable(pending, func(i, j int) bool { // Stable equal-time release order makes duplicate truth reproducible.
			return pending[i].due < pending[j].due
		})
		var tick <-chan time.Time
		if len(pending) > 0 {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			now, err := w.scenario.clock.Now()
			if err != nil {
				w.fail(err)
				return
			}
			delay := time.Duration(0)
			if pending[0].due > now {
				delay = time.Duration(pending[0].due - now)
			}
			timer.Reset(delay)
			tick = timer.C
		}
		select {
		case <-w.stop:
			return
		case r, ok := <-input:
			if !ok {
				return
			}
			if r.err != nil {
				w.fail(r.err)
				return
			}
			now, err := w.scenario.clock.Now()
			if err != nil {
				w.fail(err)
				return
			}
			pending = append(pending, w.scenario.transform(r.bytes, w.role, now)...)
			if len(pending) > 256 {
				w.fail(errors.New("fixture release budget exhausted"))
				return
			}
		case <-tick:
			r := pending[0]
			now, err := w.scenario.clock.Now()
			if err != nil {
				w.fail(err)
				return
			}
			if now < r.due {
				continue
			}
			pending = pending[1:]
			select {
			case w.output <- receiveResult{bytes: r.bytes}:
				w.scenario.record(r.bytes, w.role, r.action, time.Duration(r.due-r.seen))
			case <-w.stop:
				return
			}
		}
	}
}

// fail forwards a terminal native-I/O, selected-clock or budget error without
// blocking fixture cancellation; the receive owner retains the original cause.
func (w *faultTransport) fail(err error) {
	select {
	case w.output <- receiveResult{err: err}:
	case <-w.stop:
	}
}

// Read exposes the transformed complete datagram at the product's declared arrival boundary.
func (w *faultTransport) Read() ([]byte, error) {
	select {
	case r := <-w.output:
		return r.bytes, r.err
	case <-w.stop:
		return nil, net.ErrClosed
	}
}

// Interrupt ends only fixture I/O and joins its pumps while retaining the underlying session socket.
func (w *faultTransport) Interrupt() error {
	w.once.Do(func() { // Wake all local fixture loops exactly once before native I/O interruption.
		close(w.stop)
	})
	err := w.Transport.(interface{ Interrupt() error }).Interrupt()
	w.wg.Wait()
	return err
}

// Close closes only the transferred genuine socket and joins both fixture workers.
func (w *faultTransport) Close() error {
	w.once.Do(func() { // Cancellation precedes socket close so queued releases cannot block cleanup.
		close(w.stop)
	})
	err := w.Transport.Close()
	w.wg.Wait()
	return err
}

// faultFactory keeps the production UDP descriptor/options and wraps only test-owned real sessions.
type faultFactory struct {
	adapter.Factory
	scenario *faultScenario
}

// Dial uses the original adapter plan and actual requested destination before fault ownership begins.
func (f faultFactory) Dial(ctx context.Context, p profile.Plan) (adapter.Transport, error) {
	t, err := f.Factory.Dial(ctx, p)
	if err != nil {
		return nil, err
	}
	return f.scenario.wrap(t, "client"), nil
}

// Prepare preserves the requested production socket binding and rejects no capabilities implicitly.
func (f faultFactory) Prepare(ctx context.Context, p profile.Plan) (adapter.Prepared, error) {
	pd, err := f.Factory.Prepare(ctx, p)
	if err != nil {
		return nil, err
	}
	return faultPrepared{pd, f.scenario}, nil
}

// faultPrepared delegates the owned production listener until a candidate is accepted.
type faultPrepared struct {
	adapter.Prepared
	scenario *faultScenario
}

// Accept transfers a genuine selected UDP session to the bounded fault receive fixture.
func (p faultPrepared) Accept(ctx context.Context) (adapter.Transport, error) {
	t, err := p.Prepared.Accept(ctx)
	if err != nil {
		return nil, err
	}
	return p.scenario.wrap(t, "server"), nil
}

// Reject joins a rejected candidate's fixture pumps, then restores the shared native listener deadline.
func (p faultPrepared) Reject(t adapter.Transport) error {
	w := t.(*faultTransport)
	err := w.Interrupt()
	return errors.Join(err, p.Prepared.Reject(w.Transport))
}

// validationRig owns independent keys/stores, an ephemeral tool-owned SSH server and genuine data sockets.
type validationRig struct {
	client *control.Store
	signer ssh.Signer
	plan   profile.Plan
	state  *faultScenario
}

// newRig creates only fresh application files/listeners; no services or machine settings are changed.
func newRig(t *testing.T, mode string) *validationRig {
	t.Helper()
	root := t.TempDir()
	server, err := control.OpenStore(filepath.Join(root, "server"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := control.OpenStore(filepath.Join(root, "client"))
	if err != nil {
		t.Fatal(err)
	}
	clock, err := elapsed.New()
	if err != nil {
		t.Fatal(err)
	}
	state := newScenario(mode, clock)
	registry := adapter.NewRegistry()
	udp, _ := adapter.Default().Get("udp")
	if err = registry.Register(faultFactory{udp, state}); err != nil {
		t.Fatal(err)
	}
	server.Adapters, client.Adapters = registry, registry
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Release only this fixture listener if setup or assertions fail.
		ln.Close()
	})
	policy := control.ServerPolicy{Version: 1, ControlListen: ln.Addr().String(), DataBindIP: "127.0.0.1", PortMin: 1, PortMax: 65535, SetupTimeout: 3e9, ShutdownTimeout: 2e9, AuthPoll: 20e6, MaxConnections: 8, MaxAuthTries: 3, MaxControlRequests: 1000, MaxBindAttempts: 16, MaxQueueBytes: 1 << 20, MaxLedgerMessages: 1000, MaxEventBytes: 4 << 20, MaxFileBytes: 8 << 20}
	bundle, err := server.InitServer("validation-server", "127.0.0.1", ln.Addr().(*net.TCPAddr).Port, policy, "")
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := client.InitClient("validation-client", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = server.Enroll(enrollment); err != nil {
		t.Fatal(err)
	}
	if err = client.ImportPairing("fixture", bundle, false); err != nil {
		t.Fatal(err)
	}
	si, _ := server.Server()
	host, err := control.LoadKey(si.KeyRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	ci, _ := client.Client()
	signer, err := control.LoadKey(ci.KeyRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := control.NewServer(server, host)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { // Own real key-authenticated SSH plus production data/report channels until cleanup.
		done <- s.Serve(ctx, ln)
	}()
	t.Cleanup(func() { // Join the tool-owned server and all fixture session pumps within a bounded cleanup.
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("controlled fixture server cleanup timeout")
		}
	})
	p := testutil.Plan()
	p.Phases[0].Duration, p.Grace, p.Deadline = 600e6, 300e6, 60e6
	p.Phases[0].Client.RateNumerator, p.Phases[0].Server.RateNumerator = 20, 20
	p.Limits.MaxLag, p.Limits.HeartbeatInterval, p.Limits.ControlRequests = 1e9, 50e6, 1000
	p.Limits.QueueMessages, p.Limits.EventBytes, p.Limits.FileBytes = 128, 4<<20, 8<<20
	p.SpikeThreshold = 100e6
	reserve, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p.DestinationPort = reserve.LocalAddr().(*net.UDPAddr).Port
	reserve.Close()
	return &validationRig{client, signer, p, state}
}

// run authenticates both endpoints, transfers the real server report and independently replays exact summaries.
func (r *validationRig) run(t *testing.T) (*control.ReplayedReport, control.RunResult) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := control.Run(ctx, r.client, r.signer, r.plan)
	if err != nil || result.Status != "completed" || result.Remote == nil || result.Remote.Status != "verified" || result.Reconciliation == nil || result.Reconciliation.Status != "reconciled" {
		t.Fatalf("controlled real run/transfer: err=%v result=%+v", err, result)
	}
	replayed, err := control.ReplayReport(result.ReportPath, 32<<20, r.client.Adapters)
	if err != nil || replayed.Verification != "matched" || replayed.ServerReport == nil || replayed.ServerReport.Verification != "matched" || !reflect.DeepEqual(replayed.Reconciliation, result.Reconciliation) {
		t.Fatalf("controlled saved replay: %v", err)
	}
	for _, format := range []string{"json", "jsonl", "csv"} {
		var output bytes.Buffer
		if err = control.ExportReport(&output, replayed, format); err != nil || !bytes.Contains(output.Bytes(), []byte(result.Readiness.RunID)) {
			t.Fatalf("controlled %s export: %v", format, err)
		}
	}
	if _, err = os.Stat(filepath.Join(r.client.Root, "active-run.lock")); !os.IsNotExist(err) {
		t.Fatal("controlled run reservation leaked")
	}
	truth := r.snapshot()
	assertControlledReads(t, replayed.Evidence, truth, "client", result.Readiness)
	assertControlledReads(t, replayed.ServerReport.Evidence, truth, "server", result.Readiness)
	assertControlledPopulation(t, replayed.Evidence, replayed.ServerReport.Evidence, result.Reconciliation.Client.Summary)
	assertControlledPopulation(t, replayed.ServerReport.Evidence, replayed.Evidence, result.Reconciliation.Server.Summary)
	assertControlledTruth(t, replayed.Evidence, replayed.ServerReport.Evidence, truth, result.Readiness)
	r.retain(t, result)
	return replayed, result
}

// assertControlledTruth links every current-run transformed original to its real
// attempt and forbids injected drops from becoming delivery or receipt evidence.
// An unobserved, unconsumed or outside-cutoff original remains separately counted.
func assertControlledTruth(t *testing.T, client, server engine.Result, truth []truthEvent, ready control.ReadyResult) {
	t.Helper()
	origins := map[byte]engine.Result{1: client, 2: server}
	entries := map[byte]map[measure.Identity]measure.Entry{1: {}, 2: {}}
	for direction, origin := range origins {
		for _, entry := range origin.Entries {
			entries[direction][entry.Identity] = entry
		}
	}
	for _, event := range truth {
		if event.Identity.Lifecycle != 2 || event.Action == "stale-replay" || event.Action == "foreign-tamper" {
			continue
		}
		entry, ok := entries[event.Direction][event.Identity]
		if !ok || entry.Attempt == nil || uint64(*entry.Attempt) != event.Attempt || event.RunID != ready.RunID || event.SessionID != ready.SessionID {
			t.Fatalf("transformed bytes lack exact current original attempt: truth=%+v entry=%+v", event, entry)
		}
		if event.Action == "drop" {
			if event.Direction != 1 {
				t.Fatalf("fixture applied an unconfigured reverse-direction drop: %+v", event)
			}
			if event.Kind == probe.Data {
				if _, exists := server.Receiver.Arrivals[event.Identity]; exists {
					t.Fatalf("injected data drop became authenticated receiver evidence: %+v", event)
				}
			} else if event.Kind != probe.Receipt || entry.Receipt != nil {
				t.Fatalf("injected receipt drop became an origin sample: truth=%+v entry=%+v", event, entry)
			}
		}
	}
}

// snapshot copies joined fixture truth under its owner lock, including the unconsumed
// release suffix when a busy native endpoint legitimately reaches its finite cutoff.
func (r *validationRig) snapshot() []truthEvent {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	return append([]truthEvent(nil), r.state.truth...)
}

// assertControlledReads checks the exact consumed prefix of the fixture's FIFO
// releases. Native scheduling may leave an unconsumed suffix; it may never create,
// substitute or reorder a consumed frame, duplicate or malformed-frame rejection.
func assertControlledReads(t *testing.T, evidence engine.Result, truth []truthEvent, role string, ready control.ReadyResult) {
	t.Helper()
	var released []truthEvent
	for _, event := range truth {
		if event.Role == role && event.Action != "seen" && event.Action != "drop" && (event.Identity.Lifecycle == 2 || event.Action == "malformed") {
			released = append(released, event)
		}
	}
	seen := map[[5]uint64]bool{}
	consumed := 0
	for _, raw := range evidence.Records {
		var rec engine.Record
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatal(err)
		}
		if rec.Kind != "invalid" && (rec.Kind != "arrival" && rec.Kind != "duplicate" || rec.Identity.Lifecycle != 2) {
			continue
		}
		if consumed == len(released) {
			t.Fatalf("%s consumed frame without fixture release: %+v", role, rec)
		}
		event := released[consumed]
		consumed++
		invalid := event.Action == "malformed" || event.Action == "foreign-tamper" || event.Action == "stale-replay"
		if invalid {
			if rec.Kind != "invalid" || rec.Bytes != profile.Decimal(event.Bytes) || rec.Error == "" {
				t.Fatalf("%s injected rejection mismatch: truth=%+v record=%+v", role, event, rec)
			}
			continue
		}
		key := [5]uint64{uint64(event.Kind), uint64(event.Direction), uint64(event.Identity.Lifecycle), uint64(event.Identity.Phase), uint64(event.Identity.Sequence)}
		kind := "arrival"
		if seen[key] {
			kind = "duplicate"
		}
		seen[key] = true
		if rec.Kind != kind || rec.FrameKind != event.Kind || rec.Direction != event.Direction || rec.Identity != event.Identity || uint64(rec.Attempt) != event.Attempt || rec.Bytes != profile.Decimal(event.Bytes) || event.RunID != ready.RunID || event.SessionID != ready.SessionID {
			t.Fatalf("%s exact released identity mismatch: truth=%+v record=%+v want_kind=%s", role, event, rec, kind)
		}
	}
	t.Logf("%s native receive prefix: consumed=%d released=%d unconsumed=%d", role, consumed, len(released), len(released)-consumed)
}

// assertControlledPopulation independently counts all twelve original slots,
// explicit local skips, cutoff-eligible writes/receipts and compatible receiver
// identities. A host pause cannot become injected application loss or a lost slot.
func assertControlledPopulation(t *testing.T, origin, peer engine.Result, summary measure.Summary) {
	t.Helper()
	if !origin.Complete || origin.ObservationCutoff < 900e6 || origin.GenerationCutoff < origin.ObservationCutoff-300e6 {
		t.Fatalf("controlled completed generation/observation boundaries mismatch: complete=%v generation=%d observation=%d", origin.Complete, origin.GenerationCutoff, origin.ObservationCutoff)
	}
	start := origin.ObservationCutoff - 900e6 // Complete fixed plan cutoffs retain the original start even after a delayed generation stop.
	if len(origin.Entries) != faultSlots || summary.Planned != faultSlots || summary.TransportPacketLoss != nil {
		t.Fatalf("controlled original population or unknown packet loss changed: entries=%d summary=%+v", len(origin.Entries), summary)
	}
	var skipped, accepted, attempted, written, onTime, late, unresolved, received, missing, duplicates profile.Decimal
	ids := map[measure.Identity]bool{}
	skipReasons := map[measure.Identity]string{}
	for _, raw := range origin.Records {
		var rec engine.Record
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatal(err)
		}
		if rec.Kind == "schedule_skip" {
			if rec.Count == 0 || rec.Count > faultSlots {
				t.Fatalf("invalid native scheduler skipped range: %+v", rec)
			}
			for i := profile.Decimal(0); i < rec.Count; i++ {
				id := rec.Identity
				id.Sequence += i
				skipReasons[id] = rec.Reason
			}
		} else if rec.Kind == "admission" && rec.FrameKind == probe.Data {
			if rec.Admission == "accepted" {
				if rec.At < origin.ObservationCutoff {
					accepted++
				}
			} else {
				skipReasons[rec.Identity] = rec.Reason
			}
		} else if rec.Kind == "ticket_observed" && rec.FrameKind == probe.Data && !rec.CallbackInvoked {
			skipReasons[rec.Identity] = "canceled_unattempted: " + rec.Reason
		} else if rec.Kind == "unattempted" && rec.FrameKind == probe.Data {
			skipReasons[rec.Identity] = rec.Reason
		}
	}
	for _, entry := range origin.Entries {
		id := entry.Identity
		if id.Lifecycle != 2 || id.Phase != 0 || id.Sequence >= faultSlots || ids[id] || entry.Scheduled != start+id.Sequence*50e6 || entry.Deadline != entry.Scheduled+60e6 {
			t.Fatalf("controlled exact original identity/schedule mismatch: %+v", entry)
		}
		ids[id] = true
		if entry.State == "skipped_local" {
			if entry.Reason == "" || skipReasons[id] != entry.Reason || entry.Attempt != nil || entry.Receipt != nil {
				t.Fatalf("controlled local skip became traffic: %+v", entry)
			}
			if (entry.Admitted == nil || *entry.Admitted < origin.ObservationCutoff) && (entry.LocalResolved == nil || *entry.LocalResolved < origin.ObservationCutoff) {
				skipped++
				continue
			}
		}
		if entry.Attempt == nil || *entry.Attempt >= origin.ObservationCutoff {
			unresolved++
			continue
		}
		attempted++
		if entry.State != "written" || entry.WriteCompleted == nil || *entry.WriteCompleted < *entry.Attempt {
			t.Fatalf("controlled actual attempt failed to complete its native write: %+v", entry)
		}
		if *entry.WriteCompleted < origin.ObservationCutoff {
			written++
		}
		if entry.Receipt != nil && *entry.Receipt < origin.ObservationCutoff {
			if *entry.Receipt < *entry.Attempt {
				t.Fatalf("controlled receipt precedes original attempt: %+v", entry)
			}
			if *entry.Receipt <= entry.Deadline {
				onTime++
			} else {
				late++
			}
		} else {
			unresolved++
		}
		if arrival, ok := peer.Receiver.Arrivals[id]; ok && arrival < uint64(peer.ObservationCutoff) {
			received++
		} else if *entry.WriteCompleted < origin.ObservationCutoff {
			missing++
		}
	}
	for _, rec := range records(t, origin, "duplicate", probe.Receipt) {
		if rec.At < origin.ObservationCutoff {
			duplicates++
		}
	}
	arrivals := map[measure.Identity]uint64{}
	for _, rec := range records(t, peer, "arrival", probe.Data) {
		if _, exists := arrivals[rec.Identity]; exists {
			t.Fatalf("unique authenticated receiver identity repeated: %+v", rec)
		}
		arrivals[rec.Identity] = uint64(rec.At)
	}
	if !reflect.DeepEqual(arrivals, peer.Receiver.Arrivals) {
		t.Fatal("authenticated receiver snapshot differs from exact raw arrivals")
	}
	if summary.Skipped != skipped || summary.LocalMisses != skipped || summary.Accepted != accepted || summary.Attempted != attempted || summary.Written != written || summary.WriteFailed != 0 || summary.OnTime != onTime || summary.Late != late || summary.Unresolved != unresolved || summary.RTTSamples != onTime+late || summary.DuplicateReceipts != duplicates || summary.Received == nil || *summary.Received != received || summary.MissingAtReceiver == nil || *summary.MissingAtReceiver != missing || skipped+onTime+late+unresolved != faultSlots {
		t.Fatalf("controlled cutoff-bound populations mismatch: counts skipped/attempted/written/on-time/late/unresolved/received/missing/duplicates=%d/%d/%d/%d/%d/%d/%d/%d/%d summary=%+v", skipped, attempted, written, onTime, late, unresolved, received, missing, duplicates, summary)
	}
	t.Logf("direction=%d original population: planned=%d skipped=%d written=%d samples=%d receiver=%d missing_application_data=%d unresolved=%d", origin.Direction, summary.Planned, skipped, written, onTime+late, received, missing, unresolved)
}

// retain optionally copies only public report files and fixture truth; temporary endpoint keys are cleaned normally.
// Every run gets a fresh directory, and offline replay remains consistency evidence rather than renewed authentication.
func (r *validationRig) retain(t *testing.T, result control.RunResult) {
	t.Helper()
	base := os.Getenv("SNEAKY_PORT_CONTROLLED_EVIDENCE")
	if base == "" {
		return
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, r.state.mode+"-")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.CopyFS(filepath.Join(dir, "report"), os.DirFS(result.ReportPath)); err != nil {
		t.Fatal(err)
	}
	r.state.mu.Lock()
	truth := append([]truthEvent(nil), r.state.truth...)
	r.state.mu.Unlock()
	metadata := struct {
		Boundary string
		Scenario string
		Seed     int64
		Ready    control.ReadyResult
		Data     map[uint64]bool
		Receipts map[uint64]bool
		Truth    []truthEvent
	}{"Fixture receive pump after authentic UDP socket read, before application arrival; Hold is minimum selected-platform elapsed-source release delay in ns. Packet loss/path MTU/cross-host one-way/native Linux are unproven.", r.state.mode, 20261013, result.Readiness, r.state.dataDrops, r.state.receiptDrops, truth}
	b, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "truth.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("retained public truth plus local/transferred endpoint reports: %s", dir)
}

// records decodes saved raw engine details without trusting cached summaries or subtracting endpoint clocks.
func records(t *testing.T, r engine.Result, kind string, frame byte) []engine.Record {
	t.Helper()
	var out []engine.Record
	for _, raw := range r.Records {
		var rec engine.Record
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatal(err)
		}
		if rec.Kind == kind && rec.FrameKind == frame {
			out = append(out, rec)
		}
	}
	return out
}

// TestControlledDelaySpike checks actual native jitter/spike release floors and
// identities without assuming the host can admit or consume every planned slot.
func TestControlledDelaySpike(t *testing.T) {
	r := newRig(t, "delay")
	replayed, result := r.run(t)
	entries := map[measure.Identity]measure.Entry{}
	for _, e := range replayed.Evidence.Entries {
		entries[e.Identity] = e
	}
	spikes := 0
	for _, truth := range r.snapshot() {
		if truth.Action != "deliver" || truth.Role != "server" || truth.Kind != probe.Data || truth.Direction != 1 || truth.Identity.Lifecycle != 2 {
			continue
		}
		e := entries[truth.Identity]
		if e.Attempt == nil {
			t.Fatalf("held datum missing original attempt: truth=%+v entry=%+v", truth, e)
		}
		if uint64(*e.Attempt) != truth.Attempt || truth.RunID != result.Readiness.RunID || truth.SessionID != result.Readiness.SessionID {
			t.Fatalf("held datum identity mismatch: truth=%+v attempt_ns=%d receipt_ns=%v run=%s session=%s", truth, *e.Attempt, e.Receipt, result.Readiness.RunID, result.Readiness.SessionID)
		}
		if e.Receipt == nil {
			continue // A queued native release need not be consumed before either finite cutoff.
		}
		rtt := time.Duration(*e.Receipt - *e.Attempt)
		if rtt < truth.Hold {
			t.Fatalf("held datum RTT floor mismatch: identity=%+v attempt_ns=%d receipt_ns=%d rtt_ns=%d hold_ns=%d", truth.Identity, *e.Attempt, *e.Receipt, rtt, truth.Hold)
		}
		if truth.Hold == 140*time.Millisecond {
			spikes++
			if *e.Receipt < e.Deadline {
				t.Fatal("known spike became on-time")
			}
		}
	}
	if spikes > 2 {
		t.Fatal("controlled spike fixture repeated an original identity")
	}
	up, down := result.Reconciliation.Client.Summary, result.Reconciliation.Server.Summary
	t.Logf("native application receive-boundary delay: configured 9/11/13 ms jitter and two 140 ms spikes; observed spike receipts=%d upstream samples=%d late=%d downstream samples=%d late=%d", spikes, up.RTTSamples, up.Late, down.RTTSamples, down.Late)
}

// TestControlledDataAndReceiptLoss separates seeded/burst dropped data from receipts lost after real receiver delivery.
func TestControlledDataAndReceiptLoss(t *testing.T) {
	r := newRig(t, "loss")
	replayed, result := r.run(t)
	up, down := result.Reconciliation.Client.Summary, result.Reconciliation.Server.Summary
	for _, e := range replayed.Evidence.Entries {
		seq := uint64(e.Identity.Sequence)
		_, received := replayed.ServerReport.Evidence.Receiver.Arrivals[e.Identity]
		if r.state.dataDrops[seq] && received || (r.state.dataDrops[seq] || r.state.receiptDrops[seq]) && e.Receipt != nil {
			t.Fatalf("controlled exact identity mismatch: %+v received=%v", e, received)
		}
	}
	dataDrops, receiptDrops := 0, 0
	for _, truth := range r.snapshot() {
		if truth.Action == "drop" {
			seq := uint64(truth.Identity.Sequence)
			if truth.Role == "server" && truth.Kind == probe.Data && r.state.dataDrops[seq] {
				dataDrops++
			} else if truth.Role == "client" && truth.Kind == probe.Receipt && r.state.receiptDrops[seq] {
				receiptDrops++
			} else {
				t.Fatalf("fixture dropped an unconfigured identity: %+v", truth)
			}
		}
	}
	if dataDrops > len(r.state.dataDrops) || receiptDrops > len(r.state.receiptDrops) {
		t.Fatal("fixture repeated an injected drop identity")
	}
	t.Logf("seed=20261013; configured application data drops=%v (includes burst 2,3), receipt drops=%v; actual injected drops data/receipt=%d/%d; upstream samples=%d receiver missing=%d unresolved=%d; downstream samples=%d", r.state.dataDrops, r.state.receiptDrops, dataDrops, receiptDrops, up.RTTSamples, *up.MissingAtReceiver, up.Unresolved, down.RTTSamples)
}

// TestControlledDuplicateReorderAndReplay preserves unique populations through duplicated data/receipts,
// observed sequence reordering, malformed/tampered foreign frames and stale bytes from a prior authenticated run.
func TestControlledDuplicateReorderAndReplay(t *testing.T) {
	r := newRig(t, "duplicate")
	for run := 0; run < 2; run++ {
		replayed, result := r.run(t)
		up := result.Reconciliation.Client.Summary
		arrivals := records(t, replayed.ServerReport.Evidence, "arrival", probe.Data)
		invalid := records(t, replayed.ServerReport.Evidence, "invalid", 0)
		r.state.mu.Lock()
		r.state.stale = bytes.Clone(r.state.previous)
		r.state.truth = nil
		r.state.mu.Unlock()
		t.Logf("authenticated native run %d: %d observed unique data, %d duplicate data, %d cutoff-eligible duplicate receipts, %d rejected injected frames; %d samples/%d local skips; exact release prefix and replay/export matched", run+1, len(arrivals), len(records(t, replayed.ServerReport.Evidence, "duplicate", probe.Data)), up.DuplicateReceipts, len(invalid), up.RTTSamples, up.Skipped)
	}
}
