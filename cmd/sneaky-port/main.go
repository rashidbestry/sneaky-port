// sneaky-port exposes foundation tools, authenticated readiness and finite UDP/TCP measurement.
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
		return errors.New("usage: sneaky-port adapters | validate | preflight | keygen | server-init | client-init | server-enroll | server-disable | server-export | client-import | server | client-ready | client-run | client-api | report-replay | report-export\nUse COMMAND -h for flags. client-run saves local evidence; client-api protects local run controls; dashboard remains pending")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(out)
	switch args[0] {
	case "client-api":
		return clientAPI(ctx, args, out)
	case "report-replay", "report-export":
		path := flags.String("report", "", "explicit local run directory")
		budget := flags.Uint64("max-bytes", 0, "explicit aggregate input byte cap (1..1073741824; not a heap cap)")
		format := flags.String("format", "", "explicit export format: json, jsonl or csv")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *path == "" || *budget == 0 || flags.NArg() != 0 || args[0] == "report-replay" && *format != "" || args[0] == "report-export" && *format != "json" && *format != "jsonl" && *format != "csv" {
			return errors.New("report-replay requires -report DIR -max-bytes N; report-export also requires -format json|jsonl|csv")
		}
		r, err := control.ReplayReport(*path, *budget)
		if err != nil {
			return err
		}
		if args[0] == "report-replay" {
			return writeJSON(out, r)
		}
		return control.ExportReport(out, r, *format)
	case "server-init", "client-init", "server-enroll", "server-disable", "server-export", "client-import", "server", "client-ready", "client-run":
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
		err = adapter.Default().Validate(p)
		if err != nil {
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
