//go:build !windows

package control

import "os"

// openStoreObject preserves ordinary read-only snapshot semantics on non-Windows platforms.
func openStoreObject(path string) (*os.File, error) { return os.Open(path) }
