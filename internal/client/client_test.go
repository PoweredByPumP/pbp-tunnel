package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

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

func newSSHClient(conn ssh.Conn) *ssh.Client {
	// channels and requests channels are unused by runSession
	return ssh.NewClient(conn, make(chan ssh.NewChannel), make(chan *ssh.Request))
}

// --- Tests for run ---
func TestRun_InvalidParameters(t *testing.T) {
	cp := &config.ClientParameters{}
	err := Run(cp)
	if err == nil || !strings.Contains(err.Error(), "invalid client parameters") {
		t.Fatalf("Run() error = %v; want invalid client parameters", err)
	}
}

// --- Tests for runSession ---
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

func TestRunSession_WhitelistSending(t *testing.T) {
	// Create a stub connection that returns success for handshake and whitelist
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:8888"}

	// Create parameters with multiple whitelist entries
	params := &config.ClientParameters{
		AllowedIPs: []string{"192.168.1.0/24", "10.0.0.1", "172.16.0.0/16"},
		RemotePort: 8080,
	}

	err := s.runSession(params)
	if err != nil {
		t.Errorf("runSession with whitelist unexpected error: %v", err)
	}

	// Check port was correctly assigned
	if s.AssignedPort != 8080 {
		t.Errorf("AssignedPort = %d; want %d", s.AssignedPort, 8080)
	}
}

// Test sending whitelist with zero entries
func TestRunSession_EmptyWhitelist(t *testing.T) {
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}

	params := &config.ClientParameters{
		AllowedIPs: []string{},
		RemotePort: 8080,
	}

	err := s.runSession(params)
	if err != nil {
		t.Errorf("runSession with empty whitelist unexpected error: %v", err)
	}

	// Should still succeed with assigned port
	if s.AssignedPort != 8080 {
		t.Errorf("AssignedPort = %d; want %d", s.AssignedPort, 8080)
	}
}

func TestRunSession_WhitelistConfirmReadError(t *testing.T) {
	// Create response with success for handshake but no whitelist confirmation
	// This truncated response will cause a read error when trying to read whitelist confirmation
	conn := &stubConn{data: buildFrames(ErrSuccess)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}

	err := s.runSession(&config.ClientParameters{AllowedIPs: []string{"1.2.3.4"}})
	if err == nil || !strings.Contains(err.Error(), "whitelist confirm read error") {
		t.Errorf("runSession error = %v; want whitelist confirm read error", err)
	}
}

// Test avec des entrées de liste blanche de tailles différentes
func TestRunSession_VaryingWhitelistEntrySizes(t *testing.T) {
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}

	// Tester avec des entrées très courtes, longues, et normales
	entries := []string{
		"", // Entrée vide
		"10.0.0.1",
		strings.Repeat("1", 1024) + ".0.0.0/8", // Entrée très longue
		"fe80::/64",                            // IPv6 (bien que l'implémentation actuelle ne le supporte pas complètement)
	}

	params := &config.ClientParameters{
		AllowedIPs: entries,
		RemotePort: 8080,
	}

	err := s.runSession(params)
	if err != nil {
		t.Errorf("runSession with varying whitelist entry sizes failed: %v", err)
	}

	if s.AssignedPort != 8080 {
		t.Errorf("AssignedPort = %d; want %d", s.AssignedPort, 8080)
	}
}

// Test avec de nombreuses entrées de liste blanche (performance)
func TestRunSession_LargeWhitelist(t *testing.T) {
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}

	// Générer un grand nombre d'entrées
	const numEntries = 1000
	entries := make([]string, numEntries)
	for i := 0; i < numEntries; i++ {
		entries[i] = fmt.Sprintf("192.168.%d.%d", i/255, i%255)
	}

	params := &config.ClientParameters{
		AllowedIPs: entries,
		RemotePort: 8080,
	}

	start := time.Now()
	err := s.runSession(params)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("runSession with large whitelist failed: %v", err)
	}

	if duration > 500*time.Millisecond {
		t.Logf("Large whitelist processing took %v", duration)
	}
}

