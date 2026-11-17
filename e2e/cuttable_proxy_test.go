package e2e

import (
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
)

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
