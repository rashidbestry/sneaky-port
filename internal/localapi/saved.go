package localapi

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
)

// queryRoute permits queries only on bounded catalogs and explicit report export/diagnostic selectors.
func queryRoute(path string) bool {
	return path == "/api/v1/adapters" || path == "/api/v1/profiles" || path == "/api/v1/pairings" || path == "/api/v1/reports" || strings.HasPrefix(path, "/api/v1/reports/") && (strings.HasSuffix(path, "/export") || strings.HasSuffix(path, "/diagnostics"))
}

// selectedProfile takes a validated private snapshot; a saved ID takes precedence over the transient launch plan.
// Subsequent saves cannot mutate a plan already admitted to the finite runner.
func (s *Server) selectedProfile(id string) (profile.Plan, string, error) {
	if s.store != nil {
		p, err := s.store.Profile(id)
		if err == nil {
			return p.Plan, p.Revision, nil
		}
		if !os.IsNotExist(err) {
			return profile.Plan{}, "", err
		}
	}
	if id == s.plan.ID {
		return s.plan, s.revision, nil
	}
	return profile.Plan{}, "", os.ErrNotExist
}

// saved routes protected persistent operations with one active disk/replay operation per API process.
// Busy operations reject before body allocation; handlers never run on measurement worker goroutines.
func (s *Server) saved(w http.ResponseWriter, r *http.Request, id string) bool {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/"), "/")
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") || len(parts) == 0 {
		return false
	}
	ns := parts[0]
	if ns != "profiles" && ns != "pairings" && ns != "reports" && ns != "client-keys" && ns != "setup" && ns != "preflight" {
		return false
	}
	if s.store == nil && ns != "setup" && ns != "preflight" {
		return false
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		reject(w, 503, id, "resource_limit", "storage", "API closed")
		return true
	}
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	if !s.savedMu.TryLock() {
		reject(w, 503, id, "resource_limit", "storage", "another saved operation is executing")
		return true
	}
	defer s.savedMu.Unlock()
	if ns == "preflight" {
		if len(parts) == 1 && r.Method == "POST" {
			s.preflight(w, r, id)
		} else {
			reject(w, 404, id, "not_found", "route", "route or method unavailable")
		}
		return true
	}
	if ns == "setup" {
		if len(parts) == 1 && r.Method == "GET" {
			s.setup(w, id)
		} else {
			reject(w, 404, id, "not_found", "route", "route or method unavailable")
		}
		return true
	}
	if len(parts) == 1 && r.Method == "GET" && ns != "client-keys" {
		s.catalog(w, r, id, ns)
		return true
	}
	if len(parts) > 1 && !profile.SafeID(parts[1]) {
		reject(w, 400, id, "invalid_request", "id", "safe application ID required")
		return true
	}
	switch {
	case ns == "reports" && len(parts) == 2 && parts[1] == "compare" && r.Method == "POST":
		s.compareReports(w, r, id)
	case ns == "profiles" && len(parts) == 2 && parts[1] == "validate" && r.Method == "POST":
		s.profileWrite(w, r, id, "")
	case ns == "profiles" && len(parts) == 2 && r.Method == "PUT":
		s.profileWrite(w, r, id, parts[1])
	case ns == "profiles" && len(parts) == 2 && r.Method == "GET":
		p, err := s.store.Profile(parts[1])
		s.diskReply(w, id, p, err)
	case ns == "pairings" && len(parts) == 2 && parts[1] == "import" && r.Method == "POST":
		s.pairingImport(w, r, id)
	case ns == "pairings" && len(parts) == 2 && r.Method == "GET":
		p, err := s.store.Pairing(parts[1])
		s.diskReply(w, id, p, err)
	case ns == "client-keys" && len(parts) == 1 && r.Method == "POST":
		s.keyCreate(w, r, id)
	case ns == "client-keys" && len(parts) == 3 && parts[2] == "public" && r.Method == "GET":
		client, err := s.store.Client()
		if err == nil && client.Enrollment.ClientID != parts[1] {
			err = os.ErrNotExist
		}
		s.diskReply(w, id, client.Enrollment, err)
	case ns == "reports" && len(parts) == 2 && parts[1] == "import" && r.Method == "POST":
		s.reportImport(w, r, id)
	case ns == "reports" && len(parts) == 2 && r.Method == "GET":
		p, err := s.store.SavedReport(parts[1])
		s.diskReply(w, id, p, err)
	case ns == "reports" && len(parts) == 3 && parts[2] == "export" && r.Method == "GET":
		s.reportExport(w, r, id, parts[1])
	case ns == "reports" && len(parts) == 3 && parts[2] == "diagnostics" && r.Method == "GET":
		s.diagnostics(w, r, id, parts[1])
	default:
		reject(w, 404, id, "not_found", "route", "route or method unavailable")
	}
	return true
}

