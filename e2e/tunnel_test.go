package e2e

import (
	"bytes"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/client"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"github.com/poweredbypump/pbp-tunnel/internal/server"
)

const (
	loopback       = "127.0.0.1"
	tunnelUser     = "e2e-user"
	tunnelPassword = "e2e-password"
	readyTimeout   = 10 * time.Second
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

// startTunnelServer runs a pbp-tunnel server that only allows exposedPort to be forwarded.
// The server has no shutdown mechanism yet, so it keeps running until the test binary exits.
func startTunnelServer(t *testing.T, exposedPort int, allowedClients ...string) (sshPort int, stopped <-chan error) {
	t.Helper()
	params := &config.ServerParameters{
		AllowedIPs:         allowedClients,
		BindAddress:        loopback,
		BindPort:           freeTCPPort(t),
		PortRangeStart:     exposedPort,
		PortRangeEnd:       exposedPort,
		Username:           tunnelUser,
		Password:           tunnelPassword,
		PrivateEd25519Path: filepath.Join(t.TempDir(), "host_ed25519"),
	}

	runResult := make(chan error, 1)
	go func() { runResult <- server.Run(params) }()
	waitForTCPListener(t, params.BindPort, runResult)
	return params.BindPort, runResult
}

// startTunnelClient runs a pbp-tunnel client against the server listening on sshPort.
// Like the server, it keeps running until the test binary exits.
func startTunnelClient(t *testing.T, sshPort int, protocol string, localServicePort, exposedPort int, allowedPeers ...string) <-chan error {
	t.Helper()
	params := &config.ClientParameters{
		AllowedIPs:   allowedPeers,
		Endpoint:     loopback,
		EndpointPort: sshPort,
		Username:     tunnelUser,
		Password:     tunnelPassword,
		LocalHost:    loopback,
		LocalPort:    localServicePort,
		RemoteHost:   loopback,
		RemotePort:   exposedPort,
		Protocol:     protocol,
	}

	runResult := make(chan error, 1)
	go func() { runResult <- client.Run(params) }()
	return runResult
}

func dialTCPWhenTunnelIsUp(t *testing.T, port int, serverStopped, clientStopped <-chan error) net.Conn {
	t.Helper()
	address := net.JoinHostPort(loopback, strconv.Itoa(port))
	deadline := time.Now().Add(readyTimeout)
	for time.Now().Before(deadline) {
		failIfStopped(t, serverStopped, clientStopped)
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			return conn
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("tunnel did not expose %s within %v", address, readyTimeout)
	return nil
}

func waitForTCPListener(t *testing.T, port int, stopped <-chan error) {
	t.Helper()
	conn := dialTCPWhenTunnelIsUp(t, port, stopped, nil)
	conn.Close()
}

func failIfStopped(t *testing.T, serverStopped, clientStopped <-chan error) {
	t.Helper()
	select {
	case err := <-serverStopped:
		t.Fatalf("tunnel server stopped: %v", err)
	case err := <-clientStopped:
		t.Fatalf("tunnel client stopped: %v", err)
	default:
	}
}

func startTCPEchoService(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("start TCP echo service: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func startUDPEchoService(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("start UDP echo service: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	go func() {
		buffer := make([]byte, 64*1024)
		for {
			n, sender, err := conn.ReadFrom(buffer)
			if err != nil {
				return
			}
			_, _ = conn.WriteTo(buffer[:n], sender)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("find free TCP port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("find free UDP port: %v", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}
