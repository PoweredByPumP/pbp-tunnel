package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
)

func TestTunnelHandshake_ServerReceivesRequestAndClientGetsExposedPort(t *testing.T) {
	sentRequest := TunnelRequest{
		AllowedPeers: []string{"10.0.0.0/8", "203.0.113.7"},
		Transport:    UDP,
		Port:         40000,
	}

	receivedRequest := make(chan TunnelRequest, 1)
	exposedPort, err := requestTunnelFrom(t, sentRequest, func(conn net.Conn) {
		request, err := ReceiveTunnelRequest(conn)
		if err != nil {
			t.Errorf("ReceiveTunnelRequest: %v", err)
			return
		}
		receivedRequest <- request
		_ = AcceptTunnel(conn, 40001)
	})

	if err != nil {
		t.Fatalf("RequestTunnel: %v", err)
	}
	if exposedPort != 40001 {
		t.Errorf("exposed port = %d, want 40001", exposedPort)
	}
	got := <-receivedRequest
	if got.Transport != sentRequest.Transport || got.Port != sentRequest.Port ||
		strings.Join(got.AllowedPeers, ",") != strings.Join(sentRequest.AllowedPeers, ",") {
		t.Errorf("server received %+v, want %+v", got, sentRequest)
	}
}

func TestTunnelHandshake_ClientGetsServerRefusalAsStatus(t *testing.T) {
	for _, refusal := range []Status{
		StatusPortUnavailable,
		StatusPortOutOfRange,
		StatusInternalError,
		Status(42),
	} {
		t.Run(refusal.Error(), func(t *testing.T) {
			_, err := requestTunnelFrom(t, TunnelRequest{Transport: TCP}, func(conn net.Conn) {
				if _, err := ReceiveTunnelRequest(conn); err != nil {
					t.Errorf("ReceiveTunnelRequest: %v", err)
					return
				}
				_ = RefuseTunnel(conn, refusal)
			})
			if !errors.Is(err, refusal) {
				t.Errorf("RequestTunnel error = %v, want %v", err, refusal)
			}
		})
	}
}

func TestTunnelHandshake_ClientGetsInternalErrorForNonStatusRefusal(t *testing.T) {
	_, err := requestTunnelFrom(t, TunnelRequest{Transport: TCP}, func(conn net.Conn) {
		if _, err := ReceiveTunnelRequest(conn); err != nil {
			t.Errorf("ReceiveTunnelRequest: %v", err)
			return
		}
		_ = RefuseTunnel(conn, errors.New("address already in use"))
	})
	if !errors.Is(err, StatusInternalError) {
		t.Errorf("RequestTunnel error = %v, want %v", err, StatusInternalError)
	}
}

func TestTunnelHandshake_RejectedPeerStopsClientAtGreeting(t *testing.T) {
	_, err := requestTunnelFrom(t, TunnelRequest{Transport: TCP}, func(conn net.Conn) {
		_ = RejectPeer(conn)
	})
	if !errors.Is(err, StatusPeerNotAllowed) {
		t.Errorf("RequestTunnel error = %v, want %v", err, StatusPeerNotAllowed)
	}
}

func TestTunnelHandshake_ServerRefusesUnsupportedTransport(t *testing.T) {
	for _, transport := range []Transport{"quic", "", Transport(strings.Repeat("t", maxTransportNameLength+1))} {
		t.Run(string(transport), func(t *testing.T) {
			serverErr := make(chan error, 1)
			_, clientErr := requestTunnelFrom(t, TunnelRequest{Transport: transport}, func(conn net.Conn) {
				_, err := ReceiveTunnelRequest(conn)
				serverErr <- err
			})
			if !errors.Is(clientErr, StatusUnsupportedTransport) {
				t.Errorf("RequestTunnel error = %v, want %v", clientErr, StatusUnsupportedTransport)
			}
			if err := <-serverErr; !errors.Is(err, StatusUnsupportedTransport) {
				t.Errorf("ReceiveTunnelRequest error = %v, want %v", err, StatusUnsupportedTransport)
			}
		})
	}
}

func TestRequestTunnel_FailsWhenServerHangsUp(t *testing.T) {
	serverReplies := wireBytes(StatusOK) // greeting only, then the connection ends
	conn := &scriptedConn{replies: bytes.NewReader(serverReplies)}

	_, err := RequestTunnel(conn, TunnelRequest{Transport: TCP})
	if !errors.Is(err, io.EOF) {
		t.Errorf("RequestTunnel error = %v, want EOF", err)
	}
}