// diskReply maps filesystem/validator failures to fixed public messages without disclosing paths or keys.
func (s *Server) diskReply(w http.ResponseWriter, id string, value any, err error) {
	if err == nil {
		reply(w, 200, id, value, nil)
		return
	}
	if errors.Is(err, control.ErrRevision) {
		reject(w, 409, id, "conflict", "revision", "saved identity exists or revision is stale")
		return
	}
	if os.IsNotExist(err) {
		reject(w, 404, id, "not_found", "id", "saved identity unavailable")
		return
	}
	reject(w, 422, id, "invalid_evidence", "document", "saved operation failed validation or resource checks")
}

// catalog paginates safe IDs without loading reports; invalid/active evidence is evaluated on explicit read.
func (s *Server) catalog(w http.ResponseWriter, r *http.Request, id, namespace string) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	for k, v := range q {
		if (k != "cursor" && k != "limit") || len(v) != 1 {
			err = errors.New("invalid query")
		}
	}
	cursor, limit := 0, 16
	if err == nil && q.Has("cursor") {
		cursor, err = strconv.Atoi(q.Get("cursor"))
	}
	if err == nil && q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
	}
	if err != nil || cursor < 0 || limit < 1 || limit > 16 {
		reject(w, 400, id, "invalid_request", "query", "cursor or limit outside bounds")
		return
	}
	ids, err := s.store.SavedIDs(namespace)
	if err != nil {
		s.diskReply(w, id, nil, err)
		return
	}
	if cursor > len(ids) {
		reject(w, 400, id, "invalid_request", "cursor", "cursor outside catalog")
		return
	}
	end := min(len(ids), cursor+limit)
	next := ""
	if end < len(ids) {
		next = strconv.Itoa(end)
	}
	items := []any{}
	for _, savedID := range ids[cursor:end] {
		if namespace == "pairings" {
			bundle, err := s.store.Pairing(savedID)
			if err != nil {
				s.diskReply(w, id, nil, err)
				return
			}
			items = append(items, struct {
				ID     string         `json:"pairing_id"`
				Bundle control.Bundle `json:"bundle"`
			}{savedID, bundle})
		} else {
			items = append(items, savedID)
		}
	}
	reply(w, 200, id, struct {
		Items []any  `json:"items"`
		Next  string `json:"next_cursor"`
	}{items, next}, nil)
}

// savedBody reuses strict duplicate/presence/version validation with a 1 MiB document ceiling.
func savedBody(w http.ResponseWriter, r *http.Request, id string, out any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		reject(w, 415, id, "invalid_request", "content_type", "application/json required")
		return false
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, profile.DocumentMax))
	if err != nil {
		reject(w, 413, id, "resource_limit", "body", "request exceeds document budget")
		return false
	}
	if err = control.DecodeObject(b, out); err != nil {
		reject(w, 400, id, "invalid_request", "body", "strict versioned JSON required")
		return false
	}
	return true
}

// profileInput separates unsaved validation from an explicit revision-checked save.
type profileInput struct {
	Version   int             `json:"schema_version"`
	RequestID string          `json:"request_id"`
	Revision  string          `json:"profile_revision"`
	Profile   json.RawMessage `json:"profile"`
}

// profileWrite validates with the CLI decoder; validation never writes or changes run snapshots.
func (s *Server) profileWrite(w http.ResponseWriter, r *http.Request, id, target string) {
	var in profileInput
	if !savedBody(w, r, id, &in) {
		return
	}
	if in.Version != 1 || in.RequestID != id || target == "" && in.Revision != "" {
		reject(w, 400, id, "invalid_request", "identity", "version/request identity required; validation has no revision")
		return
	}
	p, err := profile.Decode(bytes.NewReader(in.Profile))
	if err != nil || target != "" && p.ID != target {
		reject(w, 422, id, "invalid_profile", "profile", "profile fields or identity failed shared validation")
		return
	}
	factory, err := adapter.Default().Get(p.Adapter)
	if err != nil || factory.Validate(p) != nil {
		reject(w, 422, id, "unsupported_profile", "adapter_options", "registered adapter and supported explicit options required")
		return
	}
	if target == "" {
		h, _ := p.Hash()
		s.diskReply(w, id, control.SavedProfile{Plan: p, Revision: hex.EncodeToString(h[:])}, nil)
		return
	}
	out, err := s.store.SaveProfile(target, in.Revision, p)
	s.diskReply(w, id, out, err)
}

