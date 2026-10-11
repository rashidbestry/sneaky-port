package control

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/adapter"
	"sneaky-port/internal/platform"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

const ChannelType = "sneaky-port/control/v1"

// Effective records endpoint-local bindings and API marking evidence, never packet observations.
type Effective struct {
	Local    string             `json:"local"`
	Remote   string             `json:"remote"`
	Markings []platform.Marking `json:"markings"`
}

// ReadyResult deliberately contains no session secrets or measurement samples.
type ReadyResult struct {
	Version            int            `json:"schema_version"`
	Status             string         `json:"status"`
	MeasurementStarted bool           `json:"measurement_started"`
	RunID              string         `json:"run_id"`
	SessionID          string         `json:"session_id"`
	PlanHash           string         `json:"plan_hash"`
	Client             Effective      `json:"client"`
	Server             Effective      `json:"server"`
	Control            ControlTraffic `json:"control_traffic"`
}

// ControlTraffic counts complete application-channel bytes at the client, excluding SSH/network overhead and other requests.
type ControlTraffic struct {
	Scope    string          `json:"scope"`
	Requests int             `json:"requests"`
	Sent     profile.Decimal `json:"sent_bytes"`
	Received profile.Decimal `json:"received_bytes"`
}

// frameCounter counts serialized bytes through the sole application reader/writer, without claiming wire traffic.
type frameCounter struct {
	ssh.Channel
	sent, received profile.Decimal
}

// Read retains bytes actually returned even when the SSH channel closes partially.
func (c *frameCounter) Read(b []byte) (int, error) {
	n, err := c.Channel.Read(b)
	c.received += profile.Decimal(n)
	return n, err
}

// Write retains bytes accepted by the channel even if the control frame fails partway.
func (c *frameCounter) Write(b []byte) (int, error) {
	n, err := c.Channel.Write(b)
	c.sent += profile.Decimal(n)
	return n, err
}

// helloBody advertises only capabilities implemented by this coordination stage.
type helloBody struct {
	Contracts   []int                `json:"contracts"`
	Adapters    []adapter.Descriptor `json:"adapters"`
	Measurement bool                 `json:"measurement_supported"`
}

// prepareBody commits the full normalized plan and its exact canonical digest.
type prepareBody struct {
	Plan json.RawMessage `json:"plan"`
	Hash string          `json:"plan_hash"`
}

// offerBody carries ephemeral data-authentication secrets only inside authenticated SSH.
type offerBody struct {
	Hash      string `json:"plan_hash"`
	ClientKey string `json:"client_key"`
	ServerKey string `json:"server_key"`
	Challenge string `json:"challenge"`
	Binding   string `json:"server_binding"`
}

// confirmBody acknowledges client-local effective settings after data authentication.
type confirmBody struct {
	Hash   string    `json:"plan_hash"`
	Client Effective `json:"client"`
}

// hashBody binds liveness and stop requests to the prepared plan.
type hashBody struct {
	Hash string `json:"plan_hash"`
}

// errorBody exposes a bounded stage error without encoding raw requests or secrets.
type errorBody struct {
	Message string `json:"message"`
}

// Server owns embedded SSH connections and permits one active preparation/readiness session.
type Server struct {
	Store    *Store
	Identity ServerIdentity
	Signer   ssh.Signer
	Registry *adapter.Registry
	active   chan struct{}
}

// NewServer verifies that the loaded private-key reference still matches the persisted public identity.
func NewServer(store *Store, signer ssh.Signer) (*Server, error) {
	id, err := store.Server()
	if err != nil {
		return nil, err
	}
	key, err := ParsePublic(id.Bundle.HostPublicKey)
	if err != nil {
		return nil, err
	}
	if signer == nil || !bytes.Equal(key.Marshal(), signer.PublicKey().Marshal()) {
		return nil, errors.New("host private key does not match initialized identity")
	}
	return &Server{store, id, signer, adapter.Default(), make(chan struct{}, 1)}, nil
}

