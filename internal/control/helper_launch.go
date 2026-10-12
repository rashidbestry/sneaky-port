package control

import (
	"errors"
	"strings"
	"sync"
	"unicode"
)

// helperLaunchCommand resolves a PATH install or an explicit Linux path without interpreting operator text.
// Only the selected executable is run; lookup failures return fixed secret-free JSON on SSH stdout.
func helperLaunchCommand(executable string) (string, error) {
	if len(executable) > 1024 || executable != "" && !strings.HasPrefix(executable, "/") {
		return "", errors.New("Server executable must be an absolute Linux file path or blank for SSH PATH")
	}
	for _, character := range executable {
		if unicode.IsControl(character) {
			return "", errors.New("Server executable path cannot contain control characters")
		}
	}
	selection := "program=$(command -v sneaky-port 2>/dev/null) || program=''\n"
	if executable != "" {
		selection = "program='" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'\n"
	}
	return "# sneaky-port dashboard launcher v1\nLC_ALL=C\nexport LC_ALL\n" + selection + `if [ -z "$program" ] || [ ! -f "$program" ]; then
  printf '%s\n' '{"schema_version":1,"startup_error":"missing_executable"}'
  exit 127
fi
if [ ! -x "$program" ]; then
  printf '%s\n' '{"schema_version":1,"startup_error":"not_executable"}'
  exit 126
fi
if [ "$(uname -s)" != Linux ]; then
  printf '%s\n' '{"schema_version":1,"startup_error":"not_linux"}'
  exit 125
fi
exec "$program" dashboard-server`, nil
}

// helperLaunchFailure recognizes only the launcher's fixed structured faults before helper negotiation.
func helperLaunchFailure(body []byte) error {
	var fault struct {
		Version int    `json:"schema_version"`
		Code    string `json:"startup_error"`
	}
	if DecodeObject(body, &fault) != nil || fault.Version != 1 || fault.Code == "" {
		return nil
	}
	switch fault.Code {
	case "missing_executable":
		return errors.New("server executable not found; set Server executable to the full Linux path of the copied build, or install sneaky-port in SSH PATH")
	case "not_executable":
		return errors.New("server file is not executable by the SSH account; check execute permission and directory access")
	case "not_linux":
		return errors.New("measurement server helper requires Linux")
	case "unsupported_architecture":
		return errors.New("automatic helper setup currently requires a Linux x86-64 server; ARM support is deferred")
	case "upload_tools_unavailable":
		return errors.New("Linux SSH account needs standard shell tools: mktemp, head, sha256sum, chmod, rm and rmdir")
	case "upload_storage_failed":
		return errors.New("server could not create private helper storage; check temporary-directory access and free disk space")
	case "upload_checksum_failed":
		return errors.New("server helper upload checksum failed; reconnect to retry")
	default:
		return errors.New("server executable lookup returned an unsupported startup error")
	}
}

// helperDiagnostics drains at most 8 KiB of SSH stderr; arbitrary remote text never reaches the UI.
type helperDiagnostics struct {
	mu   sync.Mutex
	text []byte
}

// Write bounds retained stderr while acknowledging discarded bytes so remote writers never back up.
func (d *helperDiagnostics) Write(bytes []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	remaining := 8192 - len(d.text)
	if len(bytes) < remaining {
		remaining = len(bytes)
	}
	if remaining > 0 {
		d.text = append(d.text, bytes[:remaining]...)
	}
	return len(bytes), nil
}

// Text takes a private diagnostic snapshot for fixed classification only, without exposing paths/secrets.
func (d *helperDiagnostics) Text() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.ToLower(string(d.text))
}

// helperStartupFailure distinguishes known runtime/old-build failures without reflecting SSH stderr.
func helperStartupFailure(stderr string, decodeErr error) error {
	if strings.Contains(stderr, "unimplemented or unknown command") || strings.Contains(stderr, "unknown command") {
		return errors.New("server executable does not support dashboard-server; copy the latest Linux server build")
	}
	if strings.Contains(stderr, "exec format error") || strings.Contains(stderr, "cannot execute binary file") {
		return errors.New("server executable has the wrong operating system or CPU architecture; use the Linux x86-64 build")
	}
	if strings.Contains(stderr, "permission denied") {
		return errors.New("server helper permission denied; check executable/directory access and private temporary storage")
	}
	if strings.Contains(stderr, "no space left on device") {
		return errors.New("server helper could not create private temporary storage; free disk space before retrying")
	}
	if decodeErr != nil && strings.Contains(decodeErr.Error(), "invalid character") {
		return errors.New("SSH startup wrote unexpected text to stdout; keep noninteractive login output silent and use a matching Linux executable")
	}
	return errors.New("server helper exited or timed out before startup; check the selected Linux executable and server-side startup logs")
}
