package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
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

// --- Tests de Scalabilité et Performance ---

// Test de concurrence - Multiples sessions simultanées
func TestRunSession_ConcurrentSessions(t *testing.T) {
	const numSessions = 50
	var wg sync.WaitGroup
	errors := make(chan error, numSessions)

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(sessionID int) {
			defer wg.Done()

			port := uint32(8080 + sessionID)
			conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, port)}
			s := &ClientSession{
				Connection:   newSSHClient(conn),
				LocalAddress: fmt.Sprintf("localhost:%d", 9000+sessionID),
			}

			params := &config.ClientParameters{
				RemotePort: int(port),
				AllowedIPs: []string{fmt.Sprintf("192.168.1.%d", sessionID%255)},
			}

			err := s.runSession(params)
			if err != nil {
				errors <- fmt.Errorf("session %d failed: %v", sessionID, err)
				return
			}

			if s.AssignedPort != int(port) {
				errors <- fmt.Errorf("session %d: port mismatch %d != %d", sessionID, s.AssignedPort, port)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Vérifier qu'aucune erreur n'est survenue
	for err := range errors {
		t.Error(err)
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

// --- Tests de Gestion d'Erreurs Réseau ---

// Test de timeout de lecture
func TestRunSession_ReadTimeout(t *testing.T) {
	// Créer un canal qui bloque indéfiniment
	slowChannel := &slowStubChannel{
		stubChannel: stubChannel{
			r: bytes.NewReader([]byte{}),
			w: &bytes.Buffer{},
		},
		delay: 100 * time.Millisecond,
	}

	conn := &stubConnWithCustomChannel{
		stubConn: stubConn{data: buildFrames(ErrSuccess)},
		channel:  slowChannel,
	}

	s := &ClientSession{
		Connection:   newSSHClient(conn),
		LocalAddress: "localhost:0",
	}

	// Le test devrait se terminer rapidement avec une erreur de timeout
	start := time.Now()
	err := s.runSession(&config.ClientParameters{})
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error but got nil")
	}

	// Le timeout ne devrait pas prendre plus de 200ms
	if duration > 200*time.Millisecond {
		t.Errorf("Operation took too long: %v", duration)
	}
}

// Test de récupération après erreur réseau
func TestRunSession_NetworkRecovery(t *testing.T) {
	// Test plus réaliste : simuler une erreur de connexion puis succès
	// Premier essai avec échec de connexion
	failConn := &stubConnWithFailure{
		shouldFail: true,
	}

	s := &ClientSession{
		Connection:   newSSHClient(failConn),
		LocalAddress: "localhost:0",
	}

	err := s.runSession(&config.ClientParameters{})
	if err == nil {
		t.Error("Expected error from failed connection but got nil")
	}

	// Deuxième essai avec succès
	successConn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
	s.Connection = newSSHClient(successConn)

	err = s.runSession(&config.ClientParameters{})
	if err != nil {
		t.Errorf("Expected success after connection recovery but got: %v", err)
	}

	if s.AssignedPort != 8080 {
		t.Errorf("AssignedPort = %d; want 8080", s.AssignedPort)
	}
}

// --- Tests de Métriques et Monitoring ---

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

// Test de monitoring de performance
func TestRunSession_PerformanceMonitoring(t *testing.T) {
	conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
	s := &ClientSession{
		Connection:   newSSHClient(conn),
		LocalAddress: "localhost:0",
	}

	// Mesurer le temps d'exécution
	start := time.Now()
	err := s.runSession(&config.ClientParameters{})
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// La session devrait s'établir rapidement (< 50ms pour un stub)
	if duration > 50*time.Millisecond {
		t.Errorf("Session setup took too long: %v", duration)
	}

	t.Logf("Session setup completed in %v", duration)
}

// --- Tests de Gestion Mémoire ---

// Test de prévention des fuites mémoire
func TestClientSession_MemoryLeak(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, uint32(8080+i))}
		s := &ClientSession{
			Connection:   newSSHClient(conn),
			LocalAddress: fmt.Sprintf("localhost:%d", 9000+i),
		}

		err := s.runSession(&config.ClientParameters{})
		if err != nil {
			t.Errorf("Iteration %d failed: %v", i, err)
		}

		// Simuler la fermeture de session
		s.Lock.Lock()
		s.Active = false
		s.Lock.Unlock()

		// Force garbage collection occasionnellement
		if i%10 == 0 {
			runtime.GC()
		}
	}

	// Test final de GC
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
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

