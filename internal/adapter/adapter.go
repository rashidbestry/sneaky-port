// Package adapter owns real transport sockets and compile-time descriptor factories.
package adapter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"sneaky-port/internal/platform"
	"sneaky-port/internal/probe"
	"sneaky-port/internal/profile"
)

// Descriptor describes a stable shared contract without protocol-specific UI switches.
type Descriptor struct {
	ID           string            `json:"adapter_id"`
	Version      int               `json:"adapter_contract_version"`
	MaxFrame     int               `json:"max_frame_bytes"`
	FramingBytes int               `json:"framing_bytes"` // Fixed transport bytes beyond each authenticated message.
	Families     []string          `json:"address_families"`
	Platforms    []string          `json:"platforms"`
	Options      map[string]string `json:"options"`
}

// Transport owns one established session and its physical outer-marking state.
// Read strips fixed framing and returns one complete authenticated message; Write counts framing bytes.
// Partial-read errors may expose consumed bytes through ReadBytes() int; Close must unblock owned I/O.
type Transport interface {
	Read() ([]byte, error)
	SetReadDeadline(time.Time) error
	Write([]byte, time.Duration) (int, error)
	Mark(int) (platform.Marking, error)
	Local() string
	Remote() string
	Close() error
}

// Prepared owns the server listener until an authenticated binding selects a peer.
type Prepared interface {
	Accept(context.Context) (Transport, error)
	Reject(Transport) error
	Address() string
	Close() error
}

// Factory supplies transport-specific validation, preparation and connection creation.
type Factory interface {
	Describe() Descriptor
	Validate(profile.Plan) error
	Prepare(context.Context, profile.Plan) (Prepared, error)
	Dial(context.Context, profile.Plan) (Transport, error)
}

// Registry provides ordered descriptors and rejects duplicate or incompatible registration.
type Registry struct{ entries map[string]Factory }

// NewRegistry starts an isolated registry; tests can add adapters without modifying the engine.
func NewRegistry() *Registry { return &Registry{entries: map[string]Factory{}} }

// Register binds one immutable descriptor/factory identity at startup.
func (r *Registry) Register(f Factory) error {
	d := f.Describe()
	if !profile.SafeID(d.ID) || d.Version != 1 || d.MaxFrame < probe.Overhead || d.MaxFrame > probe.TCPMax || d.FramingBytes < 0 || d.FramingBytes > probe.TCPMax {
		return errors.New("invalid adapter descriptor")
	}
	if _, ok := r.entries[d.ID]; ok {
		return errors.New("duplicate adapter ID")
	}
	r.entries[d.ID] = f
	return nil
}

// Get resolves only registered adapters; there is no silent carrier fallback.
func (r *Registry) Get(id string) (Factory, error) {
	f, ok := r.entries[id]
	if !ok {
		return nil, fmt.Errorf("unsupported adapter %q", id)
	}
	return f, nil
}

// Validate enforces common profile and advertised capability limits before factory-specific options.
// Unsupported identities, versions, families, platforms or frame sizes never substitute a carrier.
func (r *Registry) Validate(p profile.Plan) error {
	if err := p.Validate(); err != nil {
		return err
	}
	f, err := r.Get(p.Adapter)
	if err != nil {
		return err
	}
	d := f.Describe()
	if p.Contract != d.Version || !slices.Contains(d.Families, p.Family) || !slices.Contains(d.Platforms, runtime.GOOS) {
		return errors.New("unsupported adapter contract, address family or platform")
	}
	for name := range p.Options {
		if _, ok := d.Options[name]; !ok {
			return errors.New("undeclared adapter option")
		}
	}
	for _, ph := range p.Phases {
		if ph.Client.ContentBytes > d.MaxFrame-probe.Overhead || ph.Server.ContentBytes > d.MaxFrame-probe.Overhead {
			return errors.New("content exceeds advertised adapter frame limit")
		}
	}
	return f.Validate(p)
}

// Describe returns stable descriptor order for API, configuration and reports.
func (r *Registry) Describe() []Descriptor {
	out := make([]Descriptor, 0, len(r.entries))
	for _, f := range r.entries {
		out = append(out, f.Describe())
	}
	sort.Slice(out, func(i, j int) bool { // Keep registry enumeration independent of Go map order.
		return out[i].ID < out[j].ID
	})
	return out
}

// Default registers the real UDP and framed TCP implementations at compile time.
func Default() *Registry {
	r := NewRegistry()
	for _, f := range []Factory{socketFactory{false}, socketFactory{true}} {
		if err := r.Register(f); err != nil {
			panic(err)
		}
	}
	return r
}

