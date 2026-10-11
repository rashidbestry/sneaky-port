// Package localapi owns the loopback HTTP boundary; it never schedules probe traffic.
package localapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
)

const maxRuns = 32
const maxCommands = 128
const bodyMax = 4096

// Runner executes the existing finite lifecycle independently of HTTP request lifetimes.
type Runner func(context.Context, profile.Plan) (control.RunResult, error)

// Status distinguishes the API handle from the engine's eventual measurement identity.
// Executing includes setup/data/finalization; live phase and sample totals are not available yet.
type Status struct {
	ID                 string `json:"control_run_id"`
	ProfileID          string `json:"profile_id"`
	Revision           string `json:"profile_revision"`
	State              string `json:"status"`
	MeasurementID      string `json:"measurement_run_id"`
	MeasurementStarted *bool  `json:"measurement_started"`
	Complete           *bool  `json:"local_complete"`
	Reason             string `json:"reason"`
}

// command retains immutable acknowledgments so retries never start or cancel a different run.
type command struct {
	fingerprint string
	status      Status
}

// runState owns cancellation until the finite runner returns, including all cleanup.
type runState struct {
	status Status
	cancel context.CancelFunc
}

// Server owns a literal loopback listener, one process token and bounded lifecycle history.
type Server struct {
	listener net.Listener
	http     *http.Server
	token    string
	host     string
	plan     profile.Plan
	revision string
	runner   Runner
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	runs     map[string]*runState
	commands map[string]command
	active   string
	closed   bool
	wg       sync.WaitGroup
	store    *control.Store
	savedMu  sync.Mutex
}

// Listen validates and snapshots the configured plan before opening only a literal loopback address.
// Hostnames, wildcard binds and unspecified addresses are rejected before DNS resolution/listening.
func Listen(address string, p profile.Plan, runner Runner) (*Server, error) {
	host, port, err := net.SplitHostPort(address)
	ip := net.ParseIP(host)
	n, parseErr := strconv.ParseUint(port, 10, 16)
	if err != nil || parseErr != nil || strconv.FormatUint(n, 10) != port || ip == nil || !ip.IsLoopback() || runner == nil {
		return nil, errors.New("API requires a literal loopback IP:port and finite runner")
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	p, err = profile.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	hash, err := p.Hash()
	if err != nil {
		return nil, err
	}
	revision := hex.EncodeToString(hash[:])
	token, err := randomID(32)
	if err != nil {
		return nil, err
	}
	network := "tcp4"
	if strings.Contains(host, ":") {
		network = "tcp6"
	}
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{listener: ln, host: ln.Addr().String(), token: token, plan: p, revision: revision, runner: runner, ctx: ctx, cancel: cancel, runs: map[string]*runState{}, commands: map[string]command{}}
	s.http = &http.Server{Handler: http.HandlerFunc(s.handle), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192}
	return s, nil
}

// ListenWithStore connects saved public configuration/evidence without changing run ownership.
// The launch profile is a transient fallback; only PUT explicitly persists a profile.
func ListenWithStore(address string, p profile.Plan, store *control.Store, runner Runner) (*Server, error) {
	if store == nil {
		return nil, errors.New("application store required")
	}
	if _, err := store.Profile(p.ID); err != nil && !os.IsNotExist(err) {
		return nil, errors.New("saved launch profile failed validation")
	}
	s, err := Listen(address, p, runner)
	if err == nil {
		s.store = store
	}
	return s, err
}

// randomID uses cryptographic entropy for session tokens and control handles, never URL secrets.
func randomID(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}

// URL is the sole accepted HTTP authority/origin; aliases and proxy forwarding are not trusted.
func (s *Server) URL() string { return "http://" + s.host }

// Token exposes bootstrap credentials only to the local launcher, never through HTTP.
func (s *Server) Token() string { return s.token }

// Revision exposes the selected launch ID's current saved hash, or its transient fallback hash.
// Callers still send that revision explicitly; a later save may make a new start stale.
func (s *Server) Revision() string {
	_, revision, err := s.selectedProfile(s.plan.ID)
	if err != nil {
		return ""
	}
	return revision
}

// Serve runs the owned listener and cancels/join runs on context or listener failure.
// Shutdown must have an explicit finite budget; expiration is reported rather than hidden.
func (s *Server) Serve(ctx context.Context, shutdown time.Duration) error {
	if shutdown <= 0 {
		return errors.New("positive API shutdown budget required")
	}
	done := make(chan error, 1)
	go func() { // HTTP handlers finish quickly; measurement runs belong to the server context.
		done <- s.http.Serve(s.listener)
	}()
	var err error
	select {
	case <-ctx.Done():
	case err = <-done:
	}
	join, cancel := context.WithTimeout(context.Background(), shutdown)
	defer cancel()
	closeErr := s.Close(join)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return errors.Join(err, closeErr)
}

// Close stops admissions/HTTP, cancels the active run and joins owned finite work within the caller budget.
func (s *Server) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.http.Close()
	s.listener.Close()
	done := make(chan struct{})
	go func() { // Join all admitted runs; the runner must obey its existing bounded cleanup contract.
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return errors.New("API finite-run cleanup exceeded shutdown budget")
	}
}

