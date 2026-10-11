// sneaky-port exposes foundation tools and authenticated endpoint readiness; measurement orchestration is pending.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"sneaky-port/internal/adapter"
	"sneaky-port/internal/control"
	"sneaky-port/internal/measure"
	"sneaky-port/internal/profile"
)

// main maps cancellation and explicit command errors to a nonzero process exit.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run dispatches only implemented operations; no measurement result is fabricated.
func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: sneaky-port adapters | validate | preflight | keygen | server-init | client-init | server-enroll | server-disable | server-export | client-import | server | client-ready\nUse COMMAND -h for flags. client-ready verifies readiness only; measurement and dashboard remain pending")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(out)
	switch args[0] {
	case "server-init", "client-init", "server-enroll", "server-disable", "server-export", "client-import", "server", "client-ready":
		return coordination(ctx, args, out)
	case "adapters":
		if len(args) != 1 {
			return errors.New("adapters takes no arguments")
		}
		return writeJSON(out, adapter.Default().Describe())
	case "validate":
		path := flags.String("profile", "", "explicit YAML/JSON profile path")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *path == "" || flags.NArg() != 0 {
			return errors.New("validate requires -profile FILE")
		}
		f, err := os.Open(*path)
		if err != nil {
			return err
		}
		defer f.Close()
		p, err := profile.Decode(f)
		if err != nil {
			return err
		}
		factory, err := adapter.Default().Get(p.Adapter)
		if err != nil {
			return err
		}
		if err = factory.Validate(p); err != nil {
			return err
		}
		hash, err := p.Hash()
		if err != nil {
			return err
		}
		return writeJSON(out, struct {
			Version  int          `json:"schema_version"`
			Status   string       `json:"status"`
			PlanHash string       `json:"plan_hash"`
			Plan     profile.Plan `json:"plan"`
		}{1, "valid; no sockets opened or runtime support established", hex.EncodeToString(hash[:]), p})
	case "preflight":
		duration := flags.String("duration-ns", "", "explicit local duration in nanoseconds")
		interval := flags.String("interval-ns", "", "explicit timer interval in nanoseconds")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		d, err := strconv.ParseUint(*duration, 10, 64)
		if err != nil {
			return err
		}
		i, err := strconv.ParseUint(*interval, 10, 64)
		if err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected preflight arguments")
		}
		samples, err := measure.Preflight(ctx, d, i)
		if err != nil {
			return err
		}
		return writeJSON(out, struct {
			Version int            `json:"schema_version"`
			Kind    string         `json:"kind"`
			Clock   string         `json:"clock"`
			Samples []measure.Wake `json:"samples"`
		}{1, "local_timer_preflight", "Go monotonic elapsed time; clock resolution/uncertainty not calibrated", samples})
	case "keygen":
		path := flags.String("output", "", "new application-owned private key file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *path == "" || flags.NArg() != 0 {
			return errors.New("keygen requires -output NEW_FILE")
		}
		public, err := control.GenerateKey(*path)
		if err != nil {
			return err
		}
		return writeJSON(out, public)
	default:
		return fmt.Errorf("unimplemented or unknown command %q", args[0])
	}
}

// writeJSON emits browser-safe quantities using each model's string representations.
func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
