package e2e

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/client"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/server"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

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
