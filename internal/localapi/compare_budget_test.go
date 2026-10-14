package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
)

// comparisonBudgetBody supplies valid explicit selections without reusing any live measurement identity.
func comparisonBudgetBody(t *testing.T, id string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"schema_version": 1, "request_id": id, "pairs": []control.TrialPair{{A: "diagnostic", B: "comparison"}}, "axes": []string{}, "endpoint": "local", "scope": "main"})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// comparisonDeadlineRecorder records admitted response budgets and can fail deadline support explicitly.
type comparisonDeadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
	failure   error
}

// SetWriteDeadline models the response-controller boundary without waiting for an operating-system timer.
func (w *comparisonDeadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return w.failure
}

// TestComparisonBudgetAdmission verifies that only protected, serialized, strictly valid comparisons extend writes.
func TestComparisonBudgetAdmission(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	importDiagnosticFixture(t, s)
	importComparisonFixture(t, s)
	for _, kind := range []string{"admitted", "invalid", "unauthorized", "busy", "deadline_failure"} {
		t.Run(kind, func(t *testing.T) { // Exercise the actual protected router and saved-operation admission.
			body := comparisonBudgetBody(t, "budget")
			code, calls := 200, 1
			if kind == "invalid" {
				body, code, calls = `{}`, 400, 0
			}
			r := request(s, "POST", "/api/v1/reports/compare", "budget", body)
			if kind == "unauthorized" {
				r.Header.Del("Authorization")
				code, calls = 401, 0
			}
			if kind == "busy" {
				s.savedMu.Lock()
				defer s.savedMu.Unlock()
				code, calls = 503, 0
			}
			w := &comparisonDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			if kind == "deadline_failure" {
				w.failure, code = errors.New("fixture deadline unavailable"), 503
			}
			notBefore := time.Now().Add(comparisonResponseBudget)
			s.handle(w, r)
			notAfter := time.Now().Add(comparisonResponseBudget)
			if w.Code != code || len(w.deadlines) != calls {
				t.Fatalf("admission changed: code=%d deadlines=%d body=%s", w.Code, len(w.deadlines), w.Body.String())
			}
			if calls != 0 && (w.deadlines[0].Before(notBefore) || w.deadlines[0].After(notAfter)) {
				t.Fatal("comparison response deadline lies outside its bounded admission interval")
			}
			if strings.Contains(w.Body.String(), s.Token()) || strings.Contains(w.Body.String(), s.store.Root) {
				t.Fatal("comparison budget response disclosed private state")
			}
		})
	}
}

