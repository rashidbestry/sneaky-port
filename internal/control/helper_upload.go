package control

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

const helperBinaryMax = 64 << 20

// validateHelperBinary bounds uploads and accepts only Linux x86-64 ELF executables from the dashboard bundle.
func validateHelperBinary(binary []byte) error {
	if len(binary) < 64 || len(binary) > helperBinaryMax || !bytes.Equal(binary[:4], []byte{0x7f, 'E', 'L', 'F'}) || binary[4] != 2 || binary[5] != 1 || binary[18] != 62 || binary[19] != 0 {
		return errors.New("dashboard requires its bundled Linux x86-64 server helper; rebuild with build-dashboard.ps1")
	}
	return nil
}

// SetHelperBinary supplies an immutable matching helper before serving; fixtures can provide isolated payloads.
// No remote path is accepted or used for managed uploads, and active sessions cannot change their binary.
func (d *SSHDashboard) SetHelperBinary(binary []byte) error {
	if err := validateHelperBinary(binary); err != nil {
		return err
	}
	d.gate.Lock()
	defer d.gate.Unlock()
	d.binary = bytes.Clone(binary)
	return nil
}

// helperUploadCommand receives exactly the bundled byte count, verifies its digest and starts a private helper.
// Only an exclusively created temporary folder is cleaned; existing installs, stores and services are untouched.
func helperUploadCommand(binary []byte) (string, error) {
	if err := validateHelperBinary(binary); err != nil {
		return "", err
	}
	hash := sha256.Sum256(binary)
	return fmt.Sprintf(`# sneaky-port dashboard launcher v1
LC_ALL=C
export LC_ALL
umask 077
# fail emits a fixed secret-free startup fault before private cleanup.
fail() {
  printf '{"schema_version":1,"startup_error":"%%s"}\n' "$1"
  exit 1
}
[ "$(uname -s)" = Linux ] || fail not_linux
case "$(uname -m)" in x86_64|amd64) ;; *) fail unsupported_architecture ;; esac
for tool in mktemp head sha256sum chmod rm rmdir; do
  command -v "$tool" >/dev/null 2>&1 || fail upload_tools_unavailable
done
owned=$(mktemp -d "${TMPDIR:-/tmp}/sneaky-port-helper.XXXXXXXXXX") || fail upload_storage_failed
trap 'rm -f -- "$owned/server"; rmdir -- "$owned"' EXIT
trap 'exit 1' HUP INT TERM
printf '%%s\n' '{"schema_version":1,"upload_ready":true}'
head -c %d > "$owned/server" || fail upload_storage_failed
digest=$(sha256sum "$owned/server") || fail upload_checksum_failed
[ "${digest%%%% *}" = '%s' ] || fail upload_checksum_failed
chmod 700 "$owned/server" || fail not_executable
"$owned/server" dashboard-server
`, len(binary), hex.EncodeToString(hash[:])), nil
}
