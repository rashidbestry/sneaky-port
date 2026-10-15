package main

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"sneaky-port/internal/control"
)

const defaultServerListen = "0.0.0.0:22022"

// defaultServerDirectory keeps automatically generated identity/authorization state in the user's application directory.
// Test workloads remain client supplied; this directory is not an operator-written server configuration.
func defaultServerDirectory() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sneaky-port", "server"), nil
}

// serverAdvertiseAddress selects a same-family local address for a new server's public bundle, without changing routing.
// An explicit -advertise remains available for NAT/public addresses that local interface discovery cannot determine.
func serverAdvertiseAddress(bind string) (string, error) {
	ip := net.ParseIP(bind)
	if ip == nil {
		return "", errors.New("server listen requires a literal IP address")
	}
	if !ip.IsUnspecified() {
		return ip.String(), nil
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, address := range addresses {
			candidate, _, err := net.ParseCIDR(address.String())
			if err == nil && candidate.IsGlobalUnicast() && (candidate.To4() != nil) == (ip.To4() != nil) {
				return candidate.String(), nil
			}
		}
	}
	return "", errors.New("no active server address found; start after networking is ready or supply -advertise IP:PORT")
}

// loadOrInitServer reuses existing identity/policy unchanged, or initializes a new key and built-in policy on first start.
// Initial authorization remains empty until trusted enrollment; no embedded password/key or anonymous first-client trust is introduced.
func loadOrInitServer(store *control.Store, listen, advertise string) (control.ServerIdentity, error) {
	identity, err := store.Server()
	if err == nil {
		if listen != "" && listen != identity.Policy.ControlListen || advertise != "" && advertise != net.JoinHostPort(identity.Bundle.ControlAddress, strconv.Itoa(identity.Bundle.ControlPort)) {
			return identity, errors.New("server identity already initialized with different bindings; existing state was preserved")
		}
		return identity, nil
	}
	if !os.IsNotExist(err) {
		return identity, err
	}
	if listen == "" {
		listen = defaultServerListen
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return identity, err
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return identity, err
	}
	if advertise == "" {
		address, err := serverAdvertiseAddress(host)
		if err != nil {
			return identity, err
		}
		advertise = net.JoinHostPort(address, port)
	}
	address, advertisedPort, err := net.SplitHostPort(advertise)
	if err != nil || advertisedPort != port {
		return identity, errors.New("advertised endpoint must use the server's listening port")
	}
	if _, err = store.InitServer("server", address, n, control.DefaultServerPolicy(listen, host), ""); err != nil {
		return identity, err
	}
	return store.Server()
}
