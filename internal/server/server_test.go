package server

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
)

func TestRun_RejectsInvalidParameters(t *testing.T) {
	err := Run(&config.ServerParameters{})
	if err == nil || !strings.Contains(err.Error(), "invalid server parameters") {
		t.Fatalf("Run error = %v, want invalid server parameters", err)
	}
}

func TestServe_StopsWhenListenerIsClosed(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- newTestServer(t, 40000, 40010).serve(listener) }()

	listener.Close()

	select {
	case err := <-served:
		if err != nil {
			t.Errorf("serve returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve kept running after its listener was closed")
	}
}
