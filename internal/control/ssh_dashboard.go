package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/profile"
)

// SSHConnectRequest contains an ephemeral password, never part of a measurement profile/report.
type SSHConnectRequest struct {
	Version   int    `json:"schema_version"`
	RequestID string `json:"request_id"`
	Address   string `json:"server_ip"`
	Port      int    `json:"ssh_port"`
	User      string `json:"ssh_username"`
	Password  string `json:"ssh_password"`
	Trust     string `json:"trust_fingerprint,omitempty"`
	Replace   bool   `json:"replace_host_key,omitempty"`
}

// SSHConnectionStatus contains public endpoint/trust/helper state only.
type SSHConnectionStatus struct {
	State       string        `json:"status"`
	Address     string        `json:"server_ip,omitempty"`
	Port        int           `json:"ssh_port,omitempty"`
	User        string        `json:"ssh_username,omitempty"`
	Fingerprint string        `json:"host_key_fingerprint,omitempty"`
	Previous    string        `json:"previous_fingerprint,omitempty"`
	PairingRef  string        `json:"pairing_ref,omitempty"`
	Policy      *ServerPolicy `json:"server_policy,omitempty"`
}

// sshHostPin lives in the application store, separately from system SSH and measurement pairing.
type sshHostPin struct {
	Version     int    `json:"schema_version"`
	Address     string `json:"ssh_address"`
	Key         string `json:"host_public_key"`
	Fingerprint string `json:"host_key_fingerprint"`
}

// sshDashboardSession owns the ordinary SSH connection and its disposable helper exec.
type sshDashboardSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	cancel  context.CancelFunc
	done    chan struct{}
	status  SSHConnectionStatus
}

// SSHDashboard serializes Connect/Disconnect and supplies only control tunnels to finite runs.
type SSHDashboard struct {
	store   *Store
	signer  ssh.Signer
	mu      sync.Mutex
	gate    sync.Mutex
	current *sshDashboardSession
}

// NewSSHDashboard binds password-based setup to this client's application-owned identities.
func NewSSHDashboard(store *Store, signers ...ssh.Signer) *SSHDashboard {
	d := &SSHDashboard{store: store}
	if len(signers) > 0 {
		d.signer = signers[0]
	}
	return d
}

// Status takes a secret-free immutable view; finished helper/SSH sessions are visibly disconnected.
func (d *SSHDashboard) Status() SSHConnectionStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.current == nil {
		return SSHConnectionStatus{State: "disconnected"}
	}
	select {
	case <-d.current.done:
		return SSHConnectionStatus{State: "disconnected"}
	default:
	}
	v := d.current.status
	if v.Policy != nil {
		copy := *v.Policy
		v.Policy = &copy
	}
	return v
}

// Close ends only the owned helper and normal SSH session, then joins its lifecycle observer.
// Callers reject disconnect/reconnect while measurement is active rather than changing its target.
func (d *SSHDashboard) Close() {
	d.gate.Lock()
	defer d.gate.Unlock()
	d.mu.Lock()
	current := d.current
	d.current = nil
	d.mu.Unlock()
	if current != nil {
		current.cancel()
		<-current.done
	}
}

// hostPath hashes the explicit endpoint to prevent user-controlled paths in the private pin store.
func (d *SSHDashboard) hostPath(address string, create bool) (string, error) {
	dir, err := d.store.ownedDir("ssh-hosts", create)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(address))
	return filepath.Join(dir, hex.EncodeToString(hash[:])+".json"), nil
}

