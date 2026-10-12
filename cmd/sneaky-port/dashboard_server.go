package main

import (
	"context"
	"errors"
	"io"
	"os/signal"
	"runtime"
	"sneaky-port/internal/control"
	"syscall"
)

// dashboardServer exposes the disposable helper only on Linux inside the user's normal SSH account.
func dashboardServer(ctx context.Context, in io.ReadCloser, out io.Writer) error {
	if runtime.GOOS != "linux" {
		return errors.New("dashboard server helper requires Linux")
	}
	owned, stop := signal.NotifyContext(ctx, syscall.Signal(1)) // Linux SIGHUP from an SSH logout must run owned cleanup.
	defer stop()
	return control.DashboardServer(owned, in, out)
}
