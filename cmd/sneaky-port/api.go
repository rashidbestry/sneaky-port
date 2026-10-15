package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"sneaky-port/internal/control"
	"sneaky-port/internal/localapi"
	"sneaky-port/internal/profile"
)

// clientAPI loads optional explicit configuration or compiled defaults before serving protected finite run controls.
// Bootstrap output contains the local bearer secret; it must not be published or put into a URL.
func clientAPI(ctx context.Context, args []string, out io.Writer) error {
	f := flag.NewFlagSet("client-api", flag.ContinueOnError)
	f.SetOutput(out)
	dir := f.String("data-dir", "", "optional client data directory; default is the user's sneaky-port/client application folder")
	path := f.String("profile", "", "optional YAML/JSON profile; omitted uses built-in starter settings")
	listen := f.String("listen", "127.0.0.1:0", "literal loopback HTTP IP:port (0 selects a free port)")
	shutdown := f.Duration("shutdown-timeout", 10*time.Second, "positive finite API/run cleanup budget")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if *shutdown <= 0 || f.NArg() != 0 {
		return errors.New("client-api requires a positive shutdown timeout and rejects positional arguments")
	}
	p, err := loadProfile(*path)
	if err != nil {
		return err
	}
	if *dir == "" {
		*dir, err = defaultClientDirectory()
		if err != nil {
			return err
		}
	}
	store, err := control.OpenStore(*dir)
	if err != nil {
		return err
	}
	var signer ssh.Signer
	id, err := store.Client()
	if err == nil {
		signer, err = loadSigner(id.KeyRef)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	remote := control.NewSSHDashboard(store, signer)
	api, err := localapi.ListenWithStore(*listen, p, store, func(runCtx context.Context, frozen profile.Plan) (control.RunResult, error) { // Reuse SSH trust/reservation, independent timing, persistence and reconciliation.
		if remote.Status().State == "connected" || strings.HasPrefix(frozen.PairingRef, "ssh-") {
			return remote.Run(runCtx, frozen)
		}
		key := signer
		if key == nil {
			identity, e := store.Client()
			if e != nil {
				return control.RunResult{}, e
			}
			key, e = control.LoadKey(identity.KeyRef, nil)
			if e != nil {
				return control.RunResult{}, e
			}
		}
		return control.Run(runCtx, store, key, frozen)
	})
	if err != nil {
		return err
	}
	api.EnableSSH(remote)
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