// Connect verifies host identity before sending the password, then starts the installed Linux helper.
// First/changed identities return trust_required; only the displayed fingerprint can be accepted.
func (d *SSHDashboard) Connect(ctx context.Context, q SSHConnectRequest) (SSHConnectionStatus, error) {
	d.gate.Lock()
	defer d.gate.Unlock()
	out := SSHConnectionStatus{State: "disconnected", Address: q.Address, Port: q.Port, User: q.User}
	if net.ParseIP(q.Address) == nil || net.ParseIP(q.Address).IsUnspecified() || q.Port < 1 || q.Port > 65535 || !profile.SafeID(q.User) || len(q.Password) < 1 || len(q.Password) > 1024 {
		return out, errors.New("server IP, SSH port, username and password are required")
	}
	d.mu.Lock()
	currentPrevious := d.current
	d.mu.Unlock()
	if currentPrevious != nil {
		select {
		case <-currentPrevious.done:
		default:
			return out, errors.New("disconnect the existing server first")
		}
	}
	address := net.JoinHostPort(q.Address, strconv.Itoa(q.Port))
	path, err := d.hostPath(address, false)
	var previous sshHostPin
	if err == nil {
		err = readObject(path, &previous)
	}
	if err != nil && !os.IsNotExist(err) {
		return out, errors.New("saved SSH host identity failed validation")
	}
	if err == nil {
		key, e := ParsePublic(previous.Key)
		if e != nil || previous.Version != 1 || previous.Address != address || ssh.FingerprintSHA256(key) != previous.Fingerprint {
			return out, errors.New("saved SSH host identity failed validation")
		}
	}
	var observed sshHostPin
	trustNeeded := false
	setup, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	raw, err := (&net.Dialer{}).DialContext(setup, "tcp", address)
	if err != nil {
		return out, errors.New("SSH endpoint could not be reached")
	}
	defer func() { // Unsuccessful setup closes the owned socket; successful sessions own it through cancellation.
		if out.State != "connected" {
			raw.Close()
		}
	}()
	interrupt := context.AfterFunc(setup, func() { /* Bound handshake/helper startup independently of HTTP request lifetime. */ raw.Close() })
	defer interrupt()
	deadline, _ := setup.Deadline()
	raw.SetDeadline(deadline)
	config := &ssh.ClientConfig{User: q.User, Auth: []ssh.AuthMethod{ssh.Password(q.Password)}, HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error { // Pin before authentication; a refused/changed identity receives no password.
		observed = sshHostPin{1, address, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))), ssh.FingerprintSHA256(key)}
		if previous.Key == observed.Key && previous.Key != "" {
			return nil
		}
		if q.Trust == observed.Fingerprint && (previous.Key == "" || q.Replace) {
			return nil
		}
		trustNeeded = true
		return errors.New("SSH host identity needs explicit trust")
	}}
	conn, channels, requests, err := ssh.NewClientConn(raw, address, config)
	q.Password = ""
	if err != nil {
		if trustNeeded {
			out.State = "trust_required"
			out.Fingerprint = observed.Fingerprint
			out.Previous = previous.Fingerprint
			return out, nil
		}
		return out, errors.New("SSH authentication or handshake failed")
	}
	client := ssh.NewClient(conn, channels, requests)
	success := false
	defer func() { // Failed helper/trust publication never leaves an authenticated process running.
		if !success {
			client.Close()
		}
	}()
	identity, err := d.store.Client()
	if os.IsNotExist(err) {
		_, err = d.store.InitClient("dashboard-client", "")
		if err == nil {
			identity, err = d.store.Client()
		}
	}
	if err != nil {
		return out, errors.New("local managed client identity unavailable")
	}
	session, err := client.NewSession()
	if err != nil {
		return out, errors.New("SSH exec session unavailable")
	}
	session.Stderr = io.Discard
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return out, errors.New("SSH helper input unavailable")
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return out, errors.New("SSH helper output unavailable")
	}
	if err = session.Start("exec sneaky-port dashboard-server"); err != nil {
		session.Close()
		return out, errors.New("matching Linux sneaky-port must be installed in the SSH account's PATH")
	}
	if err = json.NewEncoder(stdin).Encode(DashboardServerRequest{1, q.Address, identity.Enrollment}); err != nil {
		session.Close()
		return out, errors.New("SSH helper enrollment failed")
	}
	var hello DashboardServerReply
	var helloJSON json.RawMessage
	if err = json.NewDecoder(io.LimitReader(stdout, FrameMax)).Decode(&helloJSON); err != nil || DecodeObject(helloJSON, &hello) != nil || hello.Version != 1 || hello.Bundle.Validate() != nil || hello.Policy.Validate() != nil || hello.Bundle.ControlAddress != q.Address || hello.Policy.ControlListen != net.JoinHostPort("127.0.0.1", strconv.Itoa(hello.Bundle.ControlPort)) {
		session.Close()
		return out, errors.New("matching Linux sneaky-port helper did not return a valid startup response")
	}
	if err = d.verifyHelper(setup, client, hello.Bundle, identity); err != nil {
		session.Close()
		return out, err
	}
	path, err = d.hostPath(address, true)
	if err != nil {
		session.Close()
		return out, errors.New("SSH host trust could not be saved")
	}
	lock, err := d.store.lock()
	if err != nil {
		session.Close()
		return out, errors.New("local trust store is busy")
	}
	var latest sshHostPin
	latestErr := readObject(path, &latest)
	if latestErr != nil && !os.IsNotExist(latestErr) || latestErr == nil && latest.Key != previous.Key && latest.Key != observed.Key {
		err = errors.New("SSH host identity changed during connection")
	} else {
		err = d.store.writeAtomic(path, observed)
	}
	d.store.unlock(lock)
	if err != nil {
		session.Close()
		return out, errors.New("SSH host trust could not be saved")
	}
	hash := sha256.Sum256([]byte(address + "/" + q.User))
	ref := "ssh-" + hex.EncodeToString(hash[:12])
	if err = d.store.ImportPairing(ref, hello.Bundle, true); err != nil {
		session.Close()
		return out, errors.New("owned helper pairing could not be saved")
	}
	if err = raw.SetDeadline(time.Time{}); err != nil {
		session.Close()
		return out, errors.New("SSH startup deadline could not be cleared")
	}
	owned, cancel := context.WithCancel(ctx)
	out = SSHConnectionStatus{State: "connected", Address: q.Address, Port: q.Port, User: q.User, Fingerprint: observed.Fingerprint, PairingRef: ref, Policy: &hello.Policy}
	current := &sshDashboardSession{client: client, session: session, stdin: stdin, cancel: cancel, done: make(chan struct{}), status: out}
	d.mu.Lock()
	d.current = current
	d.mu.Unlock()
	go func() { // Helper exit, SSH loss and API shutdown all close the same owned session and join observers.
		defer close(current.done)
		watchDone := make(chan struct{})
		finished := make(chan struct{})
		watch := context.AfterFunc(owned, func() { /* Give the helper stdin EOF before a five-second forced transport close. */
			defer close(watchDone)
			stdin.Close()
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				session.Close()
				client.Close()
				raw.Close()
			}
		})
		session.Wait()
		close(finished)
		cancel()
		stopped := watch()
		stdin.Close()
		session.Close()
		client.Close()
		raw.Close()
		if !stopped {
			<-watchDone
		}
	}()
	success = true
	return out, nil
}

