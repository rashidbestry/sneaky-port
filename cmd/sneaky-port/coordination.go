package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"sneaky-port/internal/control"
	"sneaky-port/internal/profile"
)

// coordination dispatches explicit application-store, readiness and finite measurement operations.
func coordination(ctx context.Context, args []string, out io.Writer) error {
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(out)
	dir := f.String("data-dir", "", "explicit application-owned data directory")
	var id, key, address, policyPath, enrollmentPath, bundlePath, pairing, planPath, hold string
	var replace bool
	switch args[0] {
	case "server-init":
		f.StringVar(&id, "server-id", "", "explicit server ID")
		f.StringVar(&key, "key", "", "existing local private key reference; omitted generates a new key")
		f.StringVar(&address, "advertise", "", "public control IP:port")
		f.StringVar(&policyPath, "policy", "", "explicit JSON server policy")
	case "client-init":
		f.StringVar(&id, "client-id", "", "explicit client ID")
		f.StringVar(&key, "key", "", "existing local private key reference; omitted generates a new key")
	case "server-enroll":
		f.StringVar(&enrollmentPath, "enrollment", "", "trusted public enrollment JSON")
	case "server-disable":
		f.StringVar(&id, "client-id", "", "enrolled client ID to revoke")
	case "client-import":
		f.StringVar(&pairing, "pairing-id", "", "local pairing ID matching the profile")
		f.StringVar(&bundlePath, "bundle", "", "trusted public server bundle JSON")
		f.BoolVar(&replace, "replace", false, "explicitly replace existing host trust")
	case "client-ready", "client-run":
		f.StringVar(&planPath, "profile", "", "explicit YAML/JSON profile")
		if args[0] == "client-ready" {
			f.StringVar(&hold, "hold-ns", "", "explicit readiness-only hold duration, including zero")
		}
	case "server", "server-export":
	default:
		return fmt.Errorf("unknown coordination command %q", args[0])
	}
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if *dir == "" || f.NArg() != 0 {
		return errors.New("explicit -data-dir required; unexpected positional arguments rejected")
	}
	// Validate required inputs before creating the application directories.
	switch args[0] {
	case "server-init":
		if id == "" || address == "" || policyPath == "" {
			return errors.New("server-init requires -server-id, -advertise and -policy")
		}
	case "client-init", "server-disable":
		if id == "" {
			return errors.New("explicit client ID required")
		}
	case "server-enroll":
		if enrollmentPath == "" {
			return errors.New("-enrollment required")
		}
	case "client-import":
		if pairing == "" || bundlePath == "" {
			return errors.New("-pairing-id and -bundle required")
		}
	case "client-ready":
		if planPath == "" || hold == "" {
			return errors.New("-profile and explicit -hold-ns required")
		}
	case "client-run":
		if planPath == "" {
			return errors.New("client-run requires -profile")
		}
	case "server":
		if runtime.GOOS != "linux" {
			return errors.New("server deployment is supported on Linux; Windows exposes client tools only")
		}
	}
	store, err := control.OpenStore(*dir)
	if err != nil {
		return err
	}
	switch args[0] {
	case "server-init":
		var policy control.ServerPolicy
		if err = control.ReadObject(policyPath, &policy); err != nil {
			return err
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(port)
		if err != nil {
			return err
		}
		pass, err := keyPassphrase(key)
		if err != nil {
			return err
		}
		defer clear(pass)
		bundle, err := store.InitServer(id, host, n, policy, key, pass)
		if err != nil {
			return err
		}
		return writeJSON(out, bundle)
	case "client-init":
		pass, err := keyPassphrase(key)
		if err != nil {
			return err
		}
		defer clear(pass)
		enrollment, err := store.InitClient(id, key, pass)
		if err != nil {
			return err
		}
		return writeJSON(out, enrollment)
	case "server-export":
		id, err := store.Server()
		if err != nil {
			return err
		}
		return writeJSON(out, id.Bundle)
	case "server-enroll":
		var enrollment control.Enrollment
		if err = control.ReadObject(enrollmentPath, &enrollment); err != nil {
			return err
		}
		if err = store.Enroll(enrollment); err != nil {
			return err
		}
		return writeJSON(out, map[string]string{"status": "enrolled", "client_id": enrollment.ClientID})
	case "server-disable":
		if err = store.Disable(id); err != nil {
			return err
		}
		return writeJSON(out, map[string]string{"status": "disabled", "client_id": id})
	case "client-import":
		var bundle control.Bundle
		if err = control.ReadObject(bundlePath, &bundle); err != nil {
			return err
		}
		if err = store.ImportPairing(pairing, bundle, replace); err != nil {
			return err
		}
		return writeJSON(out, map[string]string{"status": "imported", "pairing_id": pairing, "host_key_fingerprint": bundle.HostFingerprint})
	case "server":
		id, err := store.Server()
		if err != nil {
			return err
		}
		signer, err := loadSigner(id.KeyRef)
		if err != nil {
			return err
		}
		server, err := control.NewServer(store, signer)
		if err != nil {
			return err
		}
		listenHost, _, _ := net.SplitHostPort(id.Policy.ControlListen)
		network := "tcp6"
		if net.ParseIP(listenHost).To4() != nil {
			network = "tcp4"
		}
		ln, err := net.Listen(network, id.Policy.ControlListen)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "embedded SSH listening on", ln.Addr())
		return server.Serve(ctx, ln)
	case "client-ready", "client-run":
		var n uint64
		if args[0] == "client-ready" {
			n, err = strconv.ParseUint(hold, 10, 63)
			if err != nil {
				return err
			}
		}
		file, err := os.Open(planPath)
		if err != nil {
			return err
		}
		defer file.Close()
		plan, err := profile.Decode(file)
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
		if args[0] == "client-run" {
			result, runErr := control.Run(ctx, store, signer, plan)
			if err = writeJSON(out, result); err != nil {
				return err
			}
			return runErr
		}
		result, err := control.Ready(ctx, store, signer, plan, time.Duration(n))
		if err != nil {
			return err
		}
		return writeJSON(out, result)
	}
	return errors.New("unsupported coordination operation")
}

// keyPassphrase prompts only for an encrypted explicit key reference, using the terminal without echo.
func keyPassphrase(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	_, err := control.LoadKey(path, nil)
	var missing *ssh.PassphraseMissingError
	if err == nil {
		return nil, nil
	}
	if !errors.As(err, &missing) {
		return nil, err
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New("encrypted key requires an interactive terminal passphrase prompt")
	}
	fmt.Fprint(os.Stderr, "Local SSH key passphrase: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return pass, err
}

// loadSigner unlocks an existing reference in memory and clears the prompt buffer after parsing.
func loadSigner(path string) (ssh.Signer, error) {
	pass, err := keyPassphrase(path)
	if err != nil {
		return nil, err
	}
	defer clear(pass)
	return control.LoadKey(path, pass)
}