// The wire format must stay byte-for-byte compatible with clients and servers
// that are already deployed.
func TestRequestTunnel_WireFormat(t *testing.T) {
	conn := &scriptedConn{replies: bytes.NewReader(wireBytes(StatusOK, StatusOK, 8080))}

	_, err := RequestTunnel(conn, TunnelRequest{AllowedPeers: []string{"10.0.0.0/8"}, Transport: UDP, Port: 8080})
	if err != nil {
		t.Fatalf("RequestTunnel: %v", err)
	}

	want := wireBytes(1, 10, "10.0.0.0/8", 3, "udp", 8080)
	if !bytes.Equal(conn.written.Bytes(), want) {
		t.Errorf("client wrote\n%x\nwant\n%x", conn.written.Bytes(), want)
	}
}

func TestReceiveTunnelRequest_WireFormat(t *testing.T) {
	clientRequest := wireBytes(1, 10, "10.0.0.0/8", 3, "udp", 8080)

	t.Run("accepted", func(t *testing.T) {
		conn := &scriptedConn{replies: bytes.NewReader(clientRequest)}
		if _, err := ReceiveTunnelRequest(conn); err != nil {
			t.Fatalf("ReceiveTunnelRequest: %v", err)
		}
		_ = AcceptTunnel(conn, 8080)

		want := wireBytes(StatusOK, StatusOK, 8080)
		if !bytes.Equal(conn.written.Bytes(), want) {
			t.Errorf("server wrote %x, want %x", conn.written.Bytes(), want)
		}
	})

	t.Run("refused", func(t *testing.T) {
		conn := &scriptedConn{replies: bytes.NewReader(clientRequest)}
		if _, err := ReceiveTunnelRequest(conn); err != nil {
			t.Fatalf("ReceiveTunnelRequest: %v", err)
		}
		_ = RefuseTunnel(conn, StatusPortOutOfRange)

		want := wireBytes(StatusOK, StatusOK, 0x80000003)
		if !bytes.Equal(conn.written.Bytes(), want) {
			t.Errorf("server wrote %x, want %x", conn.written.Bytes(), want)
		}
	})

	t.Run("peer rejected", func(t *testing.T) {
		var written bytes.Buffer
		_ = RejectPeer(&written)

		want := wireBytes(StatusPeerNotAllowed)
		if !bytes.Equal(written.Bytes(), want) {
			t.Errorf("server wrote %x, want %x", written.Bytes(), want)
		}
	})
}

func TestParseTransport(t *testing.T) {
	tests := []struct {
		name    string
		want    Transport
		wantErr bool
	}{
		{name: "tcp", want: TCP},
		{name: "udp", want: UDP},
		{name: " UDP ", want: UDP},
		{name: "", wantErr: true},
		{name: "sctp", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTransport(tc.name)
			if tc.wantErr {
				if !errors.Is(err, StatusUnsupportedTransport) {
					t.Errorf("ParseTransport(%q) error = %v, want %v", tc.name, err, StatusUnsupportedTransport)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("ParseTransport(%q) = %q, %v, want %q", tc.name, got, err, tc.want)
			}
		})
	}
}

// requestTunnelFrom runs RequestTunnel against a fake server driven by serve.
//
// It uses a loopback TCP connection rather than net.Pipe because the server may
// refuse a request while the client is still writing it: like an SSH channel, the
// connection must buffer those writes instead of blocking both sides.
func requestTunnelFrom(t *testing.T, request TunnelRequest, serve func(conn net.Conn)) (int, error) {
	t.Helper()
	clientSide, serverSide := loopbackConnPair(t)
	clientDone := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		serve(serverSide)
		// Closing with unread client bytes would reset the connection and could
		// discard the reply before the client reads it.
		<-clientDone
		serverSide.Close()
	}()

	exposedPort, err := RequestTunnel(clientSide, request)
	clientSide.Close()
	close(clientDone)
	<-serverDone
	return exposedPort, err
}

func loopbackConnPair(t *testing.T) (clientSide, serverSide net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	defer listener.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()

	clientSide, err = net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}
	serverSide = <-accepted
	if serverSide == nil {
		t.Fatal("accept loopback connection failed")
	}
	return clientSide, serverSide
}

// scriptedConn replays prepared replies and records everything written to it.
type scriptedConn struct {
	replies io.Reader
	written bytes.Buffer
}

func (c *scriptedConn) Read(p []byte) (int, error)  { return c.replies.Read(p) }
func (c *scriptedConn) Write(p []byte) (int, error) { return c.written.Write(p) }

// wireBytes encodes integers and statuses as big-endian uint32 and copies strings as-is.
func wireBytes(fields ...any) []byte {
	var encoded []byte
	for _, field := range fields {
		switch value := field.(type) {
		case int:
			encoded = binary.BigEndian.AppendUint32(encoded, uint32(value))
		case Status:
			encoded = binary.BigEndian.AppendUint32(encoded, uint32(value))
		case string:
			encoded = append(encoded, value...)
		default:
			panic("wireBytes: unsupported field type")
		}
	}
	return encoded
}
