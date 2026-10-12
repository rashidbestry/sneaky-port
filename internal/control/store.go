package control

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"sneaky-port/internal/adapter"
	"sneaky-port/internal/profile"
)

// Store keeps application identities and public trust records away from system SSH files.
type Store struct {
	Root     string
	Adapters *adapter.Registry // Assigned at startup only; nil uses production registration.
}

// AdapterRegistry resolves the store-wide startup registry for configuration, runs and saved replay.
// Register factories before serving; runtime registration is unsupported.
func (s *Store) AdapterRegistry() *adapter.Registry {
	if s.Adapters != nil {
		return s.Adapters
	}
	return adapter.Default()
}

// Bundle contains only the public server identity and explicitly advertised control endpoint.
type Bundle struct {
	Version         int    `json:"schema_version"`
	ServerID        string `json:"server_id"`
	ControlAddress  string `json:"control_address"`
	ControlPort     int    `json:"control_port"`
	HostPublicKey   string `json:"host_public_key"`
	HostFingerprint string `json:"host_key_fingerprint"`
	Contracts       []int  `json:"supported_contract_versions"`
}

// ServerPolicy declares explicit service and preparation ceilings before accepting client plans.
type ServerPolicy struct {
	Version            int             `json:"schema_version"`
	ControlListen      string          `json:"control_listen"`
	DataBindIP         string          `json:"data_bind_ip"`
	PortMin            int             `json:"data_port_min"`
	PortMax            int             `json:"data_port_max"`
	SetupTimeout       profile.Decimal `json:"setup_timeout_ns"`
	ShutdownTimeout    profile.Decimal `json:"shutdown_timeout_ns"`
	AuthPoll           profile.Decimal `json:"authorization_poll_ns"`
	MaxConnections     int             `json:"max_connections"`
	MaxAuthTries       int             `json:"max_auth_tries"`
	MaxControlRequests int             `json:"max_control_requests"`
	MaxBindAttempts    int             `json:"max_bind_attempts"`
	MaxQueueBytes      profile.Decimal `json:"max_queue_bytes"`
	MaxLedgerMessages  int             `json:"max_ledger_messages"`
	MaxEventBytes      profile.Decimal `json:"max_event_bytes"`
	MaxFileBytes       profile.Decimal `json:"max_file_bytes"`
}

// ServerIdentity stores key references, public pairing and immutable initialization policy.
type ServerIdentity struct {
	Version int          `json:"schema_version"`
	KeyRef  string       `json:"key_ref"`
	Bundle  Bundle       `json:"bundle"`
	Policy  ServerPolicy `json:"policy"`
}

// ClientIdentity stores the local client key reference and public enrollment information.
type ClientIdentity struct {
	Version    int        `json:"schema_version"`
	KeyRef     string     `json:"key_ref"`
	Enrollment Enrollment `json:"enrollment"`
}

// Authorization holds a versioned, revisioned public-key allowlist with explicit revocation.
type Authorization struct {
	Version  int             `json:"schema_version"`
	Revision profile.Decimal `json:"revision"`
	Clients  []Enrollment    `json:"clients"`
}

// OpenStore creates owned directories and rejects symlink directory substitutions.
func OpenStore(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("explicit application data directory required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{absolute, filepath.Join(absolute, "keys"), filepath.Join(absolute, "pairings")} {
		if err = os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
		info, e := os.Lstat(dir)
		if e != nil {
			return nil, e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("application directories must be real directories")
		}
		if err = restrictDirectory(dir); err != nil {
			return nil, err
		}
	}
	return &Store{Root: absolute}, nil
}

// ParsePublic accepts one ordinary SSH key without authorized_keys options or certificates.
func ParsePublic(text string) (ssh.PublicKey, error) {
	key, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(text))
	if err != nil || len(options) > 0 || strings.TrimSpace(string(rest)) != "" {
		return nil, errors.New("one ordinary public key without SSH options required")
	}
	if _, cert := key.(*ssh.Certificate); cert {
		return nil, errors.New("SSH certificates are not supported by this enrollment contract")
	}
	return key, nil
}

// Validate checks imported public identity before any connection or persistent trust update.
func (b Bundle) Validate() error {
	if b.Version != 1 || !profile.SafeID(b.ServerID) || net.ParseIP(b.ControlAddress) == nil || net.ParseIP(b.ControlAddress).IsUnspecified() || b.ControlPort < 1 || b.ControlPort > 65535 || len(b.Contracts) != 1 || b.Contracts[0] != 1 {
		return errors.New("invalid public pairing bundle; explicit control IP and contract 1 required")
	}
	key, err := ParsePublic(b.HostPublicKey)
	if err != nil {
		return err
	}
	if ssh.FingerprintSHA256(key) != b.HostFingerprint {
		return errors.New("public bundle fingerprint mismatch")
	}
	return nil
}

