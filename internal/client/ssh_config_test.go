package client

import (
	"strings"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
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
