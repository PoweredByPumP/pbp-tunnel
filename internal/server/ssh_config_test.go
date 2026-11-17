package server

import (
	"net"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
)

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
