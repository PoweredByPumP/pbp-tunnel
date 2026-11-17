package client

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

func TestRun_InvalidParameters(t *testing.T) {
	cp := &config.ClientParameters{}
	err := Run(cp)
	if err == nil || !strings.Contains(err.Error(), "invalid client parameters") {
		t.Fatalf("Run() error = %v; want invalid client parameters", err)
	}
}

func TestRunSession_StoresExposedPort(t *testing.T) {
	session := newTestSession(func(channel io.ReadWriter) {
		if _, err := protocol.ReceiveTunnelRequest(channel); err == nil {
			_ = protocol.AcceptTunnel(channel, 4242)
		}
	})

	if err := session.runSession(&config.ClientParameters{Protocol: "tcp"}); err != nil {
		t.Fatalf("runSession: %v", err)
	}
	if session.AssignedPort != 4242 {
		t.Errorf("AssignedPort = %d, want 4242", session.AssignedPort)
	}
}

func TestRunSession_ReturnsServerRefusal(t *testing.T) {
	session := newTestSession(func(channel io.ReadWriter) {
		if _, err := protocol.ReceiveTunnelRequest(channel); err == nil {
			_ = protocol.RefuseTunnel(channel, protocol.StatusPortUnavailable)
		}
	})

	err := session.runSession(&config.ClientParameters{Protocol: "tcp"})
	if !errors.Is(err, protocol.StatusPortUnavailable) {
		t.Errorf("runSession error = %v, want %v", err, protocol.StatusPortUnavailable)
	}
}

func TestRunSession_RejectsUnsupportedTransportBeforeContactingServer(t *testing.T) {
	session := newTestSession(func(io.ReadWriter) {
		t.Error("the server should not be contacted")
	})

	err := session.runSession(&config.ClientParameters{Protocol: "quic"})
	if !errors.Is(err, protocol.StatusUnsupportedTransport) {
		t.Errorf("runSession error = %v, want %v", err, protocol.StatusUnsupportedTransport)
	}
}

func TestRunSession_FailsWhenRequestChannelCannotOpen(t *testing.T) {
	session := &ClientSession{
		Connection:   newSSHClient(&stubConn{openChannelErr: errors.New("network unreachable")}),
		LocalAddress: "localhost:0",
	}

	err := session.runSession(&config.ClientParameters{Protocol: "tcp"})
	if err == nil || !strings.Contains(err.Error(), "network unreachable") {
		t.Errorf("runSession error = %v, want network unreachable", err)
	}
}

func newTestSession(fakeServer func(channel io.ReadWriter)) *ClientSession {
	return &ClientSession{
		Connection:   newSSHClient(&stubConn{fakeServer: fakeServer}),
		LocalAddress: "localhost:0",
	}
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

// Test de stress - Nombreuses connexions sur une session
func TestClientSession_StressTest(t *testing.T) {
	s := &ClientSession{
		LocalAddress: "localhost:8080",
		Active:       true,
	}

	const numConnections = 1000
	var wg sync.WaitGroup

	// Simuler de nombreuses connexions concurrentes
	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			s.Lock.Lock()
			s.ConnectionCount++
			count := s.ConnectionCount
			s.Lock.Unlock()

			s.ActiveConnections.Add(1)

			// Simuler du travail
			time.Sleep(time.Microsecond)

			s.ActiveConnections.Done()

			if count > numConnections {
				t.Errorf("Connection count exceeded expected maximum: %d", count)
			}
		}()
	}

	wg.Wait()

	// Vérifier l'état final
	s.Lock.Lock()
	finalCount := s.ConnectionCount
	s.Lock.Unlock()

	if finalCount != numConnections {
		t.Errorf("Final connection count = %d; want %d", finalCount, numConnections)
	}
}

// Test de collecte de métriques
func TestClientSession_Metrics(t *testing.T) {
	s := &ClientSession{
		LocalAddress:    "localhost:8080",
		Active:          true,
		ConnectionCount: 0,
		AssignedPort:    8080,
	}

	// Métriques initiales
	metrics := s.GetMetrics()
	expectedMetrics := map[string]interface{}{
		"local_address":    "localhost:8080",
		"active":           true,
		"connection_count": 0,
		"assigned_port":    8080,
	}

	for key, expected := range expectedMetrics {
		if metrics[key] != expected {
			t.Errorf("Metric %s = %v; want %v", key, metrics[key], expected)
		}
	}

	// Simuler quelques connexions
	s.Lock.Lock()
	s.ConnectionCount = 5
	s.Lock.Unlock()

	updatedMetrics := s.GetMetrics()
	if updatedMetrics["connection_count"] != 5 {
		t.Errorf("Updated connection_count = %v; want 5", updatedMetrics["connection_count"])
	}
}

// Test de nettoyage des ressources
func TestClientSession_ResourceCleanup(t *testing.T) {
	s := &ClientSession{
		LocalAddress: "localhost:8080",
		Active:       true,
	}

	// Ajouter quelques connexions actives
	for i := 0; i < 5; i++ {
		s.ActiveConnections.Add(1)
		s.Lock.Lock()
		s.ConnectionCount++
		s.Lock.Unlock()
	}

	// Simuler l'arrêt de la session
	s.Lock.Lock()
	s.Active = false
	s.Lock.Unlock()

	// Nettoyer les connexions
	for i := 0; i < 5; i++ {
		s.ActiveConnections.Done()
	}

	// Attendre que toutes les connexions se terminent
	done := make(chan struct{})
	go func() {
		s.ActiveConnections.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Succès
	case <-time.After(100 * time.Millisecond):
		t.Error("Resource cleanup took too long")
	}
}

// Test de configuration dynamique
func TestClientSession_DynamicConfiguration(t *testing.T) {
	s := &ClientSession{
		LocalAddress: "localhost:8080",
		Active:       true,
	}

	// Simuler des changements de configuration pendant l'exécution
	configs := []struct {
		active bool
		port   int
	}{
		{true, 8080},
		{true, 8081},
		{false, 8081},
		{true, 8082},
	}

	for i, cfg := range configs {
		s.Lock.Lock()
		s.Active = cfg.active
		s.AssignedPort = cfg.port
		s.Lock.Unlock()

		// Vérifier l'état
		if s.Active != cfg.active {
			t.Errorf("Config %d: Active = %v; want %v", i, s.Active, cfg.active)
		}
		if s.AssignedPort != cfg.port {
			t.Errorf("Config %d: AssignedPort = %d; want %d", i, s.AssignedPort, cfg.port)
		}
	}
}

// Extension de ClientSession pour les métriques
func (s *ClientSession) GetMetrics() map[string]interface{} {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	return map[string]interface{}{
		"local_address":    s.LocalAddress,
		"active":           s.Active,
		"connection_count": s.ConnectionCount,
		"assigned_port":    s.AssignedPort,
	}
}
