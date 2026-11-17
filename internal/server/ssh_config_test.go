package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
)

func TestSSHServerConfig_AuthorizedKeysFileWithCommentsAndBlankLines(t *testing.T) {
	authorizedKey, otherKey := newPublicKey(t), newPublicKey(t)
	path := writeFile(t, "authorized_keys", "# admin laptop\n\n"+string(ssh.MarshalAuthorizedKey(authorizedKey))+"\n# old key removed\n\n")

	sshConfig, err := sshServerConfig(&config.ServerParameters{Username: "admin", AuthorizedKeysPath: path})
	if err != nil {
		t.Fatalf("sshServerConfig: %v", err)
	}

	if _, err := sshConfig.PublicKeyCallback(connFromUser("admin"), authorizedKey); err != nil {
		t.Errorf("the authorized key was rejected: %v", err)
	}
	if _, err := sshConfig.PublicKeyCallback(connFromUser("admin"), otherKey); err == nil {
		t.Error("a key missing from the file was accepted")
	}
	if _, err := sshConfig.PublicKeyCallback(connFromUser("someone"), authorizedKey); err == nil {
		t.Error("the authorized key was accepted for another user")
	}
}

func TestSSHServerConfig_ReportsInvalidKeyFiles(t *testing.T) {
	validKey := string(ssh.MarshalAuthorizedKey(newPublicKey(t)))
	tests := []struct {
		name      string
		params    func(t *testing.T) *config.ServerParameters
		wantInErr string
	}{
		{
			name: "invalid line in authorized keys",
			params: func(t *testing.T) *config.ServerParameters {
				return &config.ServerParameters{AuthorizedKeysPath: writeFile(t, "authorized_keys", validKey+"not a key\n")}
			},
			wantInErr: "line 2",
		},
		{
			name: "missing authorized keys file",
			params: func(t *testing.T) *config.ServerParameters {
				return &config.ServerParameters{AuthorizedKeysPath: filepath.Join(t.TempDir(), "missing")}
			},
			wantInErr: "read authorized keys",
		},
		{
			name: "host key that is not a key",
			params: func(t *testing.T) *config.ServerParameters {
				return &config.ServerParameters{PrivateEd25519Path: writeFile(t, "host_key", "not a key")}
			},
			wantInErr: "parse host key",
		},
		{
			name: "missing host key file",
			params: func(t *testing.T) *config.ServerParameters {
				return &config.ServerParameters{PrivateRsaPath: filepath.Join(t.TempDir(), "missing")}
			},
			wantInErr: "read host key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sshServerConfig(tc.params(t))
			if err == nil || !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("sshServerConfig error = %v, want it to mention %q", err, tc.wantInErr)
			}
		})
	}
}

func newPublicKey(t *testing.T) ssh.PublicKey {
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

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSSHServerConfig_PasswordCallbackChecksUserAndPassword(t *testing.T) {
	sshConfig, err := sshServerConfig(&config.ServerParameters{Username: "admin", Password: "passwd"})
	if err != nil {
		t.Fatalf("sshServerConfig: %v", err)
	}
	checkPassword := sshConfig.PasswordCallback
	if checkPassword == nil {
		t.Fatal("PasswordCallback is not set")
	}

	tests := []struct {
		user, password string
		wantAccepted   bool
	}{
		{"admin", "passwd", true},
		{"admin", "wrong", false},
		{"someone", "passwd", false},
	}
	for _, tc := range tests {
		_, err := checkPassword(connFromUser(tc.user), []byte(tc.password))
		if accepted := err == nil; accepted != tc.wantAccepted {
			t.Errorf("user %q with password %q: accepted = %v, want %v", tc.user, tc.password, accepted, tc.wantAccepted)
		}
	}
}

func TestSSHServerConfig_NoPasswordMeansNoPasswordAuth(t *testing.T) {
	sshConfig, err := sshServerConfig(&config.ServerParameters{})
	if err != nil {
		t.Fatalf("sshServerConfig: %v", err)
	}
	if sshConfig.PasswordCallback != nil {
		t.Error("PasswordCallback is set although no password is configured")
	}
}

type connFromUser string

func (c connFromUser) User() string                  { return string(c) }
func (c connFromUser) SessionID() []byte             { return nil }
func (c connFromUser) ClientVersion() []byte         { return nil }
func (c connFromUser) ServerVersion() []byte         { return nil }
func (c connFromUser) RemoteAddr() net.Addr          { return nil }
func (c connFromUser) LocalAddr() net.Addr           { return nil }
func (c connFromUser) Permissions() *ssh.Permissions { return nil }