// Serve bounds connections and closes every owned connection/listener when its context ends.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer ln.Close()
	stop := context.AfterFunc(ctx, func() { // Interrupt the owned accept operation on cancellation.
		ln.Close()
	})
	defer stop()
	slots := make(chan struct{}, s.Identity.Policy.MaxConnections)
	var wg sync.WaitGroup
	defer func() { // Unblock connection workers before waiting for their cleanup.
		cancel()
		wg.Wait()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case slots <- struct{}{}:
		default:
			c.Close()
			continue
		}
		wg.Add(1)
		go func() { // Each worker exclusively owns its raw SSH connection and cleanup.
			defer wg.Done()
			defer func() { // Release the connection budget after all owned resources close.
				<-slots
			}()
			s.connection(ctx, c)
		}()
	}
}

// connection enforces key-only authentication, periodic revocation and application channels only.
func (s *Server) connection(ctx context.Context, raw net.Conn) {
	defer raw.Close()
	stop := context.AfterFunc(ctx, func() { // Close blocked handshake/control IO when the server stops.
		raw.Close()
	})
	defer stop()
	policy := s.Identity.Policy
	raw.SetDeadline(time.Now().Add(time.Duration(policy.SetupTimeout)))
	config := &ssh.ServerConfig{MaxAuthTries: policy.MaxAuthTries, PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) { // Consult current application authorization for every offered key.
		id, err := s.Store.CheckClient(key, time.Now())
		if err != nil {
			return nil, err
		}
		if meta.User() != id {
			return nil, errors.New("SSH user must match enrolled client ID")
		}
		return &ssh.Permissions{Extensions: map[string]string{"client_id": id, "public_key": string(ssh.MarshalAuthorizedKey(key))}}, nil
	}}
	config.AddHostKey(s.Signer)
	conn, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { // Propagate authenticated SSH EOF to prepared data sockets immediately.
		conn.Wait()
		cancel()
	}()
	stopConn := context.AfterFunc(ctx, func() { // Couple control failure/revocation to all data work on this connection.
		raw.Close()
	})
	defer stopConn()
	go rejectRequests(ctx, requests, policy.MaxControlRequests, cancel)
	go s.watchAuthorization(ctx, conn.Permissions, cancel)
	select {
	case <-ctx.Done():
		return
	case channel, ok := <-channels:
		if !ok {
			return
		}
		if channel.ChannelType() != ChannelType || len(channel.ExtraData()) != 0 {
			channel.Reject(ssh.Prohibited, "application control channel only")
			return
		}
		ch, reqs, e := channel.Accept()
		if e != nil {
			return
		}
		defer ch.Close()
		go rejectRequests(ctx, reqs, policy.MaxControlRequests, cancel)
		go rejectChannels(ctx, channels, policy.MaxControlRequests, cancel)
		s.control(ctx, raw, ch)
	}
}

// rejectRequests refuses shell-like channel/global requests with a finite flood budget.
func rejectRequests(ctx context.Context, requests <-chan *ssh.Request, max int, cancel context.CancelFunc) {
	for n := 0; ; n++ {
		select {
		case <-ctx.Done():
			return
		case r, ok := <-requests:
			if !ok {
				return
			}
			r.Reply(false, nil)
			if n+1 >= max {
				cancel()
				return
			}
		}
	}
}

// rejectChannels promptly refuses additional channels instead of exposing SSH forwarding or subsystems.
func rejectChannels(ctx context.Context, channels <-chan ssh.NewChannel, max int, cancel context.CancelFunc) {
	for n := 0; ; n++ {
		select {
		case <-ctx.Done():
			return
		case ch, ok := <-channels:
			if !ok {
				return
			}
			ch.Reject(ssh.Prohibited, "one application control channel allowed")
			if n+1 >= max {
				cancel()
				return
			}
		}
	}
}

// watchAuthorization revokes existing connections on disabled, expired, changed or unreadable enrollment.
func (s *Server) watchAuthorization(ctx context.Context, p *ssh.Permissions, cancel context.CancelFunc) {
	key, err := ParsePublic(p.Extensions["public_key"])
	if err != nil {
		cancel()
		return
	}
	ticker := time.NewTicker(time.Duration(s.Identity.Policy.AuthPoll))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			id, err := s.Store.CheckClient(key, now)
			if err != nil || id != p.Extensions["client_id"] {
				cancel()
				return
			}
		}
	}
}

// message builds one bounded application envelope; only the offer contains ephemeral secrets.
func message(kind, request, run, session string, body any) Envelope {
	b, _ := json.Marshal(body)
	return Envelope{1, kind, request, run, session, b}
}