// Run binds a frozen plan to the connected server and reuses exact keyed plan/readiness negotiation.
// Only control/calibration/reports use SSH forwarding; UDP/TCP data sockets are still direct.
func (d *SSHDashboard) Run(ctx context.Context, p profile.Plan) (RunResult, error) {
	d.mu.Lock()
	current := d.current
	d.mu.Unlock()
	if current == nil || d.Status().State != "connected" || p.PairingRef != current.status.PairingRef || p.ServerAddress != current.status.Address {
		return RunResult{}, errors.New("connect the workload's SSH server before running")
	}
	identity, err := d.store.Client()
	if err != nil {
		return RunResult{}, err
	}
	key, err := d.clientSigner(identity)
	if err != nil {
		return RunResult{}, errors.New("local managed client key unavailable")
	}
	_, port, _ := net.SplitHostPort(current.status.Policy.ControlListen)
	dial := func(ctx context.Context, network, address string) (net.Conn, error) { // Limit forwarding to this authenticated helper's private control listener.
		bundle, err := d.store.Pairing(p.PairingRef)
		if err != nil || address != net.JoinHostPort(current.status.Address, strconv.Itoa(bundle.ControlPort)) {
			return nil, errors.New("control target changed")
		}
		remote, err := current.client.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
		if err != nil {
			return nil, err
		}
		return bridgeControl(remote), nil
	}
	return Run(WithControlDialer(ctx, dial), d.store, key, p)
}

