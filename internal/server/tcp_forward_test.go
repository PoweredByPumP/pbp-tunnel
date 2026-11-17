package server

import (
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

// An idle peer connection on the exposed port must not keep the port reserved
// once the tunnel client has disconnected.
func TestExposeTCP_ReleasesPortWhenClientLeavesWhilePeerIsIdle(t *testing.T) {
	port := freeTCPPort(t)
	server := newTestServer(t, port, port)
	client := connectToTestServer(t, server)
	answerForwardedConnectionsWithSilence(client)

	exposedPort, err := protocol.RequestTunnel(openRequestChannel(t, client), protocol.TunnelRequest{Transport: protocol.TCP, Port: port})
	if err != nil || exposedPort != port {
		t.Fatalf("RequestTunnel = %d, %v; want %d, nil", exposedPort, err, port)
	}
	idlePeer, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer idlePeer.Close()
	time.Sleep(200 * time.Millisecond)

	client.Close()

	_ = idlePeer.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = idlePeer.Read(make([]byte, 1))
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatal("the peer connection stayed open after the client disconnected")
	}
	waitUntilPortIsFree(t, server, port)
}

// answerForwardedConnectionsWithSilence accepts forwarded connections and never
// answers them, like a local service that waits for a request.
func answerForwardedConnectionsWithSilence(client *ssh.Client) {
	go func() {
		for newChannel := range client.HandleChannelOpen("direct-tcpip") {
			channel, requests, err := newChannel.Accept()
			if err != nil {
				continue
			}
			go ssh.DiscardRequests(requests)
			go io.Copy(io.Discard, channel)
		}
	}()
}

func waitUntilPortIsFree(t *testing.T, server *Server, port int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := server.ports.Reserve(port); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("port %d is still reserved after the client disconnected", port)
}