// reply correlates responses to the exact request and lifecycle identity.
func reply(ch ssh.Channel, request Envelope, kind string, body any) error {
	return Write(ch, message(kind, request.RequestID, request.RunID, request.SessionID, body))
}

// control performs plan agreement and readiness; start/calibration/report channels remain unsupported.
func (s *Server) control(ctx context.Context, raw net.Conn, ch ssh.Channel) {
	policy := s.Identity.Policy
	seen := map[string]bool{}
	read := func(run, session string) (Envelope, error) { // Enforce bounded, unique requests and exact lifecycle identity.
		e, err := Read(ch)
		if err != nil {
			return e, err
		}
		if !profile.SafeID(e.RequestID) || seen[e.RequestID] || len(seen) >= policy.MaxControlRequests || e.RunID != run || e.SessionID != session {
			return e, errors.New("invalid request identity or control budget")
		}
		seen[e.RequestID] = true
		return e, nil
	}
	fail := func(e Envelope, err error) { // Return a stage error without logging the secret-bearing request/offer.
		reply(ch, e, "error", errorBody{err.Error()})
	}
	hello, err := read("", "")
	if err != nil {
		return
	}
	var h helloBody
	if hello.Type != "hello" || DecodeObject(hello.Body, &h) != nil || len(h.Contracts) != 1 || h.Contracts[0] != 1 {
		fail(hello, errors.New("contract 1 hello required"))
		return
	}
	if err = reply(ch, hello, "hello", helloBody{[]int{1}, s.Registry.Describe(), false}); err != nil {
		return
	}
	e, err := Read(ch)
	if err != nil {
		return
	}
	run, err := probe.ID(e.RunID)
	if err != nil || e.Type != "prepare" || e.SessionID != "" || !profile.SafeID(e.RequestID) || seen[e.RequestID] {
		fail(e, errors.New("prepare with fresh run identity required"))
		return
	}
	seen[e.RequestID] = true
	var prep prepareBody
	if err = DecodeObject(e.Body, &prep); err != nil {
		fail(e, err)
		return
	}
	plan, err := profile.Decode(bytes.NewReader(prep.Plan))
	if err != nil {
		fail(e, err)
		return
	}
	hash, err := plan.Hash()
	if err != nil || prep.Hash != hex.EncodeToString(hash[:]) {
		fail(e, errors.New("frozen plan hash mismatch"))
		return
	}
	factory, err := s.Registry.Get(plan.Adapter)
	if err == nil {
		err = factory.Validate(plan)
	}
	if err == nil {
		err = policy.Check(plan)
	}
	if err != nil {
		fail(e, err)
		return
	}
	if len(seen) >= plan.Limits.ControlRequests {
		fail(e, errors.New("control request budget exhausted"))
		return
	}
	select {
	case s.active <- struct{}{}:
	default:
		fail(e, errors.New("server busy with active readiness/run"))
		return
	}
	defer func() { // Release the one-session reservation only after socket cleanup.
		<-s.active
	}()
	setup, cancel := context.WithTimeout(ctx, time.Duration(plan.Limits.SetupTimeout))
	defer cancel()
	deadline, _ := setup.Deadline()
	raw.SetDeadline(deadline)
	localPlan := plan
	localPlan.ServerAddress = policy.DataBindIP
	prepared, err := factory.Prepare(setup, localPlan)
	if err != nil {
		fail(e, err)
		return
	}
	defer prepared.Close()
	closePrepared := context.AfterFunc(ctx, func() { // Interrupt data setup when SSH/server ownership ends.
		prepared.Close()
	})
	defer closePrepared()
	var session [16]byte
	var clientKey, serverKey, challenge [32]byte
	for _, b := range [][]byte{session[:], clientKey[:], serverKey[:], challenge[:]} {
		if _, err = rand.Read(b); err != nil {
			fail(e, errors.New("session entropy unavailable"))
			return
		}
	}
	e.SessionID = hex.EncodeToString(session[:])
	offer := offerBody{prep.Hash, hex.EncodeToString(clientKey[:]), hex.EncodeToString(serverKey[:]), hex.EncodeToString(challenge[:]), prepared.Address()}
	writeDeadline := time.Now().Add(time.Duration(plan.Limits.WriteTimeout))
	if deadline.Before(writeDeadline) {
		writeDeadline = deadline
	}
	raw.SetWriteDeadline(writeDeadline)
	if err = reply(ch, e, "prepared", offer); err != nil {
		return
	}
	ca, sa := probe.Auth{Plan: hash, Adapter: plan.Adapter, Key: clientKey}, probe.Auth{Plan: hash, Adapter: plan.Adapter, Key: serverKey}
	t, err := acceptBind(setup, prepared, ca, sa, run, session, challenge, factory.Describe().MaxFrame, policy.MaxBindAttempts, time.Duration(plan.Limits.WriteTimeout))
	if err != nil {
		fail(e, err)
		return
	}
	defer t.Close()
	marks, err := markPlan(t, plan, false)
	if err != nil {
		fail(e, err)
		return
	}
	server := Effective{t.Local(), t.Remote(), marks}
	confirmation, err := read(e.RunID, e.SessionID)
	if err != nil {
		return
	}
	var confirmed confirmBody
	if confirmation.Type != "confirm" || DecodeObject(confirmation.Body, &confirmed) != nil || confirmed.Hash != prep.Hash {
		fail(confirmation, errors.New("exact plan/effective-settings confirmation required"))
		return
	}
	if err = checkEffective(confirmed.Client, plan, true); err != nil {
		fail(confirmation, err)
		return
	}
	result := ReadyResult{Version: 1, Status: "ready", RunID: e.RunID, SessionID: e.SessionID, PlanHash: prep.Hash, Client: confirmed.Client, Server: server, Control: ControlTraffic{Scope: "client counters finalized after stop"}}
	raw.SetWriteDeadline(time.Now().Add(time.Duration(plan.Limits.WriteTimeout)))
	if err = reply(ch, confirmation, "ready", result); err != nil {
		return
	}
	for {
		raw.SetDeadline(time.Now().Add(time.Duration(plan.Limits.Lease)))
		request, err := read(e.RunID, e.SessionID)
		if err != nil {
			return
		}
		if len(seen) > plan.Limits.ControlRequests {
			fail(request, errors.New("plan control request budget exhausted"))
			return
		}
		var body hashBody
		if DecodeObject(request.Body, &body) != nil || body.Hash != prep.Hash {
			fail(request, errors.New("liveness plan mismatch"))
			return
		}
		switch request.Type {
		case "heartbeat":
			raw.SetWriteDeadline(time.Now().Add(time.Duration(plan.Limits.WriteTimeout)))
			if reply(ch, request, "alive", body) != nil {
				return
			}
		case "stop":
			t.Close()
			prepared.Close()
			budget := plan.Limits.WriteTimeout
			if plan.Limits.ShutdownTimeout < budget {
				budget = plan.Limits.ShutdownTimeout
			}
			raw.SetWriteDeadline(time.Now().Add(time.Duration(budget)))
			reply(ch, request, "stopped", body)
			return
		default:
			fail(request, errors.New("unsupported lifecycle message; measurement start is not implemented"))
			return
		}
	}
}

