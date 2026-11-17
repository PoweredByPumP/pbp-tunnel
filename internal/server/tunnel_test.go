package server

import (
	"errors"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
)

func TestHandleTunnelRequest_Refusals(t *testing.T) {
	tests := []struct {
		name           string
		allowedClients []string
		request        protocol.TunnelRequest
		wantRefusal    protocol.Status
	}{
		{
			name:           "client IP not allowed",
			allowedClients: []string{"203.0.113.10"},
			request:        protocol.TunnelRequest{Transport: protocol.TCP},
			wantRefusal:    protocol.StatusPeerNotAllowed,
		},
		{
			name:        "unsupported transport",
			request:     protocol.TunnelRequest{Transport: "quic"},
			wantRefusal: protocol.StatusUnsupportedTransport,
		},
		{
			name:        "port outside the range",
			request:     protocol.TunnelRequest{Transport: protocol.TCP, Port: 50000},
			wantRefusal: protocol.StatusPortOutOfRange,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := connectToTestServer(t, newTestServer(t, 40000, 40010, tc.allowedClients...))

			_, err := protocol.RequestTunnel(openRequestChannel(t, client), tc.request)

			if !errors.Is(err, tc.wantRefusal) {
				t.Errorf("RequestTunnel error = %v, want %v", err, tc.wantRefusal)
			}
		})
	}
}
