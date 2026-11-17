package client

import (
	"strings"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
)

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