// pairingImport validates the public host bundle and requires an explicit replacement decision.
func (s *Server) pairingImport(w http.ResponseWriter, r *http.Request, id string) {
	var in struct {
		Version   int            `json:"schema_version"`
		RequestID string         `json:"request_id"`
		ID        string         `json:"pairing_id"`
		Replace   bool           `json:"replace"`
		Bundle    control.Bundle `json:"bundle"`
	}
	if !savedBody(w, r, id, &in) {
		return
	}
	if in.Version != 1 || in.RequestID != id || !profile.SafeID(in.ID) {
		reject(w, 400, id, "invalid_request", "identity", "version and safe pairing identity required")
		return
	}
	if _, err := s.store.Pairing(in.ID); err == nil && !in.Replace {
		s.diskReply(w, id, nil, control.ErrRevision)
		return
	}
	err := s.store.ImportPairing(in.ID, in.Bundle, in.Replace)
	s.diskReply(w, id, in.Bundle, err)
}

// keyCreate generates only the store's single managed client identity; no browser key paths are accepted.
func (s *Server) keyCreate(w http.ResponseWriter, r *http.Request, id string) {
	var in struct {
		Version   int    `json:"schema_version"`
		RequestID string `json:"request_id"`
		ID        string `json:"client_id"`
	}
	if !savedBody(w, r, id, &in) {
		return
	}
	if in.Version != 1 || in.RequestID != id || !profile.SafeID(in.ID) {
		reject(w, 400, id, "invalid_request", "identity", "version and safe client identity required")
		return
	}
	if _, err := s.store.Client(); err == nil {
		s.diskReply(w, id, nil, control.ErrRevision)
		return
	}
	out, err := s.store.InitClient(in.ID, "")
	s.diskReply(w, id, out, err)
}

// reportImport accepts fixed-name base64 file bytes, not arbitrary paths or trusted derived summaries.
func (s *Server) reportImport(w http.ResponseWriter, r *http.Request, id string) {
	var in struct {
		Version   int               `json:"schema_version"`
		RequestID string            `json:"request_id"`
		ID        string            `json:"report_id"`
		Files     map[string][]byte `json:"files"`
	}
	if !savedBody(w, r, id, &in) {
		return
	}
	if in.Version != 1 || in.RequestID != id || !profile.SafeID(in.ID) {
		reject(w, 400, id, "invalid_request", "identity", "version and safe report identity required")
		return
	}
	out, err := s.store.ImportReport(in.ID, in.Files)
	s.diskReply(w, id, out, err)
}

// boundedExport stops output growth before writing headers, preserving structured errors on exhaustion.
type boundedExport struct{ bytes.Buffer }

// Write enforces the serialized export cap including CSV expansion; no partial success is published.
func (b *boundedExport) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 16<<20 {
		return 0, errors.New("export budget exceeded")
	}
	return b.Buffer.Write(p)
}

// reportExport replays before emitting exact JSON/JSONL/CSV; queries cannot contain paths or bearer secrets.
func (s *Server) reportExport(w http.ResponseWriter, r *http.Request, id, target string) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	format := q.Get("format")
	if err != nil || len(q) != 1 || len(q["format"]) != 1 || format != "json" && format != "jsonl" && format != "csv" {
		reject(w, 400, id, "invalid_request", "format", "one explicit json, jsonl or csv format required")
		return
	}
	out, err := s.store.SavedReport(target)
	if err != nil {
		s.diskReply(w, id, nil, err)
		return
	}
	var b boundedExport
	if err = control.ExportReport(&b, out, format); err != nil {
		s.diskReply(w, id, nil, err)
		return
	}
	media := map[string]string{"json": "application/json", "jsonl": "application/x-ndjson", "csv": "text/csv; charset=utf-8"}
	w.Header().Set("Content-Type", media[format])
	w.Header().Set("Content-Disposition", "attachment; filename=\""+target+"."+format+"\"")
	w.Header().Set("X-Request-ID", id)
	w.WriteHeader(200)
	w.Write(b.Bytes())
}