// socketFactory selects one concrete standard-library transport behind the common interface.
type socketFactory struct{ stream bool }

// Describe exposes raw carrier limits and the explicit TCP_NODELAY setting.
func (f socketFactory) Describe() Descriptor {
	id, max := "udp", probe.UDPMax
	options := map[string]string{}
	if f.stream {
		id, max = "tcp", probe.TCPMax
		options["tcp_no_delay"] = "required boolean"
	}
	framing := 0
	if f.stream {
		framing = 4
	}
	return Descriptor{ID: id, Version: 1, MaxFrame: max, FramingBytes: framing, Families: []string{"ipv4", "ipv6"}, Platforms: []string{"windows", "linux"}, Options: options}
}

// Validate checks carrier content and options before either endpoint opens sockets.
func (f socketFactory) Validate(p profile.Plan) error {
	if err := p.Validate(); err != nil {
		return err
	}
	d := f.Describe()
	if p.Adapter != d.ID || p.Contract != d.Version {
		return errors.New("adapter contract mismatch")
	}
	if f.stream {
		if len(p.Options) != 1 {
			return errors.New("TCP requires only tcp_no_delay")
		}
		v, ok := p.Options["tcp_no_delay"]
		if !ok || string(v) != "true" && string(v) != "false" {
			return errors.New("tcp_no_delay requires explicit boolean")
		}
	} else if len(p.Options) != 0 {
		return errors.New("UDP has no adapter options")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return errors.New("unsupported platform")
	}
	for _, ph := range p.Phases {
		for _, dir := range []profile.Direction{ph.Client, ph.Server} {
			if dir.ContentBytes > d.MaxFrame-probe.Overhead {
				return errors.New("content exceeds adapter frame limit")
			}
		}
	}
	return nil
}

// network selects the requested address family explicitly for both roles.
func (f socketFactory) network(p profile.Plan) string {
	kind := f.Describe().ID
	if p.Family == "ipv4" {
		return kind + "4"
	}
	return kind + "6"
}

// Prepare binds the server's requested destination port without substituting another.
func (f socketFactory) Prepare(ctx context.Context, p profile.Plan) (Prepared, error) {
	if err := f.Validate(p); err != nil {
		return nil, err
	}
	address := net.JoinHostPort(p.ServerAddress, strconv.Itoa(p.DestinationPort))
	if f.stream {
		lc := net.ListenConfig{}
		ln, err := lc.Listen(ctx, f.network(p), address)
		if err != nil {
			return nil, err
		}
		return &tcpPrepared{listener: ln.(*net.TCPListener), noDelay: string(p.Options["tcp_no_delay"]) == "true"}, nil
	}
	a, err := net.ResolveUDPAddr(f.network(p), address)
	if err != nil {
		return nil, err
	}
	c, err := net.ListenUDP(f.network(p), a)
	if err != nil {
		return nil, err
	}
	return &udpPrepared{conn: c}, nil
}

// Dial owns the client's explicit or auto-assigned source port and connected peer.
func (f socketFactory) Dial(ctx context.Context, p profile.Plan) (Transport, error) {
	if err := f.Validate(p); err != nil {
		return nil, err
	}
	port := 0
	if p.SourcePort != "auto" {
		port, _ = strconv.Atoi(p.SourcePort)
	}
	d := net.Dialer{}
	if f.stream {
		d.LocalAddr = &net.TCPAddr{Port: port}
	} else {
		d.LocalAddr = &net.UDPAddr{Port: port}
	}
	c, err := d.DialContext(ctx, f.network(p), net.JoinHostPort(p.ServerAddress, strconv.Itoa(p.DestinationPort)))
	if err != nil {
		return nil, err
	}
	if tcp, ok := c.(*net.TCPConn); ok {
		if err = tcp.SetNoDelay(string(p.Options["tcp_no_delay"]) == "true"); err != nil {
			c.Close()
			return nil, err
		}
	}
	return &socket{conn: c, stream: f.stream}, nil
}

// socket serializes writes while allowing independent receive and scheduling goroutines.
type socket struct {
	conn   net.Conn
	stream bool
	udp    *net.UDPConn
	peer   *net.UDPAddr
	first  []byte
	mu     sync.Mutex
	mark   *platform.Marker
}

