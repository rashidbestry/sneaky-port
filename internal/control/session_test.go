package control

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/adapter"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// testRig owns an isolated embedded SSH server and endpoint-local identities.
type testRig struct {
	server, client *Store
	signer         ssh.Signer
	bundle         Bundle
	plan           profile.Plan
	stop           context.CancelFunc
	done           chan error
}

// newRig binds loopback-only ephemeral test ports; these fixtures are not product defaults or live-network evidence.
func newRig(t *testing.T, kind string) *testRig {
	t.Helper()
	family, ip, suffix := "ipv4", "127.0.0.1", "4"
	if strings.HasSuffix(kind, "6") {
		kind = strings.TrimSuffix(kind, "6")
		family, ip, suffix = "ipv6", "::1", "6"
	}
	server, err := OpenStore(filepath.Join(t.TempDir(), "server"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := OpenStore(filepath.Join(t.TempDir(), "client"))
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp"+suffix, net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Release the fixture listener if initialization fails.
		ln.Close()
	})
	port := ln.Addr().(*net.TCPAddr).Port
	policy := ServerPolicy{1, ln.Addr().String(), ip, 1, 65535, 2e9, 2e9, 20e6, 8, 3, 100, 16, 1 << 20, 1000, 1 << 20, 2 << 20}
	bundle, err := server.InitServer("server", ip, port, policy, "")
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := client.InitClient("client", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = server.Enroll(enrollment); err != nil {
		t.Fatal(err)
	}
	if err = client.ImportPairing("fixture", bundle, false); err != nil {
		t.Fatal(err)
	}
	serverID, _ := server.Server()
	host, err := LoadKey(serverID.KeyRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientID, _ := client.Client()
	signer, err := LoadKey(clientID.KeyRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(server, host)
	if err != nil {
		t.Fatal(err)
	}
	p := testutil.Plan()
	p.Adapter = kind
	p.Family = family
	p.ServerAddress = ip
	if kind == "tcp" {
		p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
		tmp, err := net.Listen("tcp"+suffix, net.JoinHostPort(ip, "0"))
		if err != nil {
			t.Fatal(err)
		}
		p.DestinationPort = tmp.Addr().(*net.TCPAddr).Port
		tmp.Close()
	} else {
		tmp, err := net.ListenPacket("udp"+suffix, net.JoinHostPort(ip, "0"))
		if err != nil {
			t.Fatal(err)
		}
		p.DestinationPort = tmp.LocalAddr().(*net.UDPAddr).Port
		tmp.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &testRig{server, client, signer, bundle, p, cancel, make(chan error, 1)}
	go func() { // Report fixture shutdown without retaining a background server after the test.
		r.done <- s.Serve(ctx, ln)
	}()
	t.Cleanup(func() { // Stop and join every fixture server within its explicit setup bound.
		cancel()
		select {
		case err := <-r.done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server cleanup exceeded test bound")
		}
	})
	return r
}

// sshClient opens the authenticated fixture connection with the imported public host pin.
func (r *testRig) sshClient(t *testing.T, signer ssh.Signer) *ssh.Client {
	t.Helper()
	pin, err := PinnedHost(r.bundle.HostPublicKey, r.bundle.HostFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ssh.Dial("tcp", net.JoinHostPort(r.bundle.ControlAddress, strconvPort(r.bundle.ControlPort)), &ssh.ClientConfig{User: "client", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: pin, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Close this owned fixture SSH connection.
		c.Close()
	})
	return c
}

// strconvPort returns the decimal control port without treating service names as supported input.
func strconvPort(n int) string { return fmt.Sprintf("%d", n) }

// TestReadyRealCarriers proves identity, exact plan, authenticated binding, effective settings and clean stop for UDP/TCP.
func TestReadyRealCarriers(t *testing.T) {
	for _, kind := range []string{"udp", "tcp", "udp6", "tcp6"} {
		t.Run(kind, func(t *testing.T) { // Exercise both production factories through the same coordinator.
			r := newRig(t, kind)
			for i := 0; i < 2; i++ {
				result, err := Ready(context.Background(), r.client, r.signer, r.plan, 150*time.Millisecond)
				if err != nil {
					t.Fatal(err)
				}
				hash, _ := r.plan.Hash()
				if result.Status != "readiness_checked" || result.MeasurementStarted || result.PlanHash != hex.EncodeToString(hash[:]) || result.RunID == result.SessionID || result.Control.Requests < 4 || result.Control.Sent == 0 || result.Control.Received == 0 {
					t.Fatalf("bad public readiness result: %+v", result)
				}
				data, _ := json.Marshal(result)
				for _, secret := range []string{"client_key", "server_key", "challenge", "PRIVATE KEY"} {
					if bytes.Contains(data, []byte(secret)) {
						t.Fatalf("public result contains %s", secret)
					}
				}
			}
		})
	}
}

// TestReadinessClearsOperationDeadlines keeps a valid held session alive when setup/write budgets are shorter than heartbeat gaps.
func TestReadinessClearsOperationDeadlines(t *testing.T) {
	r := newRig(t, "udp")
	r.plan.Limits.SetupTimeout = 400e6
	r.plan.Limits.WriteTimeout = 200e6
	r.plan.Limits.HeartbeatInterval = 700e6
	if _, err := Ready(context.Background(), r.client, r.signer, r.plan, 1500*time.Millisecond); err != nil {
		t.Fatalf("completed operation deadline leaked into held readiness: %v", err)
	}
}

// TestTrustAndChannelRestrictions rejects wrong identities and refuses shell/forwarding/global SSH requests.
func TestTrustAndChannelRestrictions(t *testing.T) {
	r := newRig(t, "udp")
	other, err := OpenStore(filepath.Join(t.TempDir(), "other"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = other.InitClient("other", "")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := other.Client()
	wrong, _ := LoadKey(id.KeyRef, nil)
	pin, _ := PinnedHost(r.bundle.HostPublicKey, r.bundle.HostFingerprint)
	address := net.JoinHostPort(r.bundle.ControlAddress, strconvPort(r.bundle.ControlPort))
	_, err = ssh.Dial("tcp", address, &ssh.ClientConfig{User: "client", Auth: []ssh.AuthMethod{ssh.PublicKeys(wrong)}, HostKeyCallback: pin, Timeout: time.Second})
	if err == nil {
		t.Fatal("unenrolled key accepted")
	}
	wrongBundle := r.bundle
	wrongBundle.HostPublicKey = publicSigner(wrong).PublicKey
	wrongBundle.HostFingerprint = publicSigner(wrong).Fingerprint
	if err = r.client.ImportPairing("fixture", wrongBundle, true); err != nil {
		t.Fatal(err)
	}
	if _, err = Ready(context.Background(), r.client, r.signer, r.plan, 0); err == nil {
		t.Fatal("wrong pinned server accepted")
	}
	if err = r.client.ImportPairing("fixture", r.bundle, true); err != nil {
		t.Fatal(err)
	}
	c := r.sshClient(t, r.signer)
	ch, requests, err := c.OpenChannel(ChannelType, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	go ssh.DiscardRequests(requests)
	if ok, _, err := c.SendRequest("tcpip-forward", true, nil); err != nil || ok {
		t.Fatalf("global forwarding result %v %v", ok, err)
	}
	if ok, err := ch.SendRequest("exec", true, nil); err != nil || ok {
		t.Fatalf("exec result %v %v", ok, err)
	}
	for _, kind := range []string{"session", "direct-tcpip", "sneaky-port/calibration/v1", "sneaky-port/report/v1"} {
		if extra, _, err := c.OpenChannel(kind, nil); err == nil {
			extra.Close()
			t.Fatalf("unsupported channel %s accepted", kind)
		}
	}
}

// prepareFixture negotiates the public contract and optionally submits a mismatched plan hash.
func prepareFixture(t *testing.T, r *testRig, ch ssh.Channel, hashOverride string) Envelope {
	t.Helper()
	if err := Write(ch, message("hello", "hello", "", "", helloBody{[]int{1}, adapter.Default().Describe(), false})); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(ch); err != nil {
		t.Fatal(err)
	}
	hash, _ := r.plan.Hash()
	h := hex.EncodeToString(hash[:])
	if hashOverride != "" {
		h = hashOverride
	}
	b, _ := json.Marshal(r.plan)
	if err := Write(ch, message("prepare", "prepare", strings.Repeat("01", 16), "", prepareBody{b, h})); err != nil {
		t.Fatal(err)
	}
	e, err := Read(ch)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// TestPlanRejectionAndDisconnect proves hash/ceiling errors happen before readiness and control EOF releases preparation.
func TestPlanRejectionAndDisconnect(t *testing.T) {
	r := newRig(t, "udp")
	c := r.sshClient(t, r.signer)
	ch, reqs, err := c.OpenChannel(ChannelType, nil)
	if err != nil {
		t.Fatal(err)
	}
	go ssh.DiscardRequests(reqs)
	if e := prepareFixture(t, r, ch, strings.Repeat("00", 32)); e.Type != "error" {
		t.Fatal("hash mismatch accepted")
	}
	ch.Close()
	c.Close()
	p := r.plan
	p.Limits.QueueBytes = 2 << 20
	if _, err = Ready(context.Background(), r.client, r.signer, p, 0); err == nil {
		t.Fatal("server queue ceiling ignored")
	}
	c = r.sshClient(t, r.signer)
	ch, reqs, err = c.OpenChannel(ChannelType, nil)
	if err != nil {
		t.Fatal(err)
	}
	go ssh.DiscardRequests(reqs)
	if e := prepareFixture(t, r, ch, ""); e.Type != "prepared" {
		t.Fatalf("not prepared: %s", e.Body)
	}
	if _, err = Ready(context.Background(), r.client, r.signer, r.plan, 0); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("concurrent preparation accepted: %v", err)
	}
	c.Close()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err = Ready(context.Background(), r.client, r.signer, r.plan, 0); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("disconnected preparation not released: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRevocationAndExpiry applies persistent authorization changes to already authenticated SSH connections.
func TestRevocationAndExpiry(t *testing.T) {
	for _, kind := range []string{"disable", "expiry", "corrupt"} {
		t.Run(kind, func(t *testing.T) { // Current store state must override successful earlier authentication.
			r := newRig(t, "udp")
			c := r.sshClient(t, r.signer)
			done := make(chan error, 1)
			go func() { // Wait for server-side revocation to close this fixture connection.
				done <- c.Wait()
			}()
			switch kind {
			case "disable":
				if err := r.server.Disable("client"); err != nil {
					t.Fatal(err)
				}
			case "expiry":
				id, _ := r.client.Client()
				expiry := time.Now().Add(30 * time.Millisecond)
				id.Enrollment.ExpiresAt = &expiry
				if err := r.server.Enroll(id.Enrollment); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(filepath.Join(r.server.Root, "authorization.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("authorization change did not revoke active SSH")
			}
		})
	}
}

// TestBindRejectsForeignFirstUDP proves an unauthenticated first tuple cannot pin the listener or elicit an acknowledgement.
func TestBindRejectsForeignFirstUDP(t *testing.T) {
	r := newRig(t, "udp")
	factory, _ := adapter.Default().Get("udp")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	prepared, err := factory.Prepare(ctx, r.plan)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	var run, session [16]byte
	run[0] = 1
	session[0] = 2
	var challenge [32]byte
	challenge[0] = 3
	ca := probe.Auth{Adapter: "udp"}
	ca.Key[0] = 4
	sa := ca
	sa.Key[0] = 5
	done := make(chan error, 1)
	go func() { // Authenticate candidates through the real UDP prepared transport.
		t, err := acceptBind(ctx, prepared, ca, sa, run, session, challenge, probe.UDPMax, 8, time.Second)
		if err == nil {
			t.Close()
		}
		done <- err
	}()
	foreign, err := net.Dial("udp4", prepared.Address())
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	foreign.Write([]byte("unauthenticated"))
	foreign.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
	var b [256]byte
	if _, err = foreign.Read(b[:]); err == nil {
		t.Fatal("invalid candidate received amplification")
	}
	legit, err := factory.Dial(ctx, r.plan)
	if err != nil {
		t.Fatal(err)
	}
	defer legit.Close()
	frame, _ := ca.Encode(probe.Frame{Kind: probe.BindRequest, Direction: 1, Run: run, Session: session, Content: challenge[:]}, probe.UDPMax)
	legit.Write(frame, time.Second)
	legit.SetReadDeadline(time.Now().Add(time.Second))
	ack, err := legit.Read()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := sa.Decode(ack, probe.UDPMax)
	if err != nil || checkBind(decoded, probe.BindAck, 2, run, session, challenge) != nil {
		t.Fatal("valid binding not acknowledged")
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	decoded.Session[0]++
	if checkBind(decoded, probe.BindAck, 2, run, session, challenge) == nil {
		t.Fatal("foreign/replayed session accepted")
	}
}

// finishFixtureBind completes data readiness while leaving control liveness under explicit test control.
func finishFixtureBind(t *testing.T, r *testRig, ch ssh.Channel, e Envelope) adapter.Transport {
	t.Helper()
	var offer offerBody
	if err := DecodeObject(e.Body, &offer); err != nil {
		t.Fatal(err)
	}
	run, _ := probe.ID(e.RunID)
	session, _ := probe.ID(e.SessionID)
	hash, _ := r.plan.Hash()
	ck, _ := parse32(offer.ClientKey)
	sk, _ := parse32(offer.ServerKey)
	challenge, _ := parse32(offer.Challenge)
	factory, _ := adapter.Default().Get(r.plan.Adapter)
	data, err := factory.Dial(context.Background(), r.plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Release the fixture's owned authenticated data socket.
		data.Close()
	})
	marks, err := markPlan(data, r.plan, true)
	if err != nil {
		t.Fatal(err)
	}
	ca, sa := probe.Auth{Plan: hash, Adapter: r.plan.Adapter, Key: ck}, probe.Auth{Plan: hash, Adapter: r.plan.Adapter, Key: sk}
	request, _ := ca.Encode(probe.Frame{Kind: probe.BindRequest, Direction: 1, Run: run, Session: session, Content: challenge[:]}, factory.Describe().MaxFrame)
	data.Write(request, time.Second)
	data.SetReadDeadline(time.Now().Add(time.Second))
	b, err := data.Read()
	if err != nil {
		t.Fatal(err)
	}
	f, err := sa.Decode(b, factory.Describe().MaxFrame)
	if err != nil || checkBind(f, probe.BindAck, 2, run, session, challenge) != nil {
		t.Fatal("bind acknowledgment mismatch")
	}
	if err = Write(ch, message("confirm", "confirm", e.RunID, e.SessionID, confirmBody{offer.Hash, Effective{data.Local(), data.Remote(), marks}})); err != nil {
		t.Fatal(err)
	}
	response, err := Read(ch)
	if err != nil || response.Type != "ready" {
		t.Fatalf("ready fixture: %v %s", err, response.Body)
	}
	return data
}

// TestLeaseAndInvalidLifecycle releases listeners on missing heartbeats or replayed/mismatched control identities.
func TestLeaseAndInvalidLifecycle(t *testing.T) {
	for _, cause := range []string{"lease", "request_replay", "session_mismatch", "cancel"} {
		t.Run(cause, func(t *testing.T) { // Every termination must release the single-session reservation and data port.
			r := newRig(t, "tcp")
			r.plan.Limits.HeartbeatInterval = 20e6
			r.plan.Limits.Lease = 100e6
			c := r.sshClient(t, r.signer)
			ch, reqs, err := c.OpenChannel(ChannelType, nil)
			if err != nil {
				t.Fatal(err)
			}
			go ssh.DiscardRequests(reqs)
			offer := prepareFixture(t, r, ch, "")
			finishFixtureBind(t, r, ch, offer)
			switch cause {
			case "request_replay":
				Write(ch, message("heartbeat", "confirm", offer.RunID, offer.SessionID, hashBody{}))
			case "session_mismatch":
				Write(ch, message("heartbeat", "new", offer.RunID, strings.Repeat("00", 16), hashBody{}))
			case "cancel":
				c.Close()
			}
			done := make(chan error, 1)
			go func() { // Await lease/lifecycle cleanup without blocking the test indefinitely.
				done <- c.Wait()
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("lease/lifecycle did not close connection")
			}
			if _, err = Ready(context.Background(), r.client, r.signer, r.plan, 0); err != nil {
				t.Fatalf("listener/session not released: %v", err)
			}
		})
	}
}
