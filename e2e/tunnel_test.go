package e2e

import (
	"bytes"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
)

const (
	loopback       = "127.0.0.1"
	tunnelUser     = "e2e-user"
	tunnelPassword = "e2e-password"
	readyTimeout   = 10 * time.Second
	// reconnectTimeout leaves room for the 5 s the client waits before reconnecting.
	reconnectTimeout = 20 * time.Second
)

func TestTCPTunnel_ForwardsTrafficToLocalService(t *testing.T) {
	localServicePort := startTCPEchoService(t)
	exposedPort := freeTCPPort(t)

	sshPort, serverStopped := startTunnelServer(t, exposedPort)
	clientStopped := startTunnelClient(t, sshPort, "tcp", localServicePort, exposedPort)

	conn := dialTCPWhenTunnelIsUp(t, exposedPort, serverStopped, clientStopped)
	defer conn.Close()

	message := []byte("hello through the tunnel")
	if _, err := conn.Write(message); err != nil {
		t.Fatalf("write to exposed port: %v", err)
	}

	echoed := make([]byte, len(message))
	_ = conn.SetReadDeadline(time.Now().Add(readyTimeout))
	if _, err := io.ReadFull(conn, echoed); err != nil {
		t.Fatalf("read echo from exposed port: %v", err)
	}
	if !bytes.Equal(echoed, message) {
		t.Fatalf("echo = %q, want %q", echoed, message)
	}
}

// Like an HTTP/1.0 server, the local service only answers once the peer has
// finished sending, so the end of the request must travel through the tunnel.
func TestTCPTunnel_LocalServiceSeesTheEndOfTheRequest(t *testing.T) {
	localServicePort := startTCPServiceAnsweringAtEndOfRequest(t)
	exposedPort := freeTCPPort(t)

	sshPort, serverStopped := startTunnelServer(t, exposedPort)
	clientStopped := startTunnelClient(t, sshPort, "tcp", localServicePort, exposedPort)

	conn := dialTCPWhenTunnelIsUp(t, exposedPort, serverStopped, clientStopped).(*net.TCPConn)
	defer conn.Close()

	if _, err := conn.Write([]byte("request")); err != nil {
		t.Fatal(err)
	}
	_ = conn.CloseWrite()
	_ = conn.SetReadDeadline(time.Now().Add(readyTimeout))
	response, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(response) != "response to request" {
		t.Errorf("response = %q, want %q", response, "response to request")
	}
}

func TestTCPTunnel_ClientReconnectsAfterLosingTheServer(t *testing.T) {
	localServicePort := startTCPEchoService(t)
	exposedPort := freeTCPPort(t)
	sshPort, serverStopped := startTunnelServer(t, exposedPort)
	proxy := startCuttableProxy(t, sshPort)
	clientStopped := startTunnelClient(t, proxy.port(), "tcp", localServicePort, exposedPort)

	waitForEchoThroughTunnel(t, exposedPort, readyTimeout, serverStopped, clientStopped)
	proxy.cutConnections()
	waitForEchoThroughTunnel(t, exposedPort, reconnectTimeout, serverStopped, clientStopped)
}

func TestTunnelClient_StrictHostKeyCheckAcceptsTheKnownServer(t *testing.T) {
	localServicePort := startTCPEchoService(t)
	exposedPort := freeTCPPort(t)
	hostKeyPath := filepath.Join(t.TempDir(), "host_ed25519")
	sshPort, serverStopped := startTunnelServerWithHostKey(t, exposedPort, hostKeyPath)

	params := tunnelClientParameters(sshPort, "tcp", localServicePort, exposedPort)
	params.HostKeyLevel = config.HostKeyMustBeKnown
	params.HostKeyPath = knownHostsTrusting(t, sshPort, publicKeyOf(t, hostKeyPath))
	clientStopped := runTunnelClient(params)

	waitForEchoThroughTunnel(t, exposedPort, readyTimeout, serverStopped, clientStopped)
}

func TestTunnelClient_StrictHostKeyCheckStopsAtAnUnknownServer(t *testing.T) {
	exposedPort := freeTCPPort(t)
	sshPort, _ := startTunnelServer(t, exposedPort)

	params := tunnelClientParameters(sshPort, "tcp", startTCPEchoService(t), exposedPort)
	params.HostKeyLevel = config.HostKeyMustBeKnown
	params.HostKeyPath = knownHostsTrusting(t, sshPort, newImpostorKey(t))
	clientStopped := runTunnelClient(params)

	select {
	case err := <-clientStopped:
		if err == nil || !strings.Contains(err.Error(), "server identity rejected") {
			t.Errorf("client stopped with %v, want a server identity rejection", err)
		}
	case <-time.After(readyTimeout):
		t.Fatalf("client kept trying an unknown server for %v instead of stopping", readyTimeout)
	}
}

func TestTCPTunnel_DisconnectsPeersOutsideAllowedList(t *testing.T) {
	localServicePort := startTCPEchoService(t)
	exposedPort := freeTCPPort(t)

	sshPort, serverStopped := startTunnelServer(t, exposedPort)
	clientStopped := startTunnelClient(t, sshPort, "tcp", localServicePort, exposedPort, "192.0.2.1")

	conn := dialTCPWhenTunnelIsUp(t, exposedPort, serverStopped, clientStopped)
	defer conn.Close()

	_, _ = conn.Write([]byte("hello"))
	_ = conn.SetReadDeadline(time.Now().Add(readyTimeout))
	n, err := conn.Read(make([]byte, 16))
	if n > 0 {
		t.Fatalf("a peer outside the allowed list received %d bytes", n)
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("the connection of a peer outside the allowed list was left open")
	}
}

func TestTunnelServer_RefusesClientsOutsideAllowedList(t *testing.T) {
	exposedPort := freeTCPPort(t)

	sshPort, _ := startTunnelServer(t, exposedPort, "192.0.2.1")
	clientStopped := startTunnelClient(t, sshPort, "tcp", startTCPEchoService(t), exposedPort)

	select {
	case err := <-clientStopped:
		if !errors.Is(err, protocol.StatusPeerNotAllowed) {
			t.Errorf("client stopped with %v, want %v", err, protocol.StatusPeerNotAllowed)
		}
	case <-time.After(readyTimeout):
		t.Fatalf("client outside the allowed list was not refused within %v", readyTimeout)
	}
}

func TestUDPTunnel_ForwardsDatagramsToLocalService(t *testing.T) {
	localServicePort := startUDPEchoService(t)
	exposedPort := freeUDPPort(t)

	sshPort, serverStopped := startTunnelServer(t, exposedPort)
	clientStopped := startTunnelClient(t, sshPort, "udp", localServicePort, exposedPort)

	conn, err := net.Dial("udp", net.JoinHostPort(loopback, strconv.Itoa(exposedPort)))
	if err != nil {
		t.Fatalf("dial exposed UDP port: %v", err)
	}
	defer conn.Close()

	// The tunnel gives no readiness signal for UDP, so keep sending the same
	// datagram until one comes back.
	message := []byte("ping through the tunnel")
	echoed := make([]byte, 1024)
	deadline := time.Now().Add(readyTimeout)
	for time.Now().Before(deadline) {
		failIfStopped(t, serverStopped, clientStopped)

		_, _ = conn.Write(message)
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := conn.Read(echoed)
		if err != nil {
			continue
		}
		if !bytes.Equal(echoed[:n], message) {
			t.Fatalf("echo = %q, want %q", echoed[:n], message)
		}
		return
	}
	t.Fatalf("no UDP echo received through the tunnel within %v", readyTimeout)
}