// SetReadDeadline bounds authentication reads without substituting a peer or transport.
func (s *socket) SetReadDeadline(t time.Time) error { return s.conn.SetReadDeadline(t) }

// Interrupt stops owned reads/writes at a cutoff while the coordinator retains the peer connection until stop.
// This prevents an earlier local grace cutoff from creating a premature peer TCP EOF.
func (s *socket) Interrupt() error { return s.conn.SetDeadline(time.Now()) }

// Read returns complete raw datagrams/frames; wrong UDP tuples are discarded, not migrated.
func (s *socket) Read() ([]byte, error) {
	if s.first != nil {
		b := s.first
		s.first = nil
		return b, nil
	}
	if s.stream {
		return probe.ReadTCP(s.conn)
	}
	b := make([]byte, probe.UDPMax+1)
	for {
		if s.udp != nil {
			n, peer, err := s.udp.ReadFromUDP(b)
			if err != nil {
				return nil, err
			}
			if peer.String() != s.peer.String() {
				continue
			}
			if n > probe.UDPMax {
				return nil, errors.New("oversized UDP frame")
			}
			return b[:n], nil
		}
		n, err := s.conn.Read(b)
		if n > probe.UDPMax {
			return nil, errors.New("oversized UDP frame")
		}
		return b[:n], err
	}
}

// Write applies a bounded socket deadline and preserves partial stream byte counts.
func (s *socket) Write(b []byte, budget time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.conn.SetWriteDeadline(time.Now().Add(budget)); err != nil {
		return 0, err
	}
	if s.stream {
		return probe.WriteTCP(s.conn, b)
	}
	if s.udp != nil {
		return s.udp.WriteToUDP(b, s.peer)
	}
	return s.conn.Write(b)
}

// Mark updates only this session's physical outgoing socket; no global policy is changed.
func (s *socket) Mark(dscp int) (platform.Marking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mark == nil {
		s.mark = platform.New(s.conn)
	}
	return s.mark.Set(dscp)
}

// Local returns the actual owned local binding rather than only requested settings.
func (s *socket) Local() string { return s.conn.LocalAddr().String() }

// Remote returns the selected session peer, including observed server-side UDP NAT bindings.
func (s *socket) Remote() string {
	if s.peer != nil {
		return s.peer.String()
	}
	return s.conn.RemoteAddr().String()
}

// Close interrupts blocked reads/writes and releases only the owned socket/QoS handle.
func (s *socket) Close() error {
	err := s.conn.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mark != nil {
		s.mark.Close()
	}
	return err
}

type tcpPrepared struct {
	listener *net.TCPListener
	noDelay  bool
}

// Accept polls cancellation and initializes the explicitly requested TCP behavior.
func (p *tcpPrepared) Accept(ctx context.Context) (Transport, error) {
	for {
		p.listener.SetDeadline(time.Now().Add(100 * time.Millisecond))
		c, err := p.listener.AcceptTCP()
		if err == nil {
			if err = c.SetNoDelay(p.noDelay); err != nil {
				c.Close()
				return nil, err
			}
			return &socket{conn: c, stream: true}, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
			return nil, err
		}
	}
}

// Address exposes the effective server listener.
func (p *tcpPrepared) Address() string { return p.listener.Addr().String() }

// Close releases this listener, leaving unrelated services untouched.
func (p *tcpPrepared) Close() error { return p.listener.Close() }

// Reject releases only the unauthenticated TCP candidate, preserving the owned listener.
func (p *tcpPrepared) Reject(t Transport) error { return t.Close() }

type udpPrepared struct{ conn *net.UDPConn }

// Accept returns a candidate tuple and first datagram; authentication precedes readiness.
func (p *udpPrepared) Accept(ctx context.Context) (Transport, error) {
	b := make([]byte, probe.UDPMax+1)
	for {
		p.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, peer, err := p.conn.ReadFromUDP(b)
		if err == nil {
			p.conn.SetReadDeadline(time.Time{})
			return &socket{conn: p.conn, udp: p.conn, peer: peer, first: append([]byte(nil), b[:n]...)}, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
			return nil, err
		}
	}
}

// Address exposes the actual UDP destination binding.
func (p *udpPrepared) Address() string { return p.conn.LocalAddr().String() }

// Close releases the owned UDP listener and any session reading from it.
func (p *udpPrepared) Close() error { return p.conn.Close() }

// Reject discards a candidate without closing the shared UDP listener or selecting its tuple.
func (p *udpPrepared) Reject(t Transport) error { return p.conn.SetReadDeadline(time.Time{}) }
