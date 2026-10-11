package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// fixtureAPI creates an owned server with bounded cleanup and synthetic lifecycle behavior.
func fixtureAPI(t *testing.T, runner Runner) *Server {
	t.Helper()
	s, err := Listen("127.0.0.1:0", testutil.Plan(), runner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // Join even when a security assertion aborts the test.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return s
}

// stoppedRunner is an inert runner for boundary tests that must never start traffic.
func stoppedRunner(context.Context, profile.Plan) (control.RunResult, error) {
	return control.RunResult{Status: "completed"}, nil
}

// request builds valid baseline local headers, with caller-specific changes made afterward.
func request(s *Server, method, path, id, body string) *http.Request {
	r := httptest.NewRequest(method, s.URL()+path, strings.NewReader(body))
	r.URL.Scheme, r.URL.Host = "", "" // Match ordinary origin-form HTTP requests.
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Authorization", "Bearer "+s.Token())
	r.Header.Set("Origin", s.URL())
	r.Header.Set("X-Request-ID", id)
	r.Header.Set("Content-Type", "application/json")
	return r
}

// startBody serializes the required frozen revision, avoiding a browser-supplied path or mutable plan.
func startBody(s *Server, id string) string {
	b, _ := json.Marshal(mutation{1, id, s.plan.ID, s.Revision()})
	return string(b)
}

// invoke executes the real guard/router and verifies response secrecy and schema on all routes.
func invoke(t *testing.T, s *Server, r *http.Request, code int) envelope {
	t.Helper()
	w := httptest.NewRecorder()
	s.handle(w, r)
	if w.Code != code {
		t.Fatalf("%s %s: got %d want %d: %s", r.Method, r.URL.Path, w.Code, code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), s.Token()) || w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("secret/CORS/cache boundary failed")
	}
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || e.Version != 1 {
		t.Fatal("invalid response envelope", err)
	}
	if code >= 400 && (e.Error == nil || e.Error.Code == "" || e.Error.Field == "" || e.Error.Reason == "") {
		t.Fatal("missing structured error")
	}
	return e
}