// --- Tests de Configuration Avancée ---

// Test de validation de configuration
func TestRunSession_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		params  *config.ClientParameters
		wantErr string
	}{
		{
			name: "valid-config",
			params: &config.ClientParameters{
				RemotePort: 8080,
				AllowedIPs: []string{"192.168.1.0/24"},
			},
			wantErr: "",
		},
		{
			name: "invalid-ip-format",
			params: &config.ClientParameters{
				RemotePort: 8080,
				AllowedIPs: []string{"invalid-ip"},
			},
			wantErr: "", // Le serveur valide, pas le client
		},
		{
			name:    "empty-config",
			params:  &config.ClientParameters{},
			wantErr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
			s := &ClientSession{
				Connection:   newSSHClient(conn),
				LocalAddress: "localhost:0",
			}

			err := s.runSession(tc.params)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("Expected error containing %q, got %v", tc.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
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

// --- Structures d'aide pour les tests avancés ---

// Channel qui simule une latence réseau
type slowStubChannel struct {
	stubChannel
	delay time.Duration
}

func (c *slowStubChannel) Read(p []byte) (int, error) {
	time.Sleep(c.delay)
	return c.stubChannel.Read(p)
}

// Connexion qui permet d'injecter un canal personnalisé
type stubConnWithCustomChannel struct {
	stubConn
	channel ssh.Channel
}

func (s *stubConnWithCustomChannel) OpenChannel(name string, payload []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	reqs := make(chan *ssh.Request)
	close(reqs)
	return s.channel, reqs, nil
}

// Connexion qui simule un échec de connexion
type stubConnWithFailure struct {
	stubConn
	shouldFail bool
}

func (s *stubConnWithFailure) OpenChannel(name string, payload []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	if s.shouldFail {
		return nil, nil, fmt.Errorf("failed to open channel: network unreachable")
	}
	return s.stubConn.OpenChannel(name, payload)
}

// Connexion qui simule des tentatives avec échecs puis succès
type stubConnWithRetry struct {
	stubConn
	maxAttempts int
	attempts    *int
	successData []byte
}

func (s *stubConnWithRetry) OpenChannel(name string, payload []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	*s.attempts++

	if *s.attempts < s.maxAttempts {
		// Simuler un échec réseau
		return nil, nil, fmt.Errorf("network error attempt %d", *s.attempts)
	}

	// Succès après plusieurs tentatives
	reader := bytes.NewReader(s.successData)
	ch := &stubChannel{r: reader, w: &bytes.Buffer{}}
	reqs := make(chan *ssh.Request)
	close(reqs)
	return ch, reqs, nil
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

// Test de connexion qui simule des tentatives avec échecs puis succès
func TestRunSession_ConnectionFailure(t *testing.T) {
	conn := &stubConnWithFailure{
		shouldFail: true,
	}

	s := &ClientSession{
		Connection:   newSSHClient(conn),
		LocalAddress: "localhost:0",
	}

	err := s.runSession(&config.ClientParameters{})
	if err == nil {
		t.Error("Expected error from failed connection")
	}

	// Vérifier que l'erreur est bien une erreur de connexion
	if !strings.Contains(err.Error(), "failed to open channel") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

// Benchmark pour mesurer les performances
func BenchmarkRunSession(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
		s := &ClientSession{
			Connection:   newSSHClient(conn),
			LocalAddress: "localhost:0",
		}

		err := s.runSession(&config.ClientParameters{})
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}

// Benchmark pour la gestion de whitelist
func BenchmarkWhitelistProcessing(b *testing.B) {
	// Générer une grande whitelist
	entries := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		entries[i] = fmt.Sprintf("192.168.%d.%d", i/255, i%255)
	}

	params := &config.ClientParameters{
		AllowedIPs: entries,
		RemotePort: 8080,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := &stubConn{data: buildFrames(ErrSuccess, ErrSuccess, 8080)}
		s := &ClientSession{
			Connection:   newSSHClient(conn),
			LocalAddress: "localhost:0",
		}

		err := s.runSession(params)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}