// acceptBind authenticates a candidate before replying or locking its tuple; rejected UDP receives no amplification.
func acceptBind(ctx context.Context, p adapter.Prepared, ca, sa probe.Auth, run, session [16]byte, challenge [32]byte, max, attempts int, write time.Duration) (adapter.Transport, error) {
	deadline, _ := ctx.Deadline()
	for i := 0; i < attempts; i++ {
		t, err := p.Accept(ctx)
		if err != nil {
			return nil, err
		}
		stopCandidate := context.AfterFunc(ctx, func() { // Interrupt a stalled accepted candidate at the setup/cancellation boundary.
			t.Close()
		})
		t.SetReadDeadline(deadline)
		b, err := t.Read()
		if err == nil {
			var f probe.Frame
			f, err = ca.Decode(b, max)
			if err == nil {
				err = checkBind(f, probe.BindRequest, 1, run, session, challenge)
			}
		}
		if err != nil {
			stopCandidate()
			p.Reject(t)
			continue
		}
		ack, err := sa.Encode(probe.Frame{Kind: probe.BindAck, Direction: 2, Run: run, Session: session, Content: challenge[:]}, max)
		if err != nil {
			stopCandidate()
			p.Reject(t)
			return nil, err
		}
		if _, err = t.Write(ack, write); err != nil {
			stopCandidate()
			p.Reject(t)
			return nil, err
		}
		if !stopCandidate() || ctx.Err() != nil {
			t.Close()
			return nil, ctx.Err()
		}
		t.SetReadDeadline(time.Time{})
		return t, nil
	}
	return nil, errors.New("authenticated bind candidate budget exhausted")
}

