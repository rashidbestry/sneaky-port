package testutil

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// SSHFixture emulates ordinary password SSH/exec/forwarding, without claiming Linux sshd coverage.
type SSHFixture struct {
	IP              string
	Port            int
	Fingerprint     string
	Authentications atomic.Int32
}

// DashboardPayload supplies a bounded synthetic ELF header for channel fixtures, never a native executable.
func DashboardPayload() []byte {
	value := make([]byte, 256)
	copy(value, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	value[18] = 62
	return value
}

// StartSSH runs isolated real SSH authentication and channels around the actual helper callback.
func StartSSH(t *testing.T, helper func(context.Context, io.ReadCloser, io.Writer) error) *SSHFixture {
	if helper == nil {
		return StartSSHCommand(t, nil)
	}
	return StartSSHCommand(t, func(ctx context.Context, command string, ch ssh.Channel) error { /* Model upload framing/digest and invoke the actual helper; native tests execute the shell separately. */
		if strings.Contains(command, "upload_ready") {
			sizeMatch := regexp.MustCompile(`head -c ([0-9]+) >`).FindStringSubmatch(command)
			hashMatch := regexp.MustCompile(`= '([a-f0-9]{64})'`).FindStringSubmatch(command)
			if len(sizeMatch) != 2 || len(hashMatch) != 2 {
				return errors.New("invalid fixture upload command")
			}
			size, err := strconv.ParseInt(sizeMatch[1], 10, 64)
			if err != nil || size < 64 || size > 64<<20 {
				return errors.New("invalid fixture upload size")
			}
			io.WriteString(ch, "{\"schema_version\":1,\"upload_ready\":true}\n")
			hash := sha256.New()
			if _, err = io.CopyN(hash, ch, size); err != nil {
				return err
			}
			if hex.EncodeToString(hash.Sum(nil)) != hashMatch[1] {
				io.WriteString(ch, "{\"schema_version\":1,\"startup_error\":\"upload_checksum_failed\"}\n")
				return errors.New("fixture upload checksum failed")
			}
		}
		return helper(ctx, ch, ch)
	})
}

// StartSSHCommand permits fixture-owned exec responses around real SSH/password/forwarding channels.
func StartSSHCommand(t *testing.T, execHelper func(context.Context, string, ssh.Channel) error) *SSHFixture {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	f := &SSHFixture{IP: "127.0.0.1", Port: number, Fingerprint: ssh.FingerprintSHA256(signer.PublicKey())}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { // The accept owner joins every SSH worker after cancellation closes the listener.
		defer close(done)
		var wg sync.WaitGroup
		defer wg.Wait()
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() { /* Each authenticated fixture owns its socket and helper channels. */
				defer wg.Done()
				serveSSHFixture(ctx, raw, signer, f, execHelper)
			}()
		}
	}()
	t.Cleanup(func() { /* Close all fixture sockets through their contexts and wait for bounded helper cleanup. */
		cancel()
		listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("SSH fixture cleanup timeout")
		}
	})
	return f
}

// serveSSHFixture accepts the one declared account/password and only helper exec or loopback forwarding.
func serveSSHFixture(ctx context.Context, raw net.Conn, signer ssh.Signer, f *SSHFixture, helper func(context.Context, string, ssh.Channel) error) {
	defer raw.Close()
	stop := context.AfterFunc(ctx, func() { /* Unblock handshake/channel reads when the isolated fixture ends. */ raw.Close() })
	defer stop()
	config := &ssh.ServerConfig{PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) { /* Count actual credential delivery so untrusted hosts cannot receive a password. */
		f.Authentications.Add(1)
		if meta.User() != "sneak" || string(password) != "fixture-password" {
			return nil, errors.New("fixture credentials rejected")
		}
		return &ssh.Permissions{}, nil
	}}
	config.AddHostKey(signer)
	conn, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		return
	}
	defer conn.Close()
	owned, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { /* SSH EOF cancels the disposable helper as a real login would. */ conn.Wait(); cancel() }()
	go ssh.DiscardRequests(requests)
	var wg sync.WaitGroup
	defer wg.Wait()
	for ch := range channels {
		wg.Add(1)
		go func() { /* Bound this channel's helper/copy ownership to its parent SSH connection. */
			defer wg.Done()
			fixtureSSHChannel(owned, ch, helper)
		}()
	}
}

// fixtureSSHChannel implements normal SSH exec plus direct-tcpip for the private measurement listener.
func fixtureSSHChannel(ctx context.Context, newChannel ssh.NewChannel, helper func(context.Context, string, ssh.Channel) error) {
	if newChannel.ChannelType() == "direct-tcpip" {
		var target struct {
			Host       string
			Port       uint32
			Origin     string
			OriginPort uint32
		}
		if ssh.Unmarshal(newChannel.ExtraData(), &target) != nil || target.Host != "127.0.0.1" || target.Port < 1 || target.Port > 65535 {
			newChannel.Reject(ssh.Prohibited, "fixture target rejected")
			return
		}
		raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(target.Host, strconv.Itoa(int(target.Port))))
		if err != nil {
			newChannel.Reject(ssh.ConnectionFailed, "fixture dial failed")
			return
		}
		ch, requests, err := newChannel.Accept()
		if err != nil {
			raw.Close()
			return
		}
		defer ch.Close()
		defer raw.Close()
		go ssh.DiscardRequests(requests)
		stop := context.AfterFunc(ctx, func() { /* A lost SSH parent interrupts direct-tcpip readers immediately. */ ch.Close(); raw.Close() })
		defer stop()
		done := make(chan struct{})
		go func() { /* Copy only private control traffic and signal/join the reverse loop. */
			io.Copy(raw, ch)
			raw.Close()
			ch.Close()
			close(done)
		}()
		io.Copy(ch, raw)
		raw.Close()
		ch.Close()
		<-done
		return
	}
	if newChannel.ChannelType() != "session" {
		newChannel.Reject(ssh.UnknownChannelType, "fixture channel rejected")
		return
	}
	ch, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	defer ch.Close()
	for request := range requests {
		var command struct{ Command string }
		accepted := request.Type == "exec" && ssh.Unmarshal(request.Payload, &command) == nil && strings.HasPrefix(command.Command, "# sneaky-port dashboard launcher v1\n") && helper != nil
		request.Reply(accepted, nil)
		if accepted {
			helper(ctx, command.Command, ch)
			return
		}
	}
}
