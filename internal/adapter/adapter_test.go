package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
	"sneaky-port/internal/testutil"
)

// TestActualAdapters exercises real loopback sockets; it does not establish Linux/live support.
func TestActualAdapters(t *testing.T) {
	for _, id := range []string{"udp", "tcp"} {
		t.Run(id, func(t *testing.T) { // Apply the same binding/authenticated-data flow to each registered carrier.
			p := testutil.Plan()
			p.Adapter = id
			if id == "tcp" {
				p.Options = map[string]json.RawMessage{"tcp_no_delay": json.RawMessage("true")}
			}
			reserve, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			_, port, _ := net.SplitHostPort(reserve.Addr().String())
			p.DestinationPort, _ = strconv.Atoi(port)
			reserve.Close()
			factory, err := Default().Get(id)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			prepared, err := factory.Prepare(ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Close()
			if _, err = factory.Prepare(ctx, p); err == nil {
				t.Fatal("busy port silently substituted")
			}
			done := make(chan error, 1)
			go func() { // Receive one authenticated datagram/frame and return a receipt through the owned peer.
				server, err := prepared.Accept(ctx)
				if err != nil {
					done <- err
					return
				}
				defer server.Close()
				stop := context.AfterFunc(ctx, func() { // Interrupt a stalled fixture receive using only the owned socket.
					server.Close()
				})
				defer stop()
				auth := probe.Auth{Adapter: id}
				b, err := server.Read()
				if err != nil {
					done <- err
					return
				}
				frame, err := auth.Decode(b, factory.Describe().MaxFrame)
				if err != nil {
					done <- err
					return
				}
				frame.Kind = probe.Receipt
				frame.Content = nil
				frame.PeerEpoch = 2
				frame.Arrival = 10
				frame.ReceiptAttempt = 11
				b, err = auth.Encode(frame, factory.Describe().MaxFrame)
				if err == nil {
					_, err = server.Write(b, time.Second)
				}
				done <- err
			}()
			client, err := factory.Dial(ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			stop := context.AfterFunc(ctx, func() { // Bound the client's receive if the server fixture fails.
				client.Close()
			})
			defer stop()
			local, _, _ := net.SplitHostPort(client.Local())
			if local != "127.0.0.1" {
				t.Fatal("wrong local family/binding")
			}
			if client.Remote() != prepared.Address() {
				t.Fatal("wrong destination")
			}
			auth := probe.Auth{Adapter: id}
			frame := probe.Frame{Kind: probe.Data, Direction: 1, Lifecycle: 2, Sequence: 9, OriginEpoch: 1, Attempt: 3, Content: p.Phases[0].Client.Payload()}
			b, err := auth.Encode(frame, factory.Describe().MaxFrame)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = client.Write(b, time.Second); err != nil {
				t.Fatal(err)
			}
			b, err = client.Read()
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := auth.Decode(b, factory.Describe().MaxFrame)
			if err != nil || receipt.Kind != probe.Receipt || receipt.Sequence != frame.Sequence || receipt.Attempt != frame.Attempt || !bytes.Equal(receipt.Content, []byte{}) {
				t.Fatalf("receipt %+v %v", receipt, err)
			}
			if err = <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

// extensionFactory tests shared descriptor/option dispatch without adding a production carrier.
type extensionFactory struct{ base Factory }

// Describe exposes the test adapter with its own typed option contract.
func (f extensionFactory) Describe() Descriptor {
	d := f.base.Describe()
	d.ID = "test-extension"
	d.Options = map[string]string{"enabled": "required boolean"}
	return d
}

// Validate interprets only declared extension options, then delegates common raw limits.
func (f extensionFactory) Validate(p profile.Plan) error {
	if string(p.Options["enabled"]) != "true" || len(p.Options) != 1 {
		return &net.AddrError{Err: "invalid test extension option", Addr: "enabled"}
	}
	p.Adapter = "udp"
	p.Options = map[string]json.RawMessage{}
	return f.base.Validate(p)
}

// Prepare retains the owned-listener contract for the test extension.
func (f extensionFactory) Prepare(ctx context.Context, p profile.Plan) (Prepared, error) {
	p.Adapter = "udp"
	p.Options = map[string]json.RawMessage{}
	return f.base.Prepare(ctx, p)
}

// Dial retains the owned-connection contract for the test extension.
func (f extensionFactory) Dial(ctx context.Context, p profile.Plan) (Transport, error) {
	p.Adapter = "udp"
	p.Options = map[string]json.RawMessage{}
	return f.base.Dial(ctx, p)
}

// TestRegistryExtension checks common discovery/validation and duplicate/unknown rejection.
func TestRegistryExtension(t *testing.T) {
	r := Default()
	udp, _ := r.Get("udp")
	f := extensionFactory{udp}
	if err := r.Register(f); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(f); err == nil {
		t.Fatal("duplicate registry ID accepted")
	}
	p := testutil.Plan()
	p.Adapter = "test-extension"
	p.Options = map[string]json.RawMessage{"enabled": json.RawMessage("true")}
	registered, err := r.Get(p.Adapter)
	if err != nil || registered.Validate(p) != nil {
		t.Fatal("shared extension validation failed")
	}
	if len(r.Describe()) != 3 {
		t.Fatal("extension not enumerated")
	}
	if _, err = r.Get("tls"); err == nil {
		t.Fatal("unimplemented carrier silently substituted")
	}
}
