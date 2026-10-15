package main

import (
	"os"
	"path/filepath"
)

// defaultClientDirectory gives no-argument startup a stable per-user store independent of the executable or working directory.
// Explicit -data-dir still selects existing stores; startup never migrates or overwrites another directory.
func defaultClientDirectory() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sneaky-port", "client"), nil
}
