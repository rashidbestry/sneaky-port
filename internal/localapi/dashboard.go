package localapi

import (
	"embed"
	"net/http"
	"os"

	"sneaky-port/internal/control"
)

// dashboardAssets contains only public static code; session credentials/settings are never interpolated.
//
//go:embed web/index.html web/dashboard.js web/dashboard.css web/diagnostics.js web/compare.js
var dashboardAssets embed.FS

// dashboard serves exact public shell paths after peer/Host/Origin guards, without weakening API auth.
// External assets permit a restrictive CSP without inline scripts, URL secrets, CDN or runtime build tools.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) bool {
	file, media := "", ""
	switch r.URL.Path {
	case "/":
		file, media = "index.html", "text/html; charset=utf-8"
	case "/dashboard.js":
		file, media = "dashboard.js", "text/javascript; charset=utf-8"
	case "/dashboard.css":
		file, media = "dashboard.css", "text/css; charset=utf-8"
	case "/diagnostics.js":
		file, media = "diagnostics.js", "text/javascript; charset=utf-8"
	case "/compare.js":
		file, media = "compare.js", "text/javascript; charset=utf-8"
	default:
		return false
	}
	if r.Method != "GET" || r.URL.RawQuery != "" || r.URL.RawPath != "" || r.URL.EscapedPath() != r.URL.Path {
		reject(w, 400, "", "invalid_request", "url", "dashboard requires exact GET asset paths without queries")
		return true
	}
	b, _ := dashboardAssets.ReadFile("web/" + file)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", media)
	w.WriteHeader(200)
	w.Write(b)
	return true
}

// setup exposes the normalized launch selection and public enrollment through protected, serialized reads.
// The saved flag distinguishes an unsaved launch fallback from a persisted revision; no key path is returned.
func (s *Server) setup(w http.ResponseWriter, id string) {
	p, revision, err := s.selectedProfile(s.plan.ID)
	if err != nil {
		s.diskReply(w, id, nil, err)
		return
	}
	saved := false
	var client *control.Enrollment
	if s.store != nil {
		_, err = s.store.Profile(p.ID)
		saved = err == nil
		if err != nil && !os.IsNotExist(err) {
			s.diskReply(w, id, nil, err)
			return
		}
		identity, err := s.store.Client()
		if err == nil {
			client = &identity.Enrollment
		} else if !os.IsNotExist(err) {
			s.diskReply(w, id, nil, err)
			return
		}
	}
	reply(w, 200, id, struct {
		Profile control.SavedProfile `json:"selected_profile"`
		Saved   bool                 `json:"saved"`
		Client  *control.Enrollment  `json:"client_enrollment"`
	}{control.SavedProfile{Plan: p, Revision: revision}, saved, client}, nil)
}
