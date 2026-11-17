package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/allowlist"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

func TestRun_InvalidParameters(t *testing.T) {
	err := Run(&config.ServerParameters{})
	if err == nil {
		t.Fatal("expected Run to fail for invalid parameters, got nil")
	}
	if !strings.Contains(err.Error(), "invalid server parameters") {
		t.Fatalf("Run error = %v; want invalid server parameters", err)
	}
}

func newTestSSHConfigs(t *testing.T) (*ssh.ServerConfig, *ssh.ClientConfig) {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("ssh.NewSignerFromKey returned error: %v", err)
	}

	serverCfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "secret" {
				return nil, nil
			}
			return nil, errors.New("invalid credentials")
		},
	}
	serverCfg.AddHostKey(signer)

	clientCfg := &ssh.ClientConfig{
		User:            "testuser",
		Auth:            []ssh.AuthMethod{ssh.Password("secret")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}

	return serverCfg, clientCfg
}

func TestHandleSSHConnection_TellsDisallowedClientWhyItIsRefused(t *testing.T) {
	serverCfg, clientCfg := newTestSSHConfigs(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen returned error: %v", err)
	}
	defer ln.Close()

	allowedClients, err := allowlist.Parse([]string{"203.0.113.10"})
	if err != nil {
		t.Fatal(err)
	}
	s := &ForwardServer{
		sshConfig:      serverCfg,
		allowedClients: allowedClients,
	}

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		s.handleSSHConnection(conn)
	}()

	clientSide, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial returned error: %v", err)
	}
	defer clientSide.Close()

	conn, chans, reqs, err := ssh.NewClientConn(clientSide, ln.Addr().String(), clientCfg)
	if err != nil {
		t.Fatalf("ssh.NewClientConn returned error: %v", err)
	}
	client := ssh.NewClient(conn, chans, reqs)

	channel, channelRequests, err := client.OpenChannel("direct-tcpip", nil)
	if err != nil {
		t.Fatalf("OpenChannel: %v", err)
	}
	go ssh.DiscardRequests(channelRequests)

	_, err = protocol.RequestTunnel(channel, protocol.TunnelRequest{Transport: protocol.TCP})
	if !errors.Is(err, protocol.StatusPeerNotAllowed) {
		t.Errorf("RequestTunnel error = %v, want %v", err, protocol.StatusPeerNotAllowed)
	}

	client.Close()
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handleSSHConnection did not return after the client disconnected")
	}
}

func TestHandleChannel_RefusesUnsupportedTransport(t *testing.T) {
	serverCfg, clientCfg := newTestSSHConfigs(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen returned error: %v", err)
	}
	defer ln.Close()

	serverConnResult := make(chan struct {
		conn  *ssh.ServerConn
		chans <-chan ssh.NewChannel
		reqs  <-chan *ssh.Request
		err   error
	}, 1)
	go func() {
		serverSide, err := ln.Accept()
		if err != nil {
			serverConnResult <- struct {
				conn  *ssh.ServerConn
				chans <-chan ssh.NewChannel
				reqs  <-chan *ssh.Request
				err   error
			}{err: err}
			return
		}
		conn, chans, reqs, err := ssh.NewServerConn(serverSide, serverCfg)
		serverConnResult <- struct {
			conn  *ssh.ServerConn
			chans <-chan ssh.NewChannel
			reqs  <-chan *ssh.Request
			err   error
		}{conn: conn, chans: chans, reqs: reqs, err: err}
	}()

	clientSide, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial returned error: %v", err)
	}
	defer clientSide.Close()

	conn, chans, reqs, err := ssh.NewClientConn(clientSide, ln.Addr().String(), clientCfg)
	if err != nil {
		t.Fatalf("ssh.NewClientConn returned error: %v", err)
	}
	client := ssh.NewClient(conn, chans, reqs)
	defer client.Close()

	res := <-serverConnResult
	if res.err != nil {
		t.Fatalf("ssh.NewServerConn returned error: %v", res.err)
	}
	defer res.conn.Close()
	go ssh.DiscardRequests(res.reqs)

	accepted := make(chan ssh.Channel, 1)
	channelErr := make(chan error, 1)
	go func() {
		for newCh := range res.chans {
			if newCh.ChannelType() != "direct-tcpip" {
				newCh.Reject(ssh.UnknownChannelType, "unsupported channel type")
				continue
			}
			ch, reqs2, err := newCh.Accept()
			if err != nil {
				channelErr <- err
				return
			}
			go ssh.DiscardRequests(reqs2)
			accepted <- ch
			return
		}
	}()

	clientChannel, clientReqs, err := client.OpenChannel("direct-tcpip", nil)
	if err != nil {
		t.Fatalf("client.OpenChannel returned error: %v", err)
	}
	go ssh.DiscardRequests(clientReqs)

	var serverChannel ssh.Channel
	select {
	case serverChannel = <-accepted:
	case err := <-channelErr:
		t.Fatalf("server failed to accept channel: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSH channel acceptance")
	}

	serverDone := make(chan struct{})
	server := &ForwardServer{
		sshConfig:      serverCfg,
		bindAddress:    "127.0.0.1",
		portRangeStart: 40000,
		portRangeEnd:   40010,
		forwards:       make(map[int]struct{}),
	}
	go func() {
		defer close(serverDone)
		server.handleChannel(res.conn, serverChannel)
	}()

	_, err = protocol.RequestTunnel(clientChannel, protocol.TunnelRequest{Transport: "quic"})
	if !errors.Is(err, protocol.StatusUnsupportedTransport) {
		t.Fatalf("RequestTunnel error = %v, want %v", err, protocol.StatusUnsupportedTransport)
	}

	_ = clientChannel.Close()
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handleChannel did not return after refusing the transport")
	}
}
