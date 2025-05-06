package client

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
)

// stubAddr implements net.Addr for ssh.Conn methods
type stubAddr struct{}

func (stubAddr) Network() string { return "tcp" }
func (stubAddr) String() string  { return "stub" }

// stubChannel implements ssh.Channel for testing runSession
type stubChannel struct {
	r *bytes.Reader
	w *bytes.Buffer
}

func (c *stubChannel) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *stubChannel) Write(p []byte) (int, error) { return c.w.Write(p) }
func (c *stubChannel) Close() error                { return nil }
func (c *stubChannel) CloseWrite() error           { return nil }
func (c *stubChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, nil
}
func (c *stubChannel) Stderr() io.ReadWriter { return c.w }

// stubConn implements ssh.Conn for testing runSession
type stubConn struct {
	data []byte
}

// SendRequest satisfies ssh.Conn interface for global requests
func (s *stubConn) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	return false, nil, nil
}
func (s *stubConn) OpenChannel(name string, payload []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	reader := bytes.NewReader(s.data)
	ch := &stubChannel{r: reader, w: &bytes.Buffer{}}
	reqs := make(chan *ssh.Request)
	close(reqs)
	return ch, reqs, nil
}
func (s *stubConn) Close() error                  { return nil }
func (s *stubConn) Wait() error                   { return nil }
func (s *stubConn) User() string                  { return "" }
func (s *stubConn) SessionID() []byte             { return nil }
func (s *stubConn) ClientVersion() []byte         { return nil }
func (s *stubConn) ServerVersion() []byte         { return nil }
func (s *stubConn) RemoteAddr() net.Addr          { return stubAddr{} }
func (s *stubConn) LocalAddr() net.Addr           { return stubAddr{} }
func (s *stubConn) Permissions() *ssh.Permissions { return nil }

// helper to build handshake frames
func buildFrames(frames ...uint32) []byte {
	buf := &bytes.Buffer{}
	for _, f := range frames {
		_ = binary.Write(buf, binary.BigEndian, f)
	}
	return buf.Bytes()
}

func TestRun_InvalidParameters(t *testing.T) {
	cp := &config.ClientParameters{}
	err := Run(cp)
	if err == nil || !strings.Contains(err.Error(), "invalid client parameters") {
		t.Fatalf("Run() error = %v; want invalid client parameters", err)
	}
}

func newSSHClient(conn ssh.Conn) *ssh.Client {
	// channels and requests channels are unused by runSession
	return ssh.NewClient(conn, make(chan ssh.NewChannel), make(chan *ssh.Request))
}

func TestRunSession_HandshakeReadError(t *testing.T) {
	conn := &stubConn{data: []byte{}}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "handshake read error") {
		t.Errorf("runSession error = %v; want handshake read error", err)
	}
}

func TestRunSession_IPNotAllowed(t *testing.T) {
	conn := &stubConn{data: buildFrames(ErrIPNotAllowed)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "server rejected IP") {
		t.Errorf("runSession error = %v; want server rejected IP", err)
	}
}

func TestRunSession_HandshakeFailed(t *testing.T) {
	conn := &stubConn{data: buildFrames(99)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "handshake failed with code 99") {
		t.Errorf("runSession error = %v; want handshake failed with code 99", err)
	}
}

func TestRunSession_WhitelistRejected(t *testing.T) {
	conn := &stubConn{data: buildFrames(ErrSuccess, 1)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{AllowedIPs: []string{"1.2.3.4"}})
	if err == nil || !strings.Contains(err.Error(), "whitelist rejected by server") {
		t.Errorf("runSession error = %v; want whitelist rejected by server", err)
	}
}

func TestRunSession_PortUnavailable(t *testing.T) {
	mask := ErrMask | ErrPortUnavailable
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, mask)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "no available ports") {
		t.Errorf("runSession error = %v; want no available ports", err)
	}
}

func TestRunSession_PortOutOfRange(t *testing.T) {
	mask := ErrMask | ErrPortOutOfRange
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, mask)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "port out of range") {
		t.Errorf("runSession error = %v; want port out of range", err)
	}
}

func TestRunSession_InternalError(t *testing.T) {
	mask := ErrMask | ErrInternal
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, mask)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "internal error") {
		t.Errorf("runSession error = %v; want internal error", err)
	}
}

func TestRunSession_UnknownServerError(t *testing.T) {
	mask := ErrMask | 42
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, mask)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "server error code 42") {
		t.Errorf("runSession error = %v; want server error code 42", err)
	}
}

func TestRunSession_Success(t *testing.T) {
	port := uint32(4242)
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, port)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}
	err := s.runSession(&config.ClientParameters{})
	if err != nil {
		t.Errorf("runSession unexpected error: %v", err)
	}
	if s.AssignedPort != int(port) {
		t.Errorf("AssignedPort = %d; want %d", s.AssignedPort, port)
	}
}