// clientSigner reuses an already-unlocked launch key or loads the managed generated identity locally.
func (d *SSHDashboard) clientSigner(identity ClientIdentity) (ssh.Signer, error) {
	if d.signer != nil {
		return d.signer, nil
	}
	return LoadKey(identity.KeyRef, nil)
}

// verifyHelper confirms forwarded keyed control/contract discovery before marking normal SSH connected.
// No prepare/start/data channel is used here; the edited plan is negotiated only by Run Test.
func (d *SSHDashboard) verifyHelper(ctx context.Context, outer *ssh.Client, bundle Bundle, identity ClientIdentity) error {
	key, err := d.clientSigner(identity)
	if err != nil {
		return errors.New("local managed client key unavailable")
	}
	remote, err := outer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(bundle.ControlPort)))
	if err != nil {
		return errors.New("normal SSH must allow forwarding to the owned measurement helper")
	}
	raw := bridgeControl(remote)
	defer raw.Close()
	deadline, _ := ctx.Deadline()
	raw.SetDeadline(deadline)
	pin, err := PinnedHost(bundle.HostPublicKey, bundle.HostFingerprint)
	if err != nil {
		return errors.New("helper identity invalid")
	}
	conn, channels, requests, err := ssh.NewClientConn(raw, "dashboard-helper", &ssh.ClientConfig{User: identity.Enrollment.ClientID, Auth: []ssh.AuthMethod{ssh.PublicKeys(key)}, HostKeyCallback: pin})
	if err != nil {
		return errors.New("owned helper authentication failed")
	}
	client := ssh.NewClient(conn, channels, requests)
	defer client.Close()
	ch, reqs, err := client.OpenChannel(ChannelType, nil)
	if err != nil {
		return errors.New("owned helper control channel unavailable")
	}
	defer ch.Close()
	go ssh.DiscardRequests(reqs)
	if err = Write(ch, message("hello", "dashboard-discovery", "", "", helloBody{[]int{1}, d.store.AdapterRegistry().Describe(), false})); err != nil {
		return errors.New("owned helper discovery failed")
	}
	e, err := Read(ch)
	var hello helloBody
	if err != nil || e.Type != "hello" || e.RequestID != "dashboard-discovery" || e.RunID != "" || e.SessionID != "" || DecodeObject(e.Body, &hello) != nil || len(hello.Contracts) != 1 || hello.Contracts[0] != 1 || !hello.Measurement {
		return errors.New("owned helper measurement contract unavailable")
	}
	return nil
}

// controlBridge adapts SSH's channel to native deadline-aware local pipes and owns both copying loops.
type controlBridge struct {
	net.Conn
	remote net.Conn
	peer   net.Conn
	once   sync.Once
	wg     sync.WaitGroup
}

// bridgeControl adds real deadlines without forwarding measurement data or buffering unbounded bytes.
func bridgeControl(remote net.Conn) net.Conn {
	local, peer := net.Pipe()
	b := &controlBridge{Conn: local, remote: remote, peer: peer}
	b.wg.Add(2)
	go func() { // Native pipe backpressure bounds outbound control bytes.
		defer b.wg.Done()
		io.Copy(remote, peer)
		b.stop()
	}()
	go func() { // Close both peers when inbound SSH control ends.
		defer b.wg.Done()
		io.Copy(peer, remote)
		b.stop()
	}()
	return b
}

// stop closes all bridge endpoints once; copying goroutines never wait for themselves.
func (b *controlBridge) stop() {
	b.once.Do(func() { /* Wake every owned read/write after timeout, EOF or cancellation. */
		b.Conn.Close()
		b.peer.Close()
		b.remote.Close()
	})
}

// Close joins both copy loops after closing, preserving the finite lifecycle's cleanup contract.
func (b *controlBridge) Close() error { b.stop(); b.wg.Wait(); return nil }
