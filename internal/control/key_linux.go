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

// restrictDirectory keeps the application's owned directory accessible only to its owner.
func restrictDirectory(path string) error {
	if err := os.Chmod(path, 0700); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0700 {
		return errors.New("application directory permissions are not owner-only")
	}
	return nil
}

// replaceFile publishes an atomic same-filesystem replacement after the temporary file is synced.
func replaceFile(source, target string) error { return os.Rename(source, target) }
