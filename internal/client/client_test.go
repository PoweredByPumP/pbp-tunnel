package client

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
)

func TestShouldReconnect(t *testing.T) {
	tests := []struct {
		name       string
		sessionErr error
		want       bool
	}{
		{name: "connection closed cleanly", sessionErr: nil, want: true},
		{name: "server went away", sessionErr: io.EOF, want: true},
		{name: "connection reset", sessionErr: fmt.Errorf("read: %w", syscall.ECONNRESET), want: true},
		{name: "localized OS message", sessionErr: errors.New("Une connexion existante a dû être fermée par l'hôte distant."), want: true},
		{name: "port refused", sessionErr: fmt.Errorf("tunnel request: %w", protocol.StatusPortUnavailable), want: false},
		{name: "client IP refused", sessionErr: fmt.Errorf("tunnel request: greeting: %w", protocol.StatusPeerNotAllowed), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldReconnect(tc.sessionErr); got != tc.want {
				t.Errorf("shouldReconnect(%v) = %v, want %v", tc.sessionErr, got, tc.want)
			}
		})
	}
}

func TestRun_RejectsInvalidParameters(t *testing.T) {
	err := Run(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "invalid client parameters") {
		t.Fatalf("Run error = %v, want invalid client parameters", err)
	}
}

func TestRun_RejectsInvalidAllowedIPsBeforeConnecting(t *testing.T) {
	params := config.DefaultClientParameters()
	params.Endpoint = "127.0.0.1"
	params.EndpointPort = 1
	params.Username = "user"
	params.Password = "pass"
	params.AllowedIPs = []string{"10.0.0.1,10.0.0.2"}

	err := Run(params)

	if err == nil || !strings.Contains(err.Error(), "allowed IPs") {
		t.Errorf("Run error = %v, want an allowed IPs error", err)
	}
}