// checkBind requires exact run/session/direction/challenge in addition to authenticated structural validation.
func checkBind(f probe.Frame, kind, direction byte, run, session [16]byte, challenge [32]byte) error {
	if f.Kind != kind || f.Direction != direction || f.Run != run || f.Session != session || subtle.ConstantTimeCompare(f.Content, challenge[:]) != 1 {
		return errors.New("foreign or replayed data binding")
	}
	return nil
}

// markPlan probes all distinct physical-direction DSCP requests and restores the first phase setting.
func markPlan(t adapter.Transport, p profile.Plan, client bool) ([]platform.Marking, error) {
	seen := map[int]bool{}
	out := []platform.Marking{}
	first := 0
	for i, phase := range p.Phases {
		d := phase.Server
		if client {
			d = phase.Client
		}
		if i == 0 {
			first = d.DSCP
		}
		if seen[d.DSCP] {
			continue
		}
		seen[d.DSCP] = true
		m, err := t.Mark(d.DSCP)
		if err != nil {
			return nil, err
		}
		if d.DSCP != 0 && (m.Applied == nil || *m.Applied != d.DSCP) {
			return nil, errors.New("requested nonzero outer DSCP not applied")
		}
		out = append(out, m)
	}
	_, err := t.Mark(first)
	return out, err
}

// checkEffective validates public role settings without treating NAT bindings or API success as packet captures.
func checkEffective(e Effective, p profile.Plan, client bool) error {
	for _, binding := range []string{e.Local, e.Remote} {
		host, port, err := net.SplitHostPort(binding)
		n, pe := strconv.Atoi(port)
		ip := net.ParseIP(host)
		if err != nil || pe != nil || n < 1 || n > 65535 || ip == nil || (ip.To4() != nil) != (p.Family == "ipv4") {
			return errors.New("invalid effective binding/family")
		}
	}
	_, port, _ := net.SplitHostPort(e.Local)
	if !client && port != strconv.Itoa(p.DestinationPort) || client && p.SourcePort != "auto" && port != p.SourcePort {
		return errors.New("effective local port mismatch")
	}
	if client && e.Remote != net.JoinHostPort(net.ParseIP(p.ServerAddress).String(), strconv.Itoa(p.DestinationPort)) {
		return errors.New("effective client destination mismatch")
	}
	want := map[int]bool{}
	for _, phase := range p.Phases {
		d := phase.Server
		if client {
			d = phase.Client
		}
		want[d.DSCP] = true
	}
	if len(e.Markings) != len(want) {
		return errors.New("effective marking coverage mismatch")
	}
	for _, m := range e.Markings {
		if !want[m.Requested] || m.Observed != nil || m.Requested != 0 && (m.Applied == nil || *m.Applied != m.Requested) {
			return errors.New("invalid effective marking evidence")
		}
		delete(want, m.Requested)
	}
	return nil
}

// parse32 accepts only canonical lowercase fixed-length secret/digest strings from authenticated control.
func parse32(s string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 || hex.EncodeToString(b) != s {
		return out, errors.New("invalid 256-bit control value")
	}
	copy(out[:], b)
	return out, nil
}

