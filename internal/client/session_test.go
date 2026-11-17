package client

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

func TestOpenTunnel_ReturnsTheExposedPort(t *testing.T) {
	s := newTestSession(func(channel io.ReadWriter) {
		if _, err := protocol.ReceiveTunnelRequest(channel); err == nil {
			_ = protocol.AcceptTunnel(channel, 4242)
		}
	})

	exposedPort, err := s.openTunnel(protocol.TunnelRequest{Transport: protocol.TCP})

	if err != nil || exposedPort != 4242 {
		t.Errorf("openTunnel = %d, %v; want 4242, nil", exposedPort, err)
	}
}

func TestOpenTunnel_ReturnsTheServerRefusal(t *testing.T) {
	s := newTestSession(func(channel io.ReadWriter) {
		if _, err := protocol.ReceiveTunnelRequest(channel); err == nil {
			_ = protocol.RefuseTunnel(channel, protocol.StatusPortUnavailable)
		}
	})

	_, err := s.openTunnel(protocol.TunnelRequest{Transport: protocol.TCP})

	if !errors.Is(err, protocol.StatusPortUnavailable) {
		t.Errorf("openTunnel error = %v, want %v", err, protocol.StatusPortUnavailable)
	}
}

func TestOpenTunnel_FailsWhenRequestChannelCannotOpen(t *testing.T) {
	s := &session{sshClient: newSSHClient(&stubConn{openChannelErr: errors.New("network unreachable")})}

	_, err := s.openTunnel(protocol.TunnelRequest{Transport: protocol.TCP})

	if err == nil || !strings.Contains(err.Error(), "network unreachable") {
		t.Errorf("openTunnel error = %v, want network unreachable", err)
	}
}

func newTestSession(fakeServer func(channel io.ReadWriter)) *session {
	return &session{sshClient: newSSHClient(&stubConn{fakeServer: fakeServer}), localAddress: "localhost:0"}
}

func newSSHClient(conn ssh.Conn) *ssh.Client {
	return ssh.NewClient(conn, make(chan ssh.NewChannel), make(chan *ssh.Request))
}

// stubConn is an SSH connection whose channels are answered by fakeServer.
type stubConn struct {
	fakeServer     func(channel io.ReadWriter)
	openChannelErr error
}

func (s *stubConn) OpenChannel(name string, payload []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	if s.openChannelErr != nil {
		return nil, nil, s.openChannelErr
	}
	clientSide, serverSide := net.Pipe()
	go func() {
		defer serverSide.Close()
		s.fakeServer(serverSide)
	}()
	requests := make(chan *ssh.Request)
	close(requests)
	return &pipeChannel{Conn: clientSide}, requests, nil
}

func (s *stubConn) SendRequest(string, bool, []byte) (bool, []byte, error) { return false, nil, nil }
func (s *stubConn) Close() error                                           { return nil }
func (s *stubConn) Wait() error                                            { return nil }
func (s *stubConn) User() string                                           { return "" }
func (s *stubConn) SessionID() []byte                                      { return nil }
func (s *stubConn) ClientVersion() []byte                                  { return nil }
func (s *stubConn) ServerVersion() []byte                                  { return nil }
func (s *stubConn) RemoteAddr() net.Addr                                   { return &net.TCPAddr{} }
func (s *stubConn) LocalAddr() net.Addr                                    { return &net.TCPAddr{} }
func (s *stubConn) Permissions() *ssh.Permissions                          { return nil }

// pipeChannel turns one end of a net.Pipe into an ssh.Channel.
type pipeChannel struct{ net.Conn }

func (c *pipeChannel) CloseWrite() error                              { return nil }
func (c *pipeChannel) SendRequest(string, bool, []byte) (bool, error) { return false, nil }
func (c *pipeChannel) Stderr() io.ReadWriter                          { return c.Conn }