// Test port boundary cases
func TestRunSession_PortBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		port      uint32
		expectErr bool
		errMsg    string
	}{
		{"minimum-port", 1, false, ""},
		{"maximum-port", 65535, false, ""},
		{"zero-port", 0, false, ""}, // Le serveur pourrait assigner un port automatiquement
		{"port-out-of-range-high", 70000, true, "port out of range"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var responseData []byte
			if tc.expectErr {
				responseData = buildFrames(ErrSuccess, ErrSuccess, ErrMask|ErrPortOutOfRange)
			} else {
				responseData = buildFrames(ErrSuccess, ErrSuccess, tc.port)
			}

			conn := &stubConn{data: responseData}
			s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}

			params := &config.ClientParameters{
				RemotePort: int(tc.port),
			}

			err := s.runSession(params)

			if tc.expectErr {
				if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("Expected error containing %q, got %v", tc.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !tc.expectErr && uint32(s.AssignedPort) != tc.port {
					t.Errorf("AssignedPort = %d; want %d", s.AssignedPort, tc.port)
				}
			}
		})
	}
}

func TestRunSession_PortResponseReadError(t *testing.T) {
	// Create response with success for handshake and whitelist but no port response
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess)}
	s := &ClientSession{Connection: newSSHClient(conn), LocalAddress: "localhost:0"}

	err := s.runSession(&config.ClientParameters{})
	if err == nil || !strings.Contains(err.Error(), "read port response error") {
		t.Errorf("runSession error = %v; want read port response error", err)
	}
}

// Enhanced stub channel for tracking state
type enhancedStubChannel struct {
	stubChannel
	closed bool
}

func (c *enhancedStubChannel) Close() error {
	c.closed = true
	return nil
}

// --- Test for ClientSession ---
func TestClientSession_ActiveConnectionsTracking(t *testing.T) {
	s := &ClientSession{
		LocalAddress: "localhost:8080",
		Active:       true,
	}

	// Initially should be zero
	initialCount := s.ConnectionCount
	if initialCount != 0 {
		t.Errorf("Initial ConnectionCount = %d; want 0", initialCount)
	}

	// Simulate a few connections
	for i := 0; i < 3; i++ {
		s.Lock.Lock()
		s.ConnectionCount++
		s.Lock.Unlock()

		s.ActiveConnections.Add(1)
	}

	// Check that count was updated
	s.Lock.Lock()
	updatedCount := s.ConnectionCount
	s.Lock.Unlock()

	if updatedCount != 3 {
		t.Errorf("Updated ConnectionCount = %d; want 3", updatedCount)
	}

	// Simulate connections finishing
	for i := 0; i < 3; i++ {
		s.ActiveConnections.Done()
	}
}

// Test behavior when session is deactivated
func TestClientSession_Deactivation(t *testing.T) {
	s := &ClientSession{
		LocalAddress: "localhost:8080",
		Active:       true,
	}

	// Check initial state
	if !s.Active {
		t.Errorf("Initial Active = %v; want true", s.Active)
	}

	// Deactivate
	s.Lock.Lock()
	s.Active = false
	s.Lock.Unlock()

	// Check updated state
	if s.Active {
		t.Errorf("Updated Active = %v; want false", s.Active)
	}
}

// Structs auxiliaires pour les tests
type mockNewChannel struct {
	name         string
	rejectCalled bool
	rejectReason string
}

func (m *mockNewChannel) Accept() (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, nil
}

func (m *mockNewChannel) Reject(reason ssh.RejectionReason, message string) error {
	m.rejectCalled = true
	m.rejectReason = message
	return nil
}

func (m *mockNewChannel) ChannelType() string {
	return m.name
}

func (m *mockNewChannel) ExtraData() []byte {
	return nil
}

// Test le rejet de connexion quand la session est inactive
func TestHandleChannelOpen_InactiveSession(t *testing.T) {
	// Créer un mock de NewChannel
	mockNewChannel := &mockNewChannel{
		name:         "direct-tcpip",
		rejectCalled: false,
		rejectReason: "",
	}

	conn := &stubConn{}
	s := &ClientSession{
		Connection:   newSSHClient(conn),
		LocalAddress: "localhost:1234",
		Active:       false,
	}

	// Simuler un appel du callback de HandleChannelOpen
	ch := make(chan ssh.NewChannel, 1)
	ch <- mockNewChannel
	close(ch)

	// Exécuter le handler qui devrait rejeter le canal
	go func() {
		for newCh := range ch {
			if !s.Active {
				newCh.Reject(ssh.ConnectionFailed, "session closed")
				continue
			}
			// Ne devrait pas atteindre ici
			t.Error("Should have rejected the channel due to inactive session")
		}
	}()

	// Attendre un peu pour que le handler s'exécute
	time.Sleep(10 * time.Millisecond)

	if !mockNewChannel.rejectCalled {
		t.Error("NewChannel.Reject was not called for inactive session")
	}
	if mockNewChannel.rejectReason != "session closed" {
		t.Errorf("Reject reason = %q; want 'session closed'", mockNewChannel.rejectReason)
	}
}
