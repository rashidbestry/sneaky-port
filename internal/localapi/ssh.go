package localapi

import (
	"net/http"
	"sneaky-port/internal/control"
	"time"
)

// EnableSSH installs the normal-Linux-SSH workflow before serving; existing fixture/CLI APIs can stay keyed.
func (s *Server) EnableSSH(ssh *control.SSHDashboard) { s.ssh = ssh }

// sshConnection serializes bounded Connect/Disconnect with run admission, never recording passwords.
func (s *Server) sshConnection(w http.ResponseWriter, r *http.Request, id string) bool {
	if r.URL.Path != "/api/v1/connection" {
		return false
	}
	if s.ssh == nil {
		reject(w, 501, id, "unsupported", "connection", "Linux SSH connector unavailable")
		return true
	}
	if r.Method == "GET" {
		reply(w, 200, id, s.ssh.Status(), nil)
		return true
	}
	if r.Method != "POST" && r.Method != "DELETE" {
		reject(w, 400, id, "invalid_request", "method", "GET, POST or DELETE required")
		return true
	}
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(25 * time.Second))
	s.mu.Lock()
	if s.closed || s.connecting || s.active != "" {
		s.mu.Unlock()
		reject(w, 409, id, "conflict", "connection", "finish or cancel the active run/connection action first")
		return true
	}
	s.connecting = true
	s.wg.Add(1)
	s.mu.Unlock()
	defer func() { /* Release connection admission after every finite setup/cleanup outcome. */
		s.mu.Lock()
		s.connecting = false
		s.mu.Unlock()
		s.wg.Done()
	}()
	if r.Method == "DELETE" {
		s.ssh.Close()
		reply(w, 200, id, s.ssh.Status(), nil)
		return true
	}
	var q control.SSHConnectRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if !savedBody(w, r, id, &q) {
		return true
	}
	if q.Version != 1 || q.RequestID != id {
		reject(w, 400, id, "invalid_request", "connection", "bounded connection fields and request identity required")
		return true
	}
	status, err := s.ssh.Connect(s.ctx, q)
	q.Password = ""
	if err != nil {
		reject(w, 400, id, "connection_failed", "ssh", err.Error())
		return true
	}
	reply(w, 200, id, status, nil)
	return true
}
