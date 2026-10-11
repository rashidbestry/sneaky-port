package control

import (
	"errors"
	"os"
)

// createPrivate creates only a new owner-only file, never replacing an existing identity.
func createPrivate(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
}

// restrictKey establishes and verifies owner-only permissions before secret bytes are written.
func restrictKey(f *os.File) error {
	if err := f.Chmod(0600); err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0600 {
		return errors.New("private key permissions are not owner-only")
	}
	return nil
}
