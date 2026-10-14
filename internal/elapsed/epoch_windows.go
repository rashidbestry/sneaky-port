//go:build windows

package elapsed

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// newPlatform selects Windows QPC once; unsupported/unavailable counters fail the run.
func newPlatform() (*Epoch, error) {
	dll := syscall.NewLazyDLL("kernel32.dll")
	counter := dll.NewProc("QueryPerformanceCounter")
	frequency := dll.NewProc("QueryPerformanceFrequency")
	if err := counter.Find(); err != nil {
		return nil, err
	}
	if err := frequency.Find(); err != nil {
		return nil, err
	}
	var hz int64
	ok, _, err := frequency.Call(uintptr(unsafe.Pointer(&hz)))
	if ok == 0 || hz <= 0 {
		return nil, fmt.Errorf("QueryPerformanceFrequency unavailable: %w", errors.Join(err, errors.New("invalid frequency")))
	}
	return counterEpoch("Windows QueryPerformanceCounter", uint64(hz), func() (int64, error) { // Capture only QPC ticks; no runtime InterruptTime or wall-clock fallback.
		var ticks int64
		ok, _, err := counter.Call(uintptr(unsafe.Pointer(&ticks)))
		if ok == 0 {
			return 0, fmt.Errorf("QueryPerformanceCounter failed: %w", err)
		}
		return ticks, nil
	})
}
