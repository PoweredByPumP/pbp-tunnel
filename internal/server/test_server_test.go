package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/allowlist"
	"golang.org/x/crypto/ssh"
)

// newTestServer builds a Server without authentication that exposes ports in
// [firstPort, lastPort] on the loopback interface.
func newTestServer(t *testing.T, firstPort, lastPort int, allowedClients ...string) *Server {
	t.Helper()
	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	sshConfig := &ssh.ServerConfig{NoClientAuth: true}
	sshConfig.AddHostKey(signer)

	allowed, err := allowlist.Parse(allowedClients)
	if err != nil {
		t.Fatal(err)
	}
	return &Server{
		sshConfig:      sshConfig,
		bindAddress:    "127.0.0.1",
		allowedClients: allowed,
		ports:          newPortPool(firstPort, lastPort),
	}
}

// connectToTestServer serves server on a loopback port and returns a client connected to it.
func connectToTestServer(t *testing.T, server *Server) *ssh.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go server.serve(listener)

	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User:            "test",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to test server: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func openRequestChannel(t *testing.T, client *ssh.Client) ssh.Channel {
	t.Helper()
	channel, requests, err := client.OpenChannel("direct-tcpip", nil)
	if err != nil {
		t.Fatalf("open tunnel request channel: %v", err)
	}
	go ssh.DiscardRequests(requests)
	return channel
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
