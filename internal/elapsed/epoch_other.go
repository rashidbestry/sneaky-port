//go:build !windows

package elapsed

import "time"

// newPlatform preserves Go's local monotonic domain on non-Windows platforms.
func newPlatform() (*Epoch, error) {
	e, err := FromTime(time.Now())
	if err == nil {
		e.metadata.Source = "Go host-local monotonic time.Since"
	}
	return e, err
}