// Ready verifies identity/plan/data readiness, optionally holds a heartbeat lease, then explicitly stops without measuring.
func Ready(ctx context.Context, store *Store, signer ssh.Signer, p profile.Plan, hold time.Duration) (ReadyResult, error) {
	ctx, stopClient := context.WithCancel(ctx)
	defer stopClient()
	var result ReadyResult
	factory, err := adapter.Default().Get(p.Adapter)
	if err == nil {
		err = factory.Validate(p)
	}
	if err != nil {
		return result, err
	}
	if hold < 0 || uint64(hold)/uint64(p.Limits.HeartbeatInterval)+6 > uint64(p.Limits.ControlRequests) {
		return result, errors.New("readiness hold exceeds control request budget")
	}
	client, err := store.Client()
	if err != nil {
		return result, err
	}
	public, _ := ParsePublic(client.Enrollment.PublicKey)
	if signer == nil || !bytes.Equal(public.Marshal(), signer.PublicKey().Marshal()) {
		return result, errors.New("client private key does not match initialized identity")
	}
	bundle, err := store.Pairing(p.PairingRef)
	if err != nil {
		return result, err
	}
	pin, err := PinnedHost(bundle.HostPublicKey, bundle.HostFingerprint)
	if err != nil {
		return result, err
	}
	hash, err := p.Hash()
	if err != nil {
		return result, err
	}
	hashText := hex.EncodeToString(hash[:])
	setup, cancel := context.WithTimeout(ctx, time.Duration(p.Limits.SetupTimeout))
	defer cancel()
	address := net.JoinHostPort(bundle.ControlAddress, strconv.Itoa(bundle.ControlPort))
	raw, err := (&net.Dialer{}).DialContext(setup, "tcp", address)
	if err != nil {
		return result, err
	}
	defer raw.Close()
	closeRaw := context.AfterFunc(ctx, func() { // Cancellation interrupts both SSH and data readiness instead of reconnecting.
		raw.Close()
	})
	defer closeRaw()
	deadline, _ := setup.Deadline()
	raw.SetDeadline(deadline)
	conn, channels, requests, err := ssh.NewClientConn(raw, address, &ssh.ClientConfig{User: client.Enrollment.ClientID, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: pin})
	if err != nil {
		return result, err
	}
	sshClient := ssh.NewClient(conn, channels, requests)
	defer sshClient.Close()
	go func() { // End held readiness promptly on authenticated SSH EOF, independently of the next heartbeat.
		conn.Wait()
		stopClient()
	}()
	ch, reqs, err := sshClient.OpenChannel(ChannelType, nil)
	if err != nil {
		return result, err
	}
	defer ch.Close()
	counted := &frameCounter{Channel: ch}
	channelCtx, closeChannel := context.WithCancel(ctx)
	defer closeChannel()
	go rejectRequests(channelCtx, reqs, p.Limits.ControlRequests, stopClient)
	var runID [16]byte
	if _, err = rand.Read(runID[:]); err != nil {
		return result, err
	}
	runText := hex.EncodeToString(runID[:])
	sessionText := ""
	counter := 0
	exchange := func(kind string, body any) (Envelope, error) { // Serialize one request/response with unique IDs and bounded owned-connection deadlines.
		counter++
		id := fmt.Sprintf("request-%d", counter)
		if counter > p.Limits.ControlRequests {
			return Envelope{}, errors.New("control request budget exhausted")
		}
		if err := Write(counted, message(kind, id, runText, sessionText, body)); err != nil {
			return Envelope{}, err
		}
		e, err := Read(counted)
		if err != nil {
			return e, err
		}
		if e.RequestID != id || e.RunID != runText || kind != "prepare" && e.SessionID != sessionText {
			return e, errors.New("response lifecycle mismatch")
		}
		if e.Type == "error" {
			var b errorBody
			if DecodeObject(e.Body, &b) != nil {
				return e, errors.New("invalid server error")
			}
			return e, errors.New(b.Message)
		}
		return e, nil
	}
	// Hello uses empty lifecycle IDs before a run is proposed.
	runText = ""
	e, err := exchange("hello", helloBody{[]int{1}, adapter.Default().Describe(), false})
	if err != nil {
		return result, err
	}
	var hello helloBody
	if e.Type != "hello" || DecodeObject(e.Body, &hello) != nil || len(hello.Contracts) != 1 || hello.Contracts[0] != 1 {
		return result, errors.New("unsupported control contract")
	}
	runText = hex.EncodeToString(runID[:])
	planJSON, _ := json.Marshal(p)
	e, err = exchange("prepare", prepareBody{planJSON, hashText})
	if err != nil {
		return result, err
	}
	var offer offerBody
	if e.Type != "prepared" || DecodeObject(e.Body, &offer) != nil || offer.Hash != hashText {
		return result, errors.New("invalid frozen-plan offer")
	}
	sessionText = e.SessionID
	session, err := probe.ID(sessionText)
	if err != nil {
		return result, err
	}
	ck, err := parse32(offer.ClientKey)
	if err != nil {
		return result, err
	}
	sk, err := parse32(offer.ServerKey)
	if err != nil {
		return result, err
	}
	challenge, err := parse32(offer.Challenge)
	if err != nil {
		return result, err
	}
	host, port, err := net.SplitHostPort(offer.Binding)
	if err != nil || net.ParseIP(host) == nil || port != strconv.Itoa(p.DestinationPort) || (net.ParseIP(host).To4() != nil) != (p.Family == "ipv4") {
		return result, errors.New("server prepared binding mismatch")
	}
	t, err := factory.Dial(setup, p)
	if err != nil {
		return result, err
	}
	defer t.Close()
	closeData := context.AfterFunc(ctx, func() { // Interrupt owned data IO when readiness is cancelled.
		t.Close()
	})
	defer closeData()
	marks, err := markPlan(t, p, true)
	if err != nil {
		return result, err
	}
	clientEffective := Effective{t.Local(), t.Remote(), marks}
	if err = checkEffective(clientEffective, p, true); err != nil {
		return result, err
	}
	ca, sa := probe.Auth{Plan: hash, Adapter: p.Adapter, Key: ck}, probe.Auth{Plan: hash, Adapter: p.Adapter, Key: sk}
	bind, err := ca.Encode(probe.Frame{Kind: probe.BindRequest, Direction: 1, Run: runID, Session: session, Content: challenge[:]}, factory.Describe().MaxFrame)
	if err != nil {
		return result, err
	}
	t.SetReadDeadline(deadline)
	if _, err = t.Write(bind, time.Duration(p.Limits.WriteTimeout)); err != nil {
		return result, err
	}
	ack, err := t.Read()
	if err != nil {
		return result, err
	}
	f, err := sa.Decode(ack, factory.Describe().MaxFrame)
	if err == nil {
		err = checkBind(f, probe.BindAck, 2, runID, session, challenge)
	}
	if err != nil {
		return result, err
	}
	t.SetReadDeadline(time.Time{})
	e, err = exchange("confirm", confirmBody{hashText, clientEffective})
	if err != nil {
		return result, err
	}
	if e.Type != "ready" || DecodeObject(e.Body, &result) != nil || result.Version != 1 || result.Status != "ready" || result.MeasurementStarted || result.PlanHash != hashText || result.RunID != runText || result.SessionID != sessionText {
		return result, errors.New("invalid readiness acknowledgment")
	}
	if err = checkEffective(result.Server, p, false); err != nil {
		return result, err
	}
	a, _ := json.Marshal(result.Client)
	b, _ := json.Marshal(clientEffective)
	if !bytes.Equal(a, b) {
		return result, errors.New("client effective settings changed by server")
	}
	if err = raw.SetDeadline(time.Time{}); err != nil {
		return result, err
	}
	end := time.Now().Add(hold)
	ticker := time.NewTicker(time.Duration(p.Limits.HeartbeatInterval))
	defer ticker.Stop()
	timer := time.NewTimer(hold)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-ticker.C:
			if time.Now().After(end) {
				continue
			}
			raw.SetDeadline(time.Now().Add(time.Duration(p.Limits.WriteTimeout)))
			e, err = exchange("heartbeat", hashBody{hashText})
			if err != nil {
				return result, err
			}
			if e.Type != "alive" {
				return result, errors.New("invalid heartbeat acknowledgment")
			}
			var acknowledged hashBody
			if DecodeObject(e.Body, &acknowledged) != nil || acknowledged.Hash != hashText {
				return result, errors.New("heartbeat plan acknowledgment mismatch")
			}
			if err = raw.SetDeadline(time.Time{}); err != nil {
				return result, err
			}
		case <-timer.C:
			raw.SetDeadline(time.Now().Add(time.Duration(p.Limits.ShutdownTimeout)))
			e, err = exchange("stop", hashBody{hashText})
			if err != nil {
				return result, err
			}
			if e.Type != "stopped" {
				return result, errors.New("stop not acknowledged")
			}
			var acknowledged hashBody
			if DecodeObject(e.Body, &acknowledged) != nil || acknowledged.Hash != hashText {
				return result, errors.New("stop plan acknowledgment mismatch")
			}
			result.Status = "readiness_checked"
			result.Control = ControlTraffic{"client application control frames through stopped acknowledgment; SSH overhead excluded", counter, counted.sent, counted.received}
			return result, nil
		}
	}
}