// statusResult extracts only the bounded public status from an API envelope.
func statusResult(t *testing.T, e envelope) Status {
	t.Helper()
	b, _ := json.Marshal(e.Result)
	var out Status
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestLoopbackAndSessionTokens rejects every unsafe bind before listening and isolates process secrets.
func TestLoopbackAndSessionTokens(t *testing.T) {
	for _, address := range []string{":0", "0.0.0.0:0", "[::]:0", "192.0.2.1:0", "localhost:0", "127.0.0.1:01", "127.0.0.1:-1", "127.0.0.1:65536"} {
		if s, err := Listen(address, testutil.Plan(), stoppedRunner); err == nil {
			s.Close(context.Background())
			t.Fatalf("unsafe address accepted: %s", address)
		}
	}
	a, b := fixtureAPI(t, stoppedRunner), fixtureAPI(t, stoppedRunner)
	if len(a.Token()) != 64 || a.Token() == b.Token() {
		t.Fatal("session entropy/isolation failed")
	}
	r := request(a, "GET", "/api/v1/adapters", "read", "")
	r.Header.Set("Authorization", "Bearer "+b.Token())
	invoke(t, a, r, 401)
}

// TestGuardsProtectEveryRoute ensures even reads, mutations, wrong methods and nonexistent routes are guarded.
func TestGuardsProtectEveryRoute(t *testing.T) {
	var calls atomic.Int32
	s := fixtureAPI(t, func(context.Context, profile.Plan) (control.RunResult, error) { // Any invocation here proves a security regression.
		calls.Add(1)
		return control.RunResult{}, nil
	})
	for _, route := range []struct{ method, path string }{{"GET", "/api/v1/adapters"}, {"POST", "/api/v1/runs"}, {"GET", "/api/v1/runs/unknown"}, {"POST", "/api/v1/runs/unknown/cancel"}, {"OPTIONS", "/api/v1/runs"}, {"GET", "/unknown"}} {
		for _, attack := range []string{"missing_token", "wrong_token", "cookie_only", "evil_host", "wrong_port", "evil_origin", "null_origin", "duplicate_origin", "cross_site", "same_site", "remote_peer", "duplicate_auth", "absolute_url"} {
			r := request(s, route.method, route.path, "attack", startBody(s, "attack"))
			code := 403
			switch attack {
			case "missing_token":
				r.Header.Del("Authorization")
				code = 401
			case "wrong_token":
				r.Header.Set("Authorization", "Bearer wrong")
				code = 401
			case "cookie_only":
				r.Header.Del("Authorization")
				r.Header.Set("Cookie", "token="+s.Token())
				code = 401
			case "evil_host":
				r.Host = "attacker.example"
			case "wrong_port":
				r.Host = "127.0.0.1:1"
			case "evil_origin":
				r.Header.Set("Origin", "https://attacker.example")
			case "null_origin":
				r.Header.Set("Origin", "null")
			case "duplicate_origin":
				r.Header.Add("Origin", s.URL())
			case "cross_site":
				r.Header.Set("Sec-Fetch-Site", "cross-site")
			case "same_site":
				r.Header.Set("Sec-Fetch-Site", "same-site")
			case "remote_peer":
				r.RemoteAddr = "192.0.2.1:1234"
			case "duplicate_auth":
				r.Header.Add("Authorization", "Bearer "+s.Token())
				code = 401
			case "absolute_url":
				r.URL.Scheme, r.URL.Host = "http", s.host
			}
			invoke(t, s, r, code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("rejected request invoked runner")
	}
}

// TestJSONAndRouteBounds checks exact schema, path, pagination and body boundaries before admission.
func TestJSONAndRouteBounds(t *testing.T) {
	s := fixtureAPI(t, stoppedRunner)
	good := startBody(s, "start")
	for _, body := range []string{"{}", "null", good + "{}", strings.Replace(good, `"schema_version":1`, `"schema_version":2`, 1), strings.Replace(good, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1), strings.Replace(good, `"schema_version":1`, `"path":"secret","schema_version":1`, 1), strings.Replace(good, s.Revision(), "stale", 1)} {
		invoke(t, s, request(s, "POST", "/api/v1/runs", "start", body), 400)
	}
	invoke(t, s, request(s, "POST", "/api/v1/runs", "start", strings.Repeat("x", bodyMax+1)), 413)
	r := request(s, "POST", "/api/v1/runs", "start", good)
	r.Header.Set("Content-Type", "text/plain")
	invoke(t, s, r, 415)
	for _, path := range []string{"/api/v1/runs?token=secret", "/api/v1/runs/%2e%2e", "/api/v1/adapters?limit=0", "/api/v1/adapters?limit=17", "/api/v1/adapters?cursor=-1", "/api/v1/adapters?cursor=999", "/api/v1/adapters?limit=1&limit=2", "/api/v1/adapters?token=secret"} {
		invoke(t, s, request(s, "GET", path, "read", ""), 400)
	}
	e := invoke(t, s, request(s, "GET", "/api/v1/adapters?cursor=0&limit=1", "read", ""), 200)
	b, _ := json.Marshal(e.Result)
	if !strings.Contains(string(b), `"next_cursor":"1"`) || !strings.Contains(string(b), `"adapter_id":"tcp"`) {
		t.Fatal("registry pagination lost ordering/cursor")
	}
	invoke(t, s, request(s, "DELETE", "/api/v1/runs", "read", ""), 404)
	invoke(t, s, request(s, "GET", "/api/v1/runs/unknown", "read", ""), 404)
	if len(s.runs) != 0 {
		t.Fatal("invalid requests admitted runs")
	}
}

// TestConcurrentIdempotencyAndCancel proves single admission, immutable retry replies and no duplicate cancellation.
func TestConcurrentIdempotencyAndCancel(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	s := fixtureAPI(t, func(ctx context.Context, p profile.Plan) (control.RunResult, error) { // Hold owned work until explicit cancel, independent of request contexts.
		calls.Add(1)
		close(started)
		<-ctx.Done()
		p.Phases[0].ID = "mutated-private-copy"
		return control.RunResult{Status: "partial", Readiness: control.ReadyResult{RunID: strings.Repeat("a", 32), MeasurementStarted: true}}, ctx.Err()
	})
	var wg sync.WaitGroup
	ids := make(chan string, 20)
	for range 20 {
		wg.Add(1)
		go func() { // Concurrent retries share an atomic command admission and original acknowledgment.
			defer wg.Done()
			e := invoke(t, s, request(s, "POST", "/api/v1/runs", "same", startBody(s, "same")), 200)
			ids <- statusResult(t, e).ID
		}()
	}
	wg.Wait()
	close(ids)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner never started")
	}
	var id string
	for value := range ids {
		if id != "" && value != id {
			t.Fatal("duplicate start handle")
		}
		id = value
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate runner")
	}
	invoke(t, s, request(s, "POST", "/api/v1/runs", "other", startBody(s, "other")), 409)
	r := request(s, "POST", "/api/v1/runs/"+id+"/cancel", "cancel", `{"schema_version":1,"request_id":"cancel"}`)
	r.Header.Set("Origin", "https://evil.example")
	invoke(t, s, r, 403)
	if statusResult(t, invoke(t, s, request(s, "GET", "/api/v1/runs/"+id, "read", ""), 200)).State != "executing" {
		t.Fatal("rejected cancel changed state")
	}
	cancelBody := `{"schema_version":1,"request_id":"cancel"}`
	ack := statusResult(t, invoke(t, s, request(s, "POST", "/api/v1/runs/"+id+"/cancel", "cancel", cancelBody), 200))
	if ack.State != "cancelling" {
		t.Fatal("cancel not admitted")
	}
	final := waitStatus(t, s, id)
	if final.State != "partial" || final.MeasurementStarted == nil || !*final.MeasurementStarted || final.MeasurementID != strings.Repeat("a", 32) || final.Reason == "" {
		t.Fatal("lost final partial identity", final)
	}
	again := statusResult(t, invoke(t, s, request(s, "POST", "/api/v1/runs/"+id+"/cancel", "cancel", cancelBody), 200))
	if again != ack {
		t.Fatal("retry acknowledgment changed")
	}
	invoke(t, s, request(s, "POST", "/api/v1/runs", "cancel", startBody(s, "cancel")), 409)
	if s.plan.Phases[0].ID != "main" {
		t.Fatal("runner mutated launch plan")
	}
}

// waitStatus bounds fixture polling while waiting for runner cleanup/final publication.
func waitStatus(t *testing.T, s *Server, id string) Status {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out := statusResult(t, invoke(t, s, request(s, "GET", "/api/v1/runs/"+id, "poll", ""), 200))
		if out.State != "executing" && out.State != "cancelling" {
			return out
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("runner did not finalize")
	return Status{}
}

// TestHistoryBudgets preserves retries after exhaustion without silently evicting old commands/runs.
func TestHistoryBudgets(t *testing.T) {
	s := fixtureAPI(t, stoppedRunner)
	var first string
	for i := range maxRuns {
		id := fmt.Sprintf("start%d", i)
		out := statusResult(t, invoke(t, s, request(s, "POST", "/api/v1/runs", id, startBody(s, id)), 200))
		if i == 0 {
			first = out.ID
		}
		waitStatus(t, s, out.ID)
	}
	invoke(t, s, request(s, "POST", "/api/v1/runs", "overflow", startBody(s, "overflow")), 503)
	for i := maxRuns; i < maxCommands; i++ {
		id := fmt.Sprintf("cancel%d", i)
		invoke(t, s, request(s, "POST", "/api/v1/runs/"+first+"/cancel", id, fmt.Sprintf(`{"schema_version":1,"request_id":%q}`, id)), 200)
	}
	invoke(t, s, request(s, "POST", "/api/v1/runs/"+first+"/cancel", "overflow", `{"schema_version":1,"request_id":"overflow"}`), 503)
	if statusResult(t, invoke(t, s, request(s, "POST", "/api/v1/runs", "start0", startBody(s, "start0")), 200)).ID != first {
		t.Fatal("old idempotency key evicted")
	}
}

// TestOldCancelRetryCannotStopNewRun verifies identity-scoped cancellation and request-disconnect independence.
func TestOldCancelRetryCannotStopNewRun(t *testing.T) {
	s := fixtureAPI(t, func(ctx context.Context, p profile.Plan) (control.RunResult, error) { // Keep each generation active until its own owned context is cancelled.
		<-ctx.Done()
		return control.RunResult{Status: "partial"}, ctx.Err()
	})
	r := request(s, "POST", "/api/v1/runs", "first", startBody(s, "first"))
	requestCtx, disconnect := context.WithCancel(context.Background())
	disconnect()
	first := statusResult(t, invoke(t, s, r.WithContext(requestCtx), 200))
	invoke(t, s, request(s, "POST", "/api/v1/runs/"+first.ID+"/cancel", "stop-first", `{"schema_version":1,"request_id":"stop-first"}`), 200)
	waitStatus(t, s, first.ID)
	second := statusResult(t, invoke(t, s, request(s, "POST", "/api/v1/runs", "second", startBody(s, "second")), 200))
	invoke(t, s, request(s, "POST", "/api/v1/runs/"+first.ID+"/cancel", "stop-first", `{"schema_version":1,"request_id":"stop-first"}`), 200)
	state := statusResult(t, invoke(t, s, request(s, "GET", "/api/v1/runs/"+second.ID, "poll", ""), 200))
	if state.State != "executing" || state.MeasurementStarted != nil || state.Complete != nil {
		t.Fatal("old cancellation affected new run or unknown readiness became a boolean", state)
	}
	invoke(t, s, request(s, "POST", "/api/v1/runs/"+second.ID+"/cancel", "stop-second", `{"schema_version":1,"request_id":"stop-second"}`), 200)
	waitStatus(t, s, second.ID)
}

// TestRealHTTPAndShutdown covers literal IPv4/IPv6 listeners, native HTTP guards and owned-run cancellation.
func TestRealHTTPAndShutdown(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "[::1]:0"} {
		t.Run(address, func(t *testing.T) { // Authentic local HTTP is distinct from same-process handler fixtures and live-network acceptance.
			started, ended := make(chan struct{}), make(chan struct{})
			s, err := Listen(address, testutil.Plan(), func(ctx context.Context, p profile.Plan) (control.RunResult, error) { // Model an independently owned finite worker with cooperative cleanup.
				close(started)
				<-ctx.Done()
				close(ended)
				return control.RunResult{Status: "partial"}, ctx.Err()
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { // Serve the actual loopback listener until explicit process-context shutdown.
				done <- s.Serve(ctx, time.Second)
			}()
			t.Cleanup(func() { // Guarantee no fixture server survives early failure.
				cancel()
				join, stop := context.WithTimeout(context.Background(), time.Second)
				defer stop()
				s.Close(join)
			})
			client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
			defer client.CloseIdleConnections()
			r, _ := http.NewRequest("POST", s.URL()+"/api/v1/runs", strings.NewReader(startBody(s, "start")))
			r.Header.Set("Authorization", "Bearer "+s.Token())
			r.Header.Set("X-Request-ID", "start")
			r.Header.Set("Content-Type", "application/json")
			response, err := client.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatal(response.Status)
			}
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("HTTP start not executed")
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("server failed to join")
			}
			select {
			case <-ended:
			default:
				t.Fatal("owned runner leaked")
			}
		})
	}
}

// TestShutdownTimeout reports a noncooperative runner rather than claiming it has stopped.
func TestShutdownTimeout(t *testing.T) {
	release := make(chan struct{})
	s := fixtureAPI(t, func(context.Context, profile.Plan) (control.RunResult, error) { // Deliberately violate the runner contract until the fixture releases it.
		<-release
		return control.RunResult{}, errors.New("fixture failure")
	})
	invoke(t, s, request(s, "POST", "/api/v1/runs", "start", startBody(s, "start")), 200)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if s.Close(ctx) == nil {
		t.Error("noncooperative worker hidden")
	}
	close(release)
	invoke(t, s, request(s, "POST", "/api/v1/runs", "new", startBody(s, "new")), 503)
}
