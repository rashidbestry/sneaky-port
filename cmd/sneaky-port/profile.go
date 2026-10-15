package main

import (
	"os"

	"sneaky-port/internal/profile"
)

// loadProfile uses compiled startup settings only when no path was supplied; explicit file errors never fall back.
// Complete normalized plans still pass the same validation before endpoint setup or API serving.
func loadProfile(path string) (profile.Plan, error) {
	if path == "" {
		p := profile.Default()
		return p, p.Validate()
	}
	file, err := os.Open(path)
	if err != nil {
		return profile.Plan{}, err
	}
	defer file.Close()
	return profile.Decode(file)
}
