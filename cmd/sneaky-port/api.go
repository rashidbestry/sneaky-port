package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"time"

	"sneaky-port/internal/control"
	"sneaky-port/internal/localapi"
	"sneaky-port/internal/profile"
)

// clientAPI loads explicit local configuration/key before serving protected finite run controls.
// Bootstrap output contains the local bearer secret; it must not be published or put into a URL.
func clientAPI(ctx context.Context, args []string, out io.Writer) error {
	f := flag.NewFlagSet("client-api", flag.ContinueOnError)
	f.SetOutput(out)
	dir := f.String("data-dir", "", "explicit application-owned client data directory")
	path := f.String("profile", "", "explicit launch-time frozen YAML/JSON profile")
	listen := f.String("listen", "127.0.0.1:0", "literal loopback HTTP IP:port (0 selects a free port)")
	shutdown := f.Duration("shutdown-timeout", 10*time.Second, "positive finite API/run cleanup budget")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if *dir == "" || *path == "" || *shutdown <= 0 || f.NArg() != 0 {
		return errors.New("client-api requires -data-dir, -profile and a positive shutdown timeout")
	}
	file, err := os.Open(*path)
	if err != nil {
		return err
	}
	p, err := profile.Decode(file)
	file.Close()
	if err != nil {
		return err
	}
	store, err := control.OpenStore(*dir)
	if err != nil {
		return err
	}
	id, err := store.Client()
	if err != nil {
		return err
	}
	signer, err := loadSigner(id.KeyRef)
	if err != nil {
		return err
	}
	api, err := localapi.Listen(*listen, p, func(runCtx context.Context, frozen profile.Plan) (control.RunResult, error) { // Reuse SSH trust/reservation, independent timing, persistence and reconciliation.
		return control.Run(runCtx, store, signer, frozen)
	})
	if err != nil {
		return err
	}
	if err = writeJSON(out, struct {
		Version   int    `json:"schema_version"`
		URL       string `json:"api_url"`
		Token     string `json:"session_token"`
		ProfileID string `json:"profile_id"`
		Revision  string `json:"profile_revision"`
	}{1, api.URL(), api.Token(), p.ID, api.Revision()}); err != nil {
		join, cancel := context.WithTimeout(context.Background(), *shutdown)
		defer cancel()
		return errors.Join(err, api.Close(join))
	}
	return api.Serve(ctx, *shutdown)
}
