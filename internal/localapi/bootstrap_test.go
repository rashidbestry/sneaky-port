package localapi

import (
	"encoding/json"
	"net/http/httptest"
	"sneaky-port/internal/control"
	"strings"
	"testing"
)

// TestDashboardBootstrapGuards permits an exact-origin page grant while retaining all private API guards.
func TestDashboardBootstrapGuards(t *testing.T) {
	s := fixtureAPI(t, stoppedRunner)
	for _, attack := range []string{"valid", "origin_missing", "origin_foreign", "fetch_missing", "fetch_foreign", "header_missing", "header_duplicate", "host", "peer", "method", "query", "body", "id_missing"} {
		r := request(s, "POST", "/api/v1/dashboard-session", "boot", "")
		r.Header.Del("Authorization")
		r.Header.Set("X-Dashboard-Bootstrap", "1")
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		switch attack {
		case "origin_missing":
			r.Header.Del("Origin")
		case "origin_foreign":
			r.Header.Set("Origin", "https://example.com")
		case "fetch_missing":
			r.Header.Del("Sec-Fetch-Site")
		case "fetch_foreign":
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		case "header_missing":
			r.Header.Del("X-Dashboard-Bootstrap")
		case "header_duplicate":
			r.Header.Add("X-Dashboard-Bootstrap", "1")
		case "host":
			r.Host = "localhost:9999"
		case "peer":
			r.RemoteAddr = "192.0.2.1:1234"
		case "method":
			r.Method = "GET"
		case "query":
			r.URL.RawQuery = "token=bad"
		case "body":
			r.ContentLength = 1
		case "id_missing":
			r.Header.Del("X-Request-ID")
		}
		w := httptest.NewRecorder()
		s.handle(w, r)
		if attack == "valid" {
			if w.Code != 200 || !strings.Contains(w.Body.String(), s.Token()) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("automatic dashboard grant failed")
			}
		} else if w.Code != 403 || strings.Contains(w.Body.String(), s.Token()) {
			t.Fatal("bootstrap bypass", attack, w.Code)
		}
	}
	r := request(s, "GET", "/api/v1/setup", "setup", "")
	r.Header.Del("Authorization")
	invoke(t, s, r, 401)
}

// TestSSHConnectionRouteGuards rejects unauthenticated, ambiguous and excessive password documents.
func TestSSHConnectionRouteGuards(t *testing.T) {
	s := fixtureSavedAPI(t, stoppedRunner)
	s.EnableSSH(control.NewSSHDashboard(s.store))
	for _, attack := range []string{"auth", "origin", "duplicate", "unknown", "large", "method"} {
		body := `{"schema_version":1,"request_id":"ssh","server_ip":"127.0.0.1","ssh_port":22,"ssh_username":"sneak","ssh_password":"fixture-secret"}`
		if attack == "duplicate" {
			body = strings.Replace(body, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1)
		}
		if attack == "unknown" {
			body = strings.Replace(body, `"schema_version":1`, `"schema_version":1,"unknown":true`, 1)
		}
		if attack == "large" {
			body = strings.Replace(body, "fixture-secret", strings.Repeat("x", 5000), 1)
		}
		r := request(s, "POST", "/api/v1/connection", "ssh", body)
		if attack == "auth" {
			r.Header.Del("Authorization")
		}
		if attack == "origin" {
			r.Header.Set("Origin", "null")
		}
		if attack == "method" {
			r.Method = "PUT"
		}
		w := httptest.NewRecorder()
		s.handle(w, r)
		if w.Code < 400 || strings.Contains(w.Body.String(), "fixture-secret") {
			t.Fatal("SSH route guard failed", attack, w.Code)
		}
	}
	w := httptest.NewRecorder()
	s.handle(w, request(s, "GET", "/api/v1/connection", "status", ""))
	var e struct {
		Result control.SSHConnectionStatus `json:"result"`
	}
	if json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Result.State != "disconnected" {
		t.Fatal("invalid connection status")
	}
}
