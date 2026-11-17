package e2e

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/client"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"github.com/poweredbypump/pbp-tunnel/internal/server"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
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

// startTunnelServer runs a pbp-tunnel server that only allows exposedPort to be forwarded.
// The server has no shutdown mechanism yet, so it keeps running until the test binary exits.
func startTunnelServer(t *testing.T, exposedPort int, allowedClients ...string) (sshPort int, stopped <-chan error) {
	t.Helper()
	return startTunnelServerWithHostKey(t, exposedPort, filepath.Join(t.TempDir(), "host_ed25519"), allowedClients...)
}

// startTunnelServerWithHostKey works like startTunnelServer with the host key
// generated at hostKeyPath, so that a client can be told to trust it.
func startTunnelServerWithHostKey(t *testing.T, exposedPort int, hostKeyPath string, allowedClients ...string) (sshPort int, stopped <-chan error) {
	t.Helper()
	params := &config.ServerParameters{
		AllowedIPs:         allowedClients,
		BindAddress:        loopback,
		BindPort:           freeTCPPort(t),
		PortRangeStart:     exposedPort,
		PortRangeEnd:       exposedPort,
		Username:           tunnelUser,
		Password:           tunnelPassword,
		PrivateEd25519Path: hostKeyPath,
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
	return runTunnelClient(tunnelClientParameters(sshPort, protocol, localServicePort, exposedPort, allowedPeers...))
}

// tunnelClientParameters does not check the server identity; the host key tests opt in.
func tunnelClientParameters(sshPort int, protocol string, localServicePort, exposedPort int, allowedPeers ...string) *config.ClientParameters {
	return &config.ClientParameters{
		AllowedIPs:   allowedPeers,
		Endpoint:     loopback,
		EndpointPort: sshPort,
		Username:     tunnelUser,
		Password:     tunnelPassword,
		LocalHost:    loopback,
		LocalPort:    localServicePort,
		RemotePort:   exposedPort,
		Protocol:     protocol,
		HostKeyLevel: config.HostKeyNotChecked,
	}
}

func runTunnelClient(params *config.ClientParameters) <-chan error {
	runResult := make(chan error, 1)
	go func() { runResult <- client.Run(params) }()
	return runResult
}

// knownHostsTrusting writes a known_hosts file that trusts serverKey for the server on sshPort.
func knownHostsTrusting(t *testing.T, sshPort int, serverKey ssh.PublicKey) string {
	t.Helper()
	address := knownhosts.Normalize(net.JoinHostPort(loopback, strconv.Itoa(sshPort)))
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(knownhosts.Line([]string{address}, serverKey)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func publicKeyOf(t *testing.T, privateKeyPath string) ssh.PublicKey {
	t.Helper()
	content, err := os.ReadFile(privateKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey(content)
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}

func newImpostorKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return sshKey
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

// waitForEchoThroughTunnel retries a full echo round trip, so that a stale listener
// left by a dead tunnel does not count as a working one.
func waitForEchoThroughTunnel(t *testing.T, port int, timeout time.Duration, serverStopped, clientStopped <-chan error) {
	t.Helper()
	address := net.JoinHostPort(loopback, strconv.Itoa(port))
	message := []byte("ping")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		failIfStopped(t, serverStopped, clientStopped)
		if echoes(address, message) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no echo through the tunnel on %s within %v", address, timeout)
}

func echoes(address string, message []byte) bool {
	conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write(message); err != nil {
		return false
	}
	echoed := make([]byte, len(message))
	_, err = io.ReadFull(conn, echoed)
	return err == nil && bytes.Equal(echoed, message)
}

// cuttableProxy relays TCP connections to a target until cutConnections drops
// them all, like a network failure between the client and the server.
type cuttableProxy struct {
	listener net.Listener

	mu    sync.Mutex
	conns []net.Conn
}

func startCuttableProxy(t *testing.T, targetPort int) *cuttableProxy {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	proxy := &cuttableProxy{listener: listener}
	t.Cleanup(func() { listener.Close(); proxy.cutConnections() })

	target := net.JoinHostPort(loopback, strconv.Itoa(targetPort))
	go func() {
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				return
			}
			targetConn, err := net.Dial("tcp", target)
			if err != nil {
				clientConn.Close()
				continue
			}
			proxy.track(clientConn, targetConn)
			go func() { _, _ = io.Copy(targetConn, clientConn); targetConn.Close() }()
			go func() { _, _ = io.Copy(clientConn, targetConn); clientConn.Close() }()
		}
	}()
	return proxy
}

func (p *cuttableProxy) port() int {
	return p.listener.Addr().(*net.TCPAddr).Port
}

func (p *cuttableProxy) track(conns ...net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conns = append(p.conns, conns...)
}

func (p *cuttableProxy) cutConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.conns {
		conn.Close()
	}
	p.conns = nil
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

func startTCPServiceAnsweringAtEndOfRequest(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("start TCP service: %v", err)
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
				request, err := io.ReadAll(conn)
				if err == nil {
					_, _ = conn.Write([]byte("response to " + string(request)))
				}
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