// TestComparisonBackendWorkBudget verifies forwarded deadlines/cancellation and structured cooperative failures.
func TestComparisonBackendWorkBudget(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	importDiagnosticFixture(t, s)
	importComparisonFixture(t, s)
	q := control.ComparisonRequest{Pairs: []control.TrialPair{{A: "a", B: "b"}}, Axes: []string{}, Endpoint: "local", Scope: "main"}
	for _, kind := range []string{"budget", "earlier_parent", "cancelled", "expired", "deadline_failure"} {
		t.Run(kind, func(t *testing.T) { // A controlled backend observes the actual context supplied at the operation boundary.
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			var parentDeadline time.Time
			if kind == "earlier_parent" {
				parentDeadline = time.Now().Add(time.Second)
				var stop context.CancelFunc
				parent, stop = context.WithDeadline(parent, parentDeadline)
				defer stop()
			}
			r := request(s, "POST", "/api/v1/reports/compare", "budget", "").WithContext(parent)
			w := &comparisonDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			if kind == "deadline_failure" {
				w.failure = errors.New("fixture deadline unsupported")
			}
			calls := 0
			var retained context.Context
			before := time.Now().Add(comparisonWorkBudget)
			s.replyComparison(w, r, "budget", q, func(ctx context.Context, got control.ComparisonRequest) (control.Comparison, error) { // Capture forwarded work bounds and report controlled backend cancellation.
				calls++
				retained = ctx
				deadline, ok := ctx.Deadline()
				if !ok || got.Endpoint != q.Endpoint || len(got.Pairs) != 1 {
					t.Fatal("comparison work context or selection missing")
				}
				if kind == "earlier_parent" {
					if !deadline.Equal(parentDeadline) || !w.deadlines[0].Equal(parentDeadline) {
						t.Fatal("comparison extended an earlier parent deadline")
					}
				} else if deadline.Before(before) || deadline.After(time.Now().Add(comparisonWorkBudget)) || !deadline.Before(w.deadlines[0]) {
					t.Fatal("comparison work budget or response headroom missing")
				}
				if kind == "cancelled" {
					cancel()
					return control.Comparison{}, ctx.Err()
				}
				if kind == "expired" {
					return control.Comparison{}, context.DeadlineExceeded
				}
				return control.Comparison{Version: 1}, nil
			})
			wantCode, wantCalls := 200, 1
			if kind == "cancelled" || kind == "expired" || kind == "deadline_failure" {
				wantCode = 503
			}
			if kind == "deadline_failure" {
				wantCalls = 0
			}
			if w.Code != wantCode || calls != wantCalls {
				t.Fatalf("comparison budget outcome: code=%d backend_calls=%d body=%s", w.Code, calls, w.Body.String())
			}
			if retained != nil && retained.Err() == nil {
				t.Fatal("comparison retained a live work context after returning")
			}
			if wantCode == 503 && (!strings.Contains(w.Body.String(), `"code":"resource_limit"`) || strings.Contains(w.Body.String(), "invalid_evidence")) {
				t.Fatal("comparison liveness failure was mislabeled as invalid evidence")
			}
		})
	}
	// A canceled real request must reach the store's between-replay cancellation checks.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := &comparisonDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	s.handle(w, request(s, "POST", "/api/v1/reports/compare", "cancelled", comparisonBudgetBody(t, "cancelled")).WithContext(ctx))
	if w.Code != 503 || !strings.Contains(w.Body.String(), `"code":"resource_limit"`) {
		t.Fatal("real comparison backend ignored canceled work context", w.Body.String())
	}
}