// apiError is a structured public error with no reflected body, token or arbitrary input.
type apiError struct {
	Code   string `json:"code"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// envelope gives every result/error a schema and request identity; read requests use header identities.
type envelope struct {
	Version   int       `json:"schema_version"`
	RequestID string    `json:"request_id"`
	Result    any       `json:"result,omitempty"`
	Error     *apiError `json:"error,omitempty"`
}

// reply writes secret-free JSON without caching or permissive CORS headers.
func reply(w http.ResponseWriter, code int, id string, result any, e *apiError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(envelope{1, id, result, e})
}

// reject returns fixed structured errors rather than echoing untrusted request values.
func reject(w http.ResponseWriter, code int, id, kind, field, reason string) {
	reply(w, code, id, nil, &apiError{kind, field, reason})
}

// handle applies peer/authority/origin guards to every route and token guards to all API reads/mutations.
// Only the exact public static dashboard shell/assets bypass credentials; they contain no session settings.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(peer).IsLoopback() || r.Host != s.host || r.URL.IsAbs() {
		reject(w, 403, "", "forbidden", "host", "loopback peer and exact local authority required")
		return
	}
	origins := r.Header.Values("Origin")
	fetch := r.Header.Get("Sec-Fetch-Site")
	if len(origins) > 1 || len(r.Header.Values("Sec-Fetch-Site")) > 1 || len(origins) == 1 && origins[0] != s.URL() || fetch != "" && fetch != "same-origin" && fetch != "none" {
		reject(w, 403, "", "forbidden", "origin", "same local origin required")
		return
	}
	if s.dashboard(w, r) {
		return
	}
	auth := r.Header.Values("Authorization")
	if len(auth) != 1 || subtle.ConstantTimeCompare([]byte(auth[0]), []byte("Bearer "+s.token)) != 1 {
		reject(w, 401, "", "unauthorized", "authorization", "session bearer token required")
		return
	}
	id := r.Header.Get("X-Request-ID")
	if len(r.Header.Values("X-Request-ID")) != 1 || !profile.SafeID(id) {
		reject(w, 400, "", "invalid_request", "request_id", "safe X-Request-ID required")
		return
	}
	if r.URL.EscapedPath() != r.URL.Path || r.URL.RawPath != "" || r.URL.RawQuery != "" && !queryRoute(r.URL.Path) {
		reject(w, 400, id, "invalid_request", "url", "encoded paths and unexpected queries rejected")
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/api/v1/adapters" {
		s.adapters(w, r, id)
		return
	}
	if s.saved(w, r, id) {
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs" {
		s.mutate(w, r, id, "")
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/runs/"), "/")
	if strings.HasPrefix(r.URL.Path, "/api/v1/runs/") && profile.SafeID(parts[0]) {
		if r.Method == http.MethodGet && len(parts) == 1 {
			s.status(w, id, parts[0])
			return
		}
		if r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "cancel" {
			s.mutate(w, r, id, parts[0])
			return
		}
	}
	reject(w, 404, id, "not_found", "route", "route or method unavailable")
}

// adapters returns a bounded registry page; no protocol switches or wildcard CORS are added.
func (s *Server) adapters(w http.ResponseWriter, r *http.Request, id string) {
	q, queryErr := url.ParseQuery(r.URL.RawQuery)
	if queryErr != nil {
		reject(w, 400, id, "invalid_request", "query", "malformed query rejected")
		return
	}
	for k, v := range q {
		if (k != "cursor" && k != "limit") || len(v) != 1 {
			reject(w, 400, id, "invalid_request", "query", "only one cursor and limit allowed")
			return
		}
	}
	cursor, limit := 0, 16
	var err error
	if q.Has("cursor") {
		cursor, err = strconv.Atoi(q.Get("cursor"))
	}
	if err == nil && q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
	}
	items := adapter.Default().Describe()
	if err != nil || cursor < 0 || cursor > len(items) || limit < 1 || limit > 16 {
		reject(w, 400, id, "invalid_request", "query", "cursor or limit outside bounds")
		return
	}
	end := min(len(items), cursor+limit)
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	}
	reply(w, 200, id, struct {
		Items []adapter.Descriptor `json:"items"`
		Next  string               `json:"next_cursor"`
	}{items[cursor:end], next}, nil)
}

// status takes a small immutable snapshot; it never blocks the measurement engine or exposes key/file paths.
func (s *Server) status(w http.ResponseWriter, request, run string) {
	s.mu.Lock()
	v, ok := s.runs[run]
	var out Status
	if ok {
		out = v.status
	}
	s.mu.Unlock()
	if !ok {
		reject(w, 404, request, "not_found", "run_id", "unknown control run")
		return
	}
	reply(w, 200, request, out, nil)
}

// mutation names explicit frozen profile inputs; cancellation carries identity/version only.
type mutation struct {
	Version   int    `json:"schema_version"`
	RequestID string `json:"request_id"`
	ProfileID string `json:"profile_id,omitempty"`
	Revision  string `json:"profile_revision,omitempty"`
}

// mutate validates bounded JSON before atomic admission, and caches exact successful acknowledgments.
// Histories never evict idempotency keys: exhaustion requires a new process, not a duplicate run.
func (s *Server) mutate(w http.ResponseWriter, r *http.Request, id, target string) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		reject(w, 415, id, "invalid_request", "content_type", "application/json required")
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, bodyMax))
	if err != nil {
		reject(w, 413, id, "invalid_request", "body", "request exceeds body budget")
		return
	}
	var m mutation
	if err = control.DecodeObject(b, &m); err != nil || m.Version != 1 || m.RequestID != id || target == "" && (!profile.SafeID(m.ProfileID) || m.Revision == "") || target != "" && (m.ProfileID != "" || m.Revision != "") {
		reject(w, 400, id, "invalid_request", "body", "version, identity and frozen profile revision must match")
		return
	}
	fingerprint := "start:" + m.ProfileID + ":" + m.Revision
	if target != "" {
		fingerprint = "cancel:" + target
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.commands[id]; ok {
		if previous.fingerprint != fingerprint {
			reject(w, 409, id, "conflict", "request_id", "request ID already names another command")
			return
		}
		reply(w, 200, id, previous.status, nil)
		return
	}
	if s.closed || len(s.commands) >= maxCommands {
		reject(w, 503, id, "resource_limit", "commands", "API closed or command history exhausted")
		return
	}
	var state *runState
	if target == "" {
		p, revision, err := s.selectedProfile(m.ProfileID)
		if err != nil || revision != m.Revision {
			reject(w, 409, id, "conflict", "profile_revision", "selected profile revision unavailable or stale")
			return
		}
		if s.active != "" {
			reject(w, 409, id, "conflict", "run", "one finite run already executing")
			return
		}
		if len(s.runs) >= maxRuns {
			reject(w, 503, id, "resource_limit", "runs", "run history exhausted")
			return
		}
		run, err := randomID(16)
		if err != nil {
			reject(w, 500, id, "internal", "run", "run identity unavailable")
			return
		}
		ctx, cancel := context.WithCancel(s.ctx)
		state = &runState{status: Status{ID: run, ProfileID: p.ID, Revision: revision, State: "executing"}, cancel: cancel}
		s.runs[run], s.active = state, run
		s.wg.Add(1)
		go s.execute(ctx, state, p)
	} else {
		var ok bool
		state, ok = s.runs[target]
		if !ok {
			reject(w, 404, id, "not_found", "run_id", "unknown control run")
			return
		}
		if s.active == target {
			state.status.State = "cancelling"
			state.cancel()
		}
	}
	s.commands[id] = command{fingerprint, state.status}
	reply(w, 200, id, state.status, nil)
}

// execute owns a private frozen plan and publishes only actual final lifecycle/identity/completeness.
// Request cancellation/disconnect never cancels a run; explicit cancel or process shutdown does.
func (s *Server) execute(ctx context.Context, state *runState, frozen profile.Plan) {
	defer s.wg.Done()
	defer state.cancel()
	b, _ := json.Marshal(frozen)
	p, _ := profile.Decode(bytes.NewReader(b))
	result, err := s.runner(ctx, p)
	s.mu.Lock()
	defer s.mu.Unlock()
	state.status.State = result.Status
	if err != nil && result.Status != "partial" {
		state.status.State = "failed"
	}
	state.status.MeasurementID = result.Readiness.RunID
	started := result.Readiness.MeasurementStarted
	state.status.MeasurementStarted = &started
	if result.Local != nil {
		complete := result.Local.Complete
		state.status.Complete = &complete
	}
	if err != nil {
		state.status.Reason = "finite run failed or was interrupted; inspect local evidence"
	}
	s.active = ""
}
