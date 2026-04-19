package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/util"
	"golang.org/x/crypto/ssh"
)

type failingWriter struct {
	failOnWrite int
	writes      int
	err         error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes >= w.failOnWrite {
		return 0, w.err
	}
	return len(p), nil
}

func TestPeerAllowed(t *testing.T) {
	tests := []struct {
		name       string
		peer       string
		clientWL   []string
		wantAccept bool
	}{
		{name: "empty-whitelist-allows-any-peer", peer: "203.0.113.10", clientWL: nil, wantAccept: true},
		{name: "exact-match-allowed", peer: "203.0.113.10", clientWL: []string{"203.0.113.10"}, wantAccept: true},
		{name: "cidr-match-allowed", peer: "192.168.1.42", clientWL: []string{"192.168.1.0/24"}, wantAccept: true},
		{name: "mismatch-denied", peer: "198.51.100.20", clientWL: []string{"192.168.1.0/24", "203.0.113.11"}, wantAccept: false},
		{name: "invalid-cidr-denied", peer: "198.51.100.20", clientWL: []string{"bad/cidr"}, wantAccept: false},
		{name: "invalid-peer-denied", peer: "not-an-ip", clientWL: []string{"10.0.0.0/8"}, wantAccept: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := peerAllowed(tc.peer, tc.clientWL)
			if got != tc.wantAccept {
				t.Fatalf("peerAllowed(%q, %v) = %v; want %v", tc.peer, tc.clientWL, got, tc.wantAccept)
			}
		})
	}
}

func TestWriteFramedPacket_RoundTrip(t *testing.T) {
	payloads := [][]byte{
		nil,
		{},
		[]byte("hello world"),
		bytes.Repeat([]byte{0xAB}, 1024),
	}

	for i, payload := range payloads {
		t.Run(fmt.Sprintf("payload-%d", i), func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeFramedPacket(&buf, payload); err != nil {
				t.Fatalf("writeFramedPacket returned error: %v", err)
			}

			got, err := readFramedPacket(&buf)
			if err != nil {
				t.Fatalf("readFramedPacket returned error: %v", err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatalf("round-trip payload = %v; want %v", got, payload)
			}
		})
	}
}

func TestWriteFramedPacket_WriteErrors(t *testing.T) {
	t.Run("length-write-fails", func(t *testing.T) {
		w := &failingWriter{failOnWrite: 1, err: errors.New("length write failed")}
		if err := writeFramedPacket(w, []byte("payload")); err == nil || !strings.Contains(err.Error(), "length write failed") {
			t.Fatalf("writeFramedPacket error = %v; want length write failure", err)
		}
	})

	t.Run("payload-write-fails", func(t *testing.T) {
		w := &failingWriter{failOnWrite: 2, err: errors.New("payload write failed")}
		if err := writeFramedPacket(w, []byte("payload")); err == nil || !strings.Contains(err.Error(), "payload write failed") {
			t.Fatalf("writeFramedPacket error = %v; want payload write failure", err)
		}
	})
}

func TestReadFramedPacket_Errors(t *testing.T) {
	t.Run("missing-length", func(t *testing.T) {
		_, err := readFramedPacket(bytes.NewReader(nil))
		if err == nil {
			t.Fatal("expected error for missing length, got nil")
		}
	})

	t.Run("truncated-payload", func(t *testing.T) {
		var buf bytes.Buffer
		_ = binary.Write(&buf, binary.BigEndian, uint32(4))
		buf.Write([]byte{0x01, 0x02})

		_, err := readFramedPacket(&buf)
		if err == nil {
			t.Fatal("expected error for truncated payload, got nil")
		}
	})
}

func TestRun_InvalidParameters(t *testing.T) {
	err := Run(&config.ServerParameters{})
	if err == nil {
		t.Fatal("expected Run to fail for invalid parameters, got nil")
	}
	if !strings.Contains(err.Error(), "invalid server parameters") {
		t.Fatalf("Run error = %v; want invalid server parameters", err)
	}
}

func TestReadFramedPacket_ZeroLengthPayload(t *testing.T) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))

	payload, err := readFramedPacket(&buf)
	if err != nil {
		t.Fatalf("readFramedPacket returned error: %v", err)
	}
	if len(payload) != 0 {
		t.Fatalf("expected empty payload, got %d bytes", len(payload))
	}
}

func TestWriteFramedPacket_LengthEncoding(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte{0x10, 0x20, 0x30}
	if err := writeFramedPacket(&buf, payload); err != nil {
		t.Fatalf("writeFramedPacket returned error: %v", err)
	}

	var length uint32
	if err := binary.Read(&buf, binary.BigEndian, &length); err != nil && err != io.EOF {
		t.Fatalf("binary.Read returned error: %v", err)
	}
	if length != 3 {
		t.Fatalf("framed length = %d; want 3", length)
	}
}

func newTestSSHConfigs(t *testing.T) (*ssh.ServerConfig, *ssh.ClientConfig) {
	t.Helper()

	privateKey, err := util.GenerateED25519PrivateKey()
	if err != nil {
		t.Fatalf("GenerateED25519PrivateKey returned error: %v", err)
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

func TestHandleSSHConnection_RejectsDisallowedIP(t *testing.T) {
	serverCfg, clientCfg := newTestSSHConfigs(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen returned error: %v", err)
	}
	defer ln.Close()

	s := &ForwardServer{
		sshConfig:  serverCfg,
		allowedIPs: []string{"203.0.113.10"},
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
	defer client.Close()

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handleSSHConnection did not return after rejecting disallowed IP")
	}
}

func TestHandleChannel_InvalidProtocolLength(t *testing.T) {
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

	readCode := func() uint32 {
		t.Helper()
		var code uint32
		if err := binary.Read(clientChannel, binary.BigEndian, &code); err != nil {
			t.Fatalf("binary.Read returned error: %v", err)
		}
		return code
	}
	writeCode := func(v uint32) {
		t.Helper()
		if err := binary.Write(clientChannel, binary.BigEndian, v); err != nil {
			t.Fatalf("binary.Write returned error: %v", err)
		}
	}

	if got := readCode(); got != ErrSuccess {
		t.Fatalf("initial handshake code = %d; want %d", got, ErrSuccess)
	}
	writeCode(0)
	if got := readCode(); got != ErrSuccess {
		t.Fatalf("whitelist confirm code = %d; want %d", got, ErrSuccess)
	}
	writeCode(17)
	if got := readCode(); got != (ErrMask | ErrUnsupportedProtocol) {
		t.Fatalf("protocol error code = %08x; want %08x", got, ErrMask|ErrUnsupportedProtocol)
	}

	_ = clientChannel.Close()
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handleChannel did not return after invalid protocol length")
	}
}