// Validate checks explicit server ceilings without silently tuning or clamping a plan.
func (p ServerPolicy) Validate() error {
	host, port, err := net.SplitHostPort(p.ControlListen)
	if err != nil || net.ParseIP(host) == nil {
		return errors.New("control_listen requires explicit IP:port")
	}
	parsed, err := strconv.Atoi(port)
	if err != nil || parsed < 1 || parsed > 65535 {
		return errors.New("invalid control listen port")
	}
	if p.Version != 1 || net.ParseIP(p.DataBindIP) == nil || p.PortMin < 1 || p.PortMax < p.PortMin || p.PortMax > 65535 {
		return errors.New("invalid server binding/port policy")
	}
	for _, v := range []profile.Decimal{p.SetupTimeout, p.ShutdownTimeout, p.AuthPoll} {
		if v == 0 || v > 1<<63-1 {
			return errors.New("explicit positive server timeout/poll durations required")
		}
	}
	if p.AuthPoll > p.SetupTimeout || p.MaxConnections < 1 || p.MaxConnections > 128 || p.MaxAuthTries < 1 || p.MaxAuthTries > 32 || p.MaxControlRequests < 5 || p.MaxControlRequests > 1<<20 || p.MaxBindAttempts < 1 || p.MaxBindAttempts > 4096 || p.MaxQueueBytes < 248 || p.MaxQueueBytes > 1<<30 || p.MaxLedgerMessages < 1 || p.MaxLedgerMessages > 1<<20 || p.MaxEventBytes < 16384 || p.MaxEventBytes > 1<<30 || p.MaxFileBytes < p.MaxEventBytes || p.MaxFileBytes > 1<<40 {
		return errors.New("invalid explicit server resource ceilings")
	}
	return nil
}

// Check rejects plans outside locally approved bindings/resources rather than substituting settings.
func (p ServerPolicy) Check(plan profile.Plan) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if (net.ParseIP(p.DataBindIP).To4() != nil) != (plan.Family == "ipv4") {
		return errors.New("server data binding does not support requested address family")
	}
	l := plan.Limits
	if plan.DestinationPort < p.PortMin || plan.DestinationPort > p.PortMax || l.SetupTimeout > p.SetupTimeout || l.ShutdownTimeout > p.ShutdownTimeout || l.ControlRequests > p.MaxControlRequests || l.QueueBytes > p.MaxQueueBytes || l.LedgerMessages > p.MaxLedgerMessages || l.EventBytes > p.MaxEventBytes || l.FileBytes > p.MaxFileBytes {
		return errors.New("requested plan exceeds explicit server ceilings")
	}
	return nil
}

