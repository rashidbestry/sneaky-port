package control

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
)

// DashboardServerRequest enrolls only the local public identity in a disposable helper store.
type DashboardServerRequest struct {
	Version int        `json:"schema_version"`
	Address string     `json:"server_address"`
	Client  Enrollment `json:"client_enrollment"`
}

// DashboardServerReply advertises the private control listener, not a replacement data transport.
type DashboardServerReply struct {
	Version int          `json:"schema_version"`
	Bundle  Bundle       `json:"bundle"`
	Policy  ServerPolicy `json:"policy"`
}

// DashboardServer owns a disposable keyed measurement helper beneath an authenticated Linux SSH exec.
// Closing its stdin ends the listener and joins all runs; existing stores/services are untouched.
func DashboardServer(ctx context.Context, in io.ReadCloser, out io.Writer) (retErr error) {
	defer in.Close()
	reader := bufio.NewReader(io.LimitReader(in, FrameMax+1))
	line, err := reader.ReadBytes('\n')
	var q DashboardServerRequest
	if err != nil || len(line) > FrameMax || DecodeObject(line, &q) != nil || q.Version != 1 || net.ParseIP(q.Address) == nil || net.ParseIP(q.Address).IsUnspecified() || validateEnrollment(q.Client) != nil || !q.Client.Enabled {
		return errors.New("invalid dashboard helper enrollment")
	}
	root, err := os.MkdirTemp("", "sneaky-port-dashboard-")
	if err != nil {
		return err
	}
	defer func() { /* Remove only the exclusively created helper directory; report failed owned cleanup. */
		retErr = errors.Join(retErr, os.RemoveAll(root))
	}()
	store, err := OpenStore(root)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	bind := "0.0.0.0"
	if net.ParseIP(q.Address).To4() == nil {
		bind = "::"
	}
	policy := ServerPolicy{Version: 1, ControlListen: listener.Addr().String(), DataBindIP: bind, PortMin: 1, PortMax: 65535, SetupTimeout: 30_000_000_000, ShutdownTimeout: 30_000_000_000, AuthPoll: 1_000_000_000, MaxConnections: 4, MaxAuthTries: 3, MaxControlRequests: 1 << 20, MaxBindAttempts: 16, MaxQueueBytes: 1 << 30, MaxLedgerMessages: 1 << 20, MaxEventBytes: 1 << 30, MaxFileBytes: 1 << 30}
	bundle, err := store.InitServer("dashboard-server", q.Address, port, policy, "")
	if err != nil {
		return err
	}
	if err = store.Enroll(q.Client); err != nil {
		return err
	}
	identity, err := store.Server()
	if err != nil {
		return err
	}
	key, err := LoadKey(identity.KeyRef, nil)
	if err != nil {
		return err
	}
	server, err := NewServer(store, key)
	if err != nil {
		return err
	}
	owned, cancel := context.WithCancel(ctx)
	defer cancel()
	closed := context.AfterFunc(owned, func() { /* Interrupt SSH stdin when shutdown arrives before EOF. */ in.Close() })
	defer closed()
	readerDone := make(chan struct{})
	go func() { // A closed SSH stdin or unexpected further input stops this helper.
		defer close(readerDone)
		var next [1]byte
		reader.Read(next[:])
		cancel()
	}()
	defer func() { /* Join the helper's sole input watcher before deleting its owned store. */
		cancel()
		in.Close()
		<-readerDone
	}()
	if err = json.NewEncoder(out).Encode(DashboardServerReply{1, bundle, policy}); err != nil {
		return err
	}
	return server.Serve(owned, listener)
}

// controlDialerKey carries an owned authenticated control tunnel, never a measured data route.
type controlDialerKey struct{}

// WithControlDialer keeps a Linux SSH tunnel confined to control; protocol data remains direct.
func WithControlDialer(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error)) context.Context {
	return context.WithValue(ctx, controlDialerKey{}, dial)
}

// dialControl uses the direct path unless an owned SSH connector supplies a control tunnel.
func dialControl(ctx context.Context, network, address string) (net.Conn, error) {
	if dial, ok := ctx.Value(controlDialerKey{}).(func(context.Context, string, string) (net.Conn, error)); ok {
		return dial(ctx, network, address)
	}
	return (&net.Dialer{}).DialContext(ctx, network, address)
}
