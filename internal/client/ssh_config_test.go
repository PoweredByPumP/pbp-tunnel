package client

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestSSHClientConfig_PasswordAuth(t *testing.T) {
	sshConfig, err := sshClientConfig(&config.ClientParameters{Username: "testuser", Password: "secret"})
	if err != nil {
		t.Fatalf("sshClientConfig: %v", err)
	}
	if sshConfig.User != "testuser" {
		t.Errorf("User = %q, want %q", sshConfig.User, "testuser")
	}
	if len(sshConfig.Auth) != 1 {
		t.Errorf("got %d auth methods, want 1", len(sshConfig.Auth))
	}
}

func TestSSHClientConfig_FailsWhenPrivateKeyIsMissing(t *testing.T) {
	_, err := sshClientConfig(&config.ClientParameters{Username: "testuser", PrivateKeyPath: "/nonexistent/key"})
	if err == nil || !strings.Contains(err.Error(), "read private key") {
		t.Errorf("sshClientConfig error = %v, want a read private key error", err)
	}
}

const serverAddress = "tunnel.example.com:52135"

func TestHostKeyCallback_ChecksTheServerAsTheLevelAsks(t *testing.T) {
	knownServerKey, impostorKey := newHostPublicKey(t), newHostPublicKey(t)
	knownHosts := writeKnownHosts(t, serverAddress, knownServerKey)

	tests := []struct {
		name             string
		level            int
		knownHostsPath   string
		wantKnownServer  bool
		wantImpostorSeen bool
	}{
		{name: "no check", level: config.HostKeyNotChecked, wantKnownServer: true, wantImpostorSeen: true},
		{name: "warn without known_hosts", level: config.HostKeyWarnIfUnknown, wantKnownServer: true, wantImpostorSeen: true},
		{name: "warn with known_hosts", level: config.HostKeyWarnIfUnknown, knownHostsPath: knownHosts, wantKnownServer: true, wantImpostorSeen: true},
		{name: "strict", level: config.HostKeyMustBeKnown, knownHostsPath: knownHosts, wantKnownServer: true, wantImpostorSeen: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checkServerIdentity, err := hostKeyCallback(tc.level, tc.knownHostsPath)
			if err != nil {
				t.Fatalf("hostKeyCallback: %v", err)
			}

			if accepted := connectsTo(checkServerIdentity, knownServerKey); accepted != tc.wantKnownServer {
				t.Errorf("known server accepted = %v, want %v", accepted, tc.wantKnownServer)
			}
			if accepted := connectsTo(checkServerIdentity, impostorKey); accepted != tc.wantImpostorSeen {
				t.Errorf("impostor accepted = %v, want %v", accepted, tc.wantImpostorSeen)
			}
		})
	}
}

func TestHostKeyCallback_StrictRejectionIsAKnownHostsKeyError(t *testing.T) {
	knownHosts := writeKnownHosts(t, serverAddress, newHostPublicKey(t))
	checkServerIdentity, err := hostKeyCallback(config.HostKeyMustBeKnown, knownHosts)
	if err != nil {
		t.Fatal(err)
	}

	err = checkServerIdentity(serverAddress, serverTCPAddr(), newHostPublicKey(t))

	if !isServerIdentityRejected(err) {
		t.Errorf("rejection %v is not recognised as a server identity rejection", err)
	}
	if isServerIdentityRejected(errors.New("connection refused")) {
		t.Error("a network error was taken for a server identity rejection")
	}
}

func TestHostKeyCallback_ReportsUnusableKnownHosts(t *testing.T) {
	tests := []struct {
		name           string
		level          int
		knownHostsPath string
		wantInErr      string
	}{
		{name: "strict without a file", level: config.HostKeyMustBeKnown, wantInErr: "needs a known_hosts file"},
		{name: "missing file", level: config.HostKeyMustBeKnown, knownHostsPath: filepath.Join(t.TempDir(), "missing"), wantInErr: "read known hosts"},
		{name: "public key file instead of known_hosts", level: config.HostKeyWarnIfUnknown, knownHostsPath: writePublicKeyFile(t), wantInErr: "read known hosts"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := hostKeyCallback(tc.level, tc.knownHostsPath)
			if err == nil || !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("hostKeyCallback error = %v, want it to mention %q", err, tc.wantInErr)
			}
		})
	}
}

func connectsTo(checkServerIdentity ssh.HostKeyCallback, serverKey ssh.PublicKey) bool {
	return checkServerIdentity(serverAddress, serverTCPAddr(), serverKey) == nil
}

func serverTCPAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 52135}
}

func newHostPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return sshKey
}

func writeKnownHosts(t *testing.T, address string, key ssh.PublicKey) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(address)}, key) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// writePublicKeyFile writes an id_ed25519.pub style file, which is not a known_hosts file.
func writePublicKeyFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "host_key.pub")
	if err := os.WriteFile(path, ssh.MarshalAuthorizedKey(newHostPublicKey(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