// lock serializes cooperating CLI writers without deleting or stealing a crashed writer's lock.
func (s *Store) lock() (*os.File, error) {
	return os.OpenFile(filepath.Join(s.Root, ".store-lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
}

// unlock removes only the lock created by this operation.
func (s *Store) unlock(f *os.File) { f.Close(); os.Remove(filepath.Join(s.Root, ".store-lock")) }

// writeAtomic publishes a fully synced restricted temporary file, preserving valid state on failure.
func (s *Store) writeAtomic(target string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > FrameMax {
		return errors.New("store document exceeds 1 MiB")
	}
	var suffix [16]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(target), ".write-"+hex.EncodeToString(suffix[:]))
	f, err := createPrivate(temp)
	if err != nil {
		return err
	}
	defer func() { // Remove only this operation's unpublished temporary file.
		f.Close()
		os.Remove(temp)
	}()
	if err = restrictKey(f); err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return replaceFile(temp, target)
}

// readObject bounds disk inputs and rejects symlink files, unknown fields and duplicate JSON keys.
func readObject(path string, out any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("store input must be a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, FrameMax+1))
	if err != nil {
		return err
	}
	return DecodeObject(b, out)
}

// ReadObject loads an explicitly selected public import/policy document through the common guard.
func ReadObject(path string, out any) error { return readObject(path, out) }

// InitServer generates a host key on this endpoint and records public pairing plus explicit policy.
func (s *Store) InitServer(id, address string, port int, policy ServerPolicy, keyRef string, passphrase ...[]byte) (Bundle, error) {
	var out Bundle
	if err := policy.Validate(); err != nil {
		return out, err
	}
	lock, err := s.lock()
	if err != nil {
		return out, err
	}
	defer s.unlock(lock)
	path := filepath.Join(s.Root, "server.json")
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return out, errors.New("server identity already exists or cannot be inspected")
	}
	if _, err = os.Lstat(filepath.Join(s.Root, "authorization.json")); !os.IsNotExist(err) {
		return out, errors.New("authorization store already exists or cannot be inspected")
	}
	created := keyRef == ""
	if created {
		keyRef = filepath.Join(s.Root, "keys", "server-key")
	}
	generated := false
	success := false
	defer func() { // Roll back only files created by this initialization, preserving referenced keys.
		if !success && generated {
			os.Remove(keyRef)
		}
	}()
	var pub PublicIdentity
	if created {
		pub, err = GenerateKey(keyRef)
		generated = err == nil
	} else {
		var signer ssh.Signer
		signer, err = LoadKey(keyRef, firstPassphrase(passphrase))
		if err == nil {
			pub = publicSigner(signer)
		}
	}
	if err != nil {
		return out, err
	}
	out = Bundle{1, id, address, port, pub.PublicKey, pub.Fingerprint, []int{1}}
	if err = out.Validate(); err != nil {
		return Bundle{}, err
	}
	keyRef, err = filepath.Abs(keyRef)
	if err != nil {
		return Bundle{}, err
	}
	if err = s.writeAtomic(filepath.Join(s.Root, "authorization.json"), Authorization{Version: 1, Revision: 0, Clients: []Enrollment{}}); err != nil {
		return Bundle{}, err
	}
	if err = s.writeAtomic(path, ServerIdentity{1, keyRef, out, policy}); err != nil {
		os.Remove(filepath.Join(s.Root, "authorization.json"))
		return Bundle{}, err
	}
	success = true
	return out, nil
}

// publicSigner exposes only canonical public material from a loaded private-key reference.
func publicSigner(signer ssh.Signer) PublicIdentity {
	return PublicIdentity{strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), ssh.FingerprintSHA256(signer.PublicKey())}
}

// InitClient creates the client's local key and public enrollment record without server enrollment.
func (s *Store) InitClient(id, keyRef string, passphrase ...[]byte) (Enrollment, error) {
	var out Enrollment
	if !profile.SafeID(id) {
		return out, errors.New("invalid client ID")
	}
	lock, err := s.lock()
	if err != nil {
		return out, err
	}
	defer s.unlock(lock)
	path := filepath.Join(s.Root, "client.json")
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return out, errors.New("client identity already exists or cannot be inspected")
	}
	created := keyRef == ""
	if created {
		keyRef = filepath.Join(s.Root, "keys", "client-key")
	}
	generated := false
	success := false
	defer func() { // Never remove a user-supplied key reference when initialization fails.
		if !success && generated {
			os.Remove(keyRef)
		}
	}()
	var pub PublicIdentity
	if created {
		pub, err = GenerateKey(keyRef)
		generated = err == nil
	} else {
		var signer ssh.Signer
		signer, err = LoadKey(keyRef, firstPassphrase(passphrase))
		if err == nil {
			pub = publicSigner(signer)
		}
	}
	if err != nil {
		return out, err
	}
	keyRef, err = filepath.Abs(keyRef)
	if err != nil {
		return out, err
	}
	out = Enrollment{ClientID: id, PublicKey: pub.PublicKey, Enabled: true}
	if err = s.writeAtomic(path, ClientIdentity{1, keyRef, out}); err != nil {
		return Enrollment{}, err
	}
	success = true
	return out, nil
}

// firstPassphrase forwards an optional caller-owned passphrase without storing or logging it.
func firstPassphrase(values [][]byte) []byte {
	if len(values) > 0 {
		return values[0]
	}
	return nil
}

// Server reads and validates the application server identity without loading its private key.
func (s *Store) Server() (ServerIdentity, error) {
	var out ServerIdentity
	err := readObject(filepath.Join(s.Root, "server.json"), &out)
	if err != nil {
		return out, err
	}
	if out.Version != 1 || !filepath.IsAbs(out.KeyRef) {
		return out, errors.New("invalid server identity store")
	}
	if err = out.Bundle.Validate(); err != nil {
		return out, err
	}
	return out, out.Policy.Validate()
}

// Client reads validated public enrollment and the immutable local private-key reference.
func (s *Store) Client() (ClientIdentity, error) {
	var out ClientIdentity
	err := readObject(filepath.Join(s.Root, "client.json"), &out)
	if err != nil {
		return out, err
	}
	if out.Version != 1 || !filepath.IsAbs(out.KeyRef) {
		return out, errors.New("invalid client identity store")
	}
	return out, validateEnrollment(out.Enrollment)
}

