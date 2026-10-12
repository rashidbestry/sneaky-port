package localapi

import (
	"net/http"
	"sneaky-port/internal/profile"
)

// bootstrap grants this exact-origin dashboard its page-memory bearer without manual token entry.
// A foreign page cannot set the custom request header without a rejected CORS preflight.
// Literal peer/Host checks run first; absent/foreign Origin or fetch metadata never bootstrap.
func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/v1/dashboard-session" {
		return false
	}
	id := r.Header.Get("X-Request-ID")
	if r.Method != "POST" || r.URL.RawQuery != "" || r.URL.RawPath != "" || r.Header.Get("Origin") != s.URL() || r.Header.Get("Sec-Fetch-Site") != "same-origin" || len(r.Header.Values("X-Dashboard-Bootstrap")) != 1 || r.Header.Get("X-Dashboard-Bootstrap") != "1" || r.ContentLength != 0 || len(r.Header.Values("X-Request-ID")) != 1 || !profile.SafeID(id) {
		reject(w, 403, "", "forbidden", "bootstrap", "exact-origin dashboard bootstrap required")
		return true
	}
	reply(w, 200, id, struct {
		Token string `json:"session_token"`
	}{s.token}, nil)
	return true
}