// comparisonGateFactory retains real adapter validation while pausing the first actual saved replay.
type comparisonGateFactory struct {
	adapter.Factory
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

// Validate gates one immutable-report replay; it never changes its plan, carrier or validation result.
func (f *comparisonGateFactory) Validate(p profile.Plan) error {
	f.once.Do(func() { // Let the HTTP test control replay completion after observing both deadlines.
		close(f.entered)
		<-f.release
	})
	return f.Factory.Validate(p)
}

// comparisonDeadlineConn records the HTTP server's first actual response deadline without changing it.
type comparisonDeadlineConn struct {
	net.Conn
	first chan time.Time
	once  sync.Once
}

// SetWriteDeadline forwards socket deadlines and publishes the original short server response deadline.
func (c *comparisonDeadlineConn) SetWriteDeadline(deadline time.Time) error {
	err := c.Conn.SetWriteDeadline(deadline)
	if !deadline.IsZero() {
		c.once.Do(func() { // One buffered notification cannot block the HTTP worker or expose response bytes.
			c.first <- deadline
		})
	}
	return err
}

// comparisonDeadlineListener wraps only the fixture's accepted socket to observe its original write deadline.
type comparisonDeadlineListener struct {
	net.Listener
	first chan time.Time
}

// Accept preserves the owned listener and supplies a deadline-observing connection for each accepted socket.
func (l *comparisonDeadlineListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &comparisonDeadlineConn{Conn: c, first: l.first}, nil
}

// comparisonHTTPWriter observes comparison deadline selection while forwarding every real HTTP response operation.
type comparisonHTTPWriter struct {
	http.ResponseWriter
	selected chan time.Time
}

// SetWriteDeadline forwards the operation's selected deadline to the actual HTTP connection.
func (w *comparisonHTTPWriter) SetWriteDeadline(deadline time.Time) error {
	err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
	w.selected <- deadline
	return err
}

// awaitComparisonDeadline bounds fixture liveness while waiting for an actual selected socket deadline.
func awaitComparisonDeadline(t *testing.T, deadlines <-chan time.Time) time.Time {
	t.Helper()
	select {
	case deadline := <-deadlines:
		return deadline
	case <-time.After(5 * time.Second):
		t.Fatal("comparison fixture deadline was not selected")
		return time.Time{}
	}
}

// TestComparisonHTTPOutlivesDefaultWriteBudget gates real replay past the original response deadline.
// It proves actual protected HTTP completion, with no run/HTTP retry or arbitrary replay sleep.
func TestComparisonHTTPOutlivesDefaultWriteBudget(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	importDiagnosticFixture(t, s)
	importComparisonFixture(t, s)
	base, err := adapter.Default().Get("udp")
	if err != nil {
		t.Fatal(err)
	}
	release, entered := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { // Release the replay gate even if deadline/admission assertions fail.
		releaseOnce.Do(func() { // End this owned replay gate exactly once.
			close(release)
		})
	})
	registry := adapter.NewRegistry()
	if err := registry.Register(&comparisonGateFactory{Factory: base, entered: entered, release: release}); err != nil {
		t.Fatal(err)
	}
	s.store.Adapters = registry
	original, selected := make(chan time.Time, 1), make(chan time.Time, 1)
	s.listener = &comparisonDeadlineListener{Listener: s.listener, first: original}
	// Keep the ordinary product deadline; the regression crosses its observed socket boundary.
	s.http.WriteTimeout = 3 * time.Second
	s.http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // Observe the admitted route's deadline on the real protected HTTP handler.
		s.handle(&comparisonHTTPWriter{ResponseWriter: w, selected: selected}, r)
	})
	serverDone := make(chan error, 1)
	go func() { // Serve the existing owned listener; cleanup joins this worker after releasing replay.
		serverDone <- s.http.Serve(s.listener)
	}()
	t.Cleanup(func() { // Close actual HTTP ownership before the shared fixture store is removed.
		releaseOnce.Do(func() { // Unblock the selected replay before joining HTTP ownership.
			close(release)
		})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.Close(ctx); err != nil {
			t.Error(err)
		}
		select {
		case err := <-serverDone:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("comparison HTTP server cleanup timeout")
		}
	})
	r, err := http.NewRequest("POST", s.URL()+"/api/v1/reports/compare", strings.NewReader(comparisonBudgetBody(t, "http-budget")))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+s.Token())
	r.Header.Set("Origin", s.URL())
	r.Header.Set("X-Request-ID", "http-budget")
	r.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 35 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	type response struct {
		code int
		body []byte
		err  error
	}
	result := make(chan response, 1)
	go func() { // Read one actual response without logging bearer headers or retrying transport errors.
		got, err := client.Do(r)
		if err != nil {
			result <- response{err: err}
			return
		}
		defer got.Body.Close()
		b, err := io.ReadAll(got.Body)
		result <- response{code: got.StatusCode, body: b, err: err}
	}()
	oldDeadline := awaitComparisonDeadline(t, original)
	newDeadline := awaitComparisonDeadline(t, selected)
	if !newDeadline.After(oldDeadline) {
		t.Fatal("comparison did not extend the ordinary HTTP response budget")
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("actual comparison replay never reached its controlled gate")
	}
	// This timer waits for an observed contract boundary; it does not delay replay by an invented duration.
	timer := time.NewTimer(max(0, time.Until(oldDeadline)))
	<-timer.C
	releaseOnce.Do(func() { // Permit actual replay only after the original response deadline has passed.
		close(release)
	})
	select {
	case got := <-result:
		var e envelope
		if got.err != nil || got.code != 200 || json.Unmarshal(got.body, &e) != nil || e.Version != 1 {
			t.Fatalf("comparison failed after original response deadline: status=%d error=%v", got.code, got.err)
		}
		if !strings.Contains(string(got.body), `"state":"inconclusive"`) || strings.Contains(string(got.body), s.Token()) {
			t.Fatal("actual comparison response lost evidence or secret boundaries")
		}
	case <-time.After(40 * time.Second):
		t.Fatal("actual comparison response watchdog expired")
	}
}