// validateEnrollment checks exact public-key syntax and explicit application identity.
func validateEnrollment(e Enrollment) error {
	if !profile.SafeID(e.ClientID) {
		return errors.New("invalid enrollment client ID")
	}
	_, err := ParsePublic(e.PublicKey)
	return err
}

// Authorizations reloads the current bounded allowlist, rejecting duplicate IDs or keys.
func (s *Store) Authorizations() (Authorization, error) {
	var out Authorization
	if err := readObject(filepath.Join(s.Root, "authorization.json"), &out); err != nil {
		return out, err
	}
	if out.Version != 1 || out.Clients == nil || len(out.Clients) > 4096 {
		return out, errors.New("invalid authorization store")
	}
	ids, keys := map[string]bool{}, map[string]bool{}
	for _, e := range out.Clients {
		if err := validateEnrollment(e); err != nil {
			return out, err
		}
		key, _ := ParsePublic(e.PublicKey)
		canonical := string(key.Marshal())
		if ids[e.ClientID] || keys[canonical] {
			return out, errors.New("duplicate enrolled client identity/key")
		}
		ids[e.ClientID] = true
		keys[canonical] = true
	}
	return out, nil
}

// Enroll atomically adds/updates one explicit public enrollment; changed IDs cannot reuse another key.
func (s *Store) Enroll(e Enrollment) error {
	if err := validateEnrollment(e); err != nil {
		return err
	}
	lock, err := s.lock()
	if err != nil {
		return err
	}
	defer s.unlock(lock)
	auth, err := s.Authorizations()
	if err != nil {
		return err
	}
	key, _ := ParsePublic(e.PublicKey)
	found := false
	for i, old := range auth.Clients {
		other, _ := ParsePublic(old.PublicKey)
		if old.ClientID == e.ClientID {
			auth.Clients[i] = e
			found = true
		} else if string(other.Marshal()) == string(key.Marshal()) {
			return errors.New("key already belongs to another client ID")
		}
	}
	if !found {
		if len(auth.Clients) >= 4096 {
			return errors.New("authorization entry budget exhausted")
		}
		auth.Clients = append(auth.Clients, e)
	}
	if uint64(auth.Revision) == ^uint64(0) {
		return errors.New("authorization revision overflow")
	}
	auth.Revision++
	return s.writeAtomic(filepath.Join(s.Root, "authorization.json"), auth)
}

// Disable explicitly revokes one client and preserves its public identity/history entry.
func (s *Store) Disable(id string) error {
	lock, err := s.lock()
	if err != nil {
		return err
	}
	defer s.unlock(lock)
	auth, err := s.Authorizations()
	if err != nil {
		return err
	}
	for i, e := range auth.Clients {
		if e.ClientID == id {
			if uint64(auth.Revision) == ^uint64(0) {
				return errors.New("authorization revision overflow")
			}
			auth.Clients[i].Enabled = false
			auth.Revision++
			return s.writeAtomic(filepath.Join(s.Root, "authorization.json"), auth)
		}
	}
	return errors.New("client ID is not enrolled")
}

// ImportPairing saves a trusted public bundle; changing an existing host key requires explicit replacement.
func (s *Store) ImportPairing(id string, b Bundle, replace bool) error {
	if !profile.SafeID(id) {
		return errors.New("invalid pairing ID")
	}
	if err := b.Validate(); err != nil {
		return err
	}
	lock, err := s.lock()
	if err != nil {
		return err
	}
	defer s.unlock(lock)
	if _, err = s.ownedDir("pairings", false); err != nil {
		return err
	}
	path := filepath.Join(s.Root, "pairings", id+".json")
	var previous Bundle
	if err = readObject(path, &previous); err == nil && !replace {
		return errors.New("pairing exists; explicit -replace is required")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return s.writeAtomic(path, b)
}

// Pairing reads a validated imported host identity by safe application ID only.
func (s *Store) Pairing(id string) (Bundle, error) {
	var out Bundle
	if !profile.SafeID(id) {
		return out, errors.New("invalid pairing ID")
	}
	if _, err := s.ownedDir("pairings", false); err != nil {
		return out, err
	}
	if err := readObject(filepath.Join(s.Root, "pairings", id+".json"), &out); err != nil {
		return out, err
	}
	return out, out.Validate()
}

// CheckClient re-evaluates the current allowlist for handshake/readiness and revocation polling.
func (s *Store) CheckClient(key ssh.PublicKey, now time.Time) (string, error) {
	auth, err := s.Authorizations()
	if err != nil {
		return "", err
	}
	return Authorize(auth.Clients, key, now)
}
