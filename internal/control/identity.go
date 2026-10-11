package control

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// PublicIdentity contains only the enrollment material safe for trusted public transfer.
type PublicIdentity struct {
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
}

// GenerateKey creates a new local Ed25519 private key and never overwrites existing files.
func GenerateKey(path string) (PublicIdentity, error) {
	var out PublicIdentity
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return out, err
	}
	block, err := ssh.MarshalPrivateKey(key, "sneaky-port local identity")
	if err != nil {
		return out, err
	}
	f, err := createPrivate(path)
	if err != nil {
		return out, err
	}
	success := false
	defer func() { // Remove only the newly created incomplete file when restrictive write fails.
		f.Close()
		if !success {
			os.Remove(path)
		}
	}()
	if err = restrictKey(f); err != nil {
		return out, err
	}
	if _, err = f.Write(pem.EncodeToMemory(block)); err != nil {
		return out, err
	}
	if err = f.Sync(); err != nil {
		return out, err
	}
	if err = f.Close(); err != nil {
		return out, err
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return out, err
	}
	success = true
	out.PublicKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	out.Fingerprint = ssh.FingerprintSHA256(signer.PublicKey())
	return out, nil
}

// LoadKey reads a bounded existing key without copying, rewriting or saving a passphrase.
func LoadKey(path string, passphrase []byte) (ssh.Signer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, FrameMax+1))
	if err != nil {
		return nil, err
	}
	if len(b) > FrameMax {
		return nil, errors.New("private key file too large")
	}
	if len(passphrase) > 0 {
		return ssh.ParsePrivateKeyWithPassphrase(b, passphrase)
	}
	return ssh.ParsePrivateKey(b)
}

// PinnedHost rejects every identity other than the explicitly imported public server key.
func PinnedHost(authorizedKey, fingerprint string) (ssh.HostKeyCallback, error) {
	key, err := ParsePublic(authorizedKey)
	if err != nil {
		return nil, errors.New("invalid single server public key")
	}
	if ssh.FingerprintSHA256(key) != fingerprint {
		return nil, errors.New("server fingerprint does not match public key")
	}
	return func(_ string, _ net.Addr, remote ssh.PublicKey) error { // Compare the imported identity exactly; never trust on first use.
		if string(remote.Marshal()) != string(key.Marshal()) {
			return errors.New("server host identity mismatch; trusted re-import required")
		}
		return nil
	}, nil
}

// Enrollment models application authorization, including explicit expiry and revocation.
type Enrollment struct {
	ClientID  string     `json:"client_id"`
	PublicKey string     `json:"public_key"`
	Enabled   bool       `json:"enabled"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Authorize checks current application records; system SSH authorization is not consulted.
func Authorize(records []Enrollment, key ssh.PublicKey, now time.Time) (string, error) {
	for _, r := range records {
		candidate, err := ParsePublic(r.PublicKey)
		if err != nil {
			return "", fmt.Errorf("invalid enrollment record %s", r.ClientID)
		}
		if string(candidate.Marshal()) == string(key.Marshal()) {
			if !r.Enabled || r.ExpiresAt != nil && !now.Before(*r.ExpiresAt) {
				return "", errors.New("client disabled or expired")
			}
			if r.ClientID == "" {
				return "", errors.New("empty client identity")
			}
			return r.ClientID, nil
		}
	}
	return "", errors.New("client public key not enrolled")
}
