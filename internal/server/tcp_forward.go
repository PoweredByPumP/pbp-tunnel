package server

import (
	"errors"
	"log"
	"net"
	"sync"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"github.com/poweredbypump/pbp-tunnel/internal/relay"
	"golang.org/x/crypto/ssh"
)

// exposeTCP relays every allowed connection on the exposed port to the client,
// through a new SSH channel each, until the SSH connection ends.
func exposeTCP(exposed exposedPort) {
	listener, err := net.Listen("tcp", exposed.address)
	if err != nil {
		_ = protocol.RefuseTunnel(exposed.requestChannel, protocol.StatusInternalError)
		log.Printf("[-] Bind TCP port %d failed: %v", exposed.port, err)
		return
	}
	defer listener.Close()

	_ = protocol.AcceptTunnel(exposed.requestChannel, exposed.port)
	log.Printf("[+] Notified client of TCP port %d", exposed.port)

	go func() {
		_ = exposed.sshConn.Wait()
		listener.Close()
	}()

	connections := newRelayedConnections()
	defer connections.closeAllAndWait()

	for relayID := 1; ; relayID++ {
		peerConn, err := listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("[-] Forward accept error: %v", err)
			}
			return
		}
		if !exposed.allowedPeers.Allows(peerConn.RemoteAddr()) {
			log.Printf("[-] Connection from %s rejected: not in allowed peers", peerConn.RemoteAddr())
			peerConn.Close()
			continue
		}
		connections.relay(peerConn, func() { relayToClient(exposed.sshConn, peerConn.(*net.TCPConn), relayID) })
	}
}

func relayToClient(sshConn ssh.Conn, peerConn *net.TCPConn, relayID int) {
	channel, requests, err := sshConn.OpenChannel("direct-tcpip", nil)
	if err != nil {
		log.Printf("[-] Open back-channel failed: %v", err)
		return
	}
	go ssh.DiscardRequests(requests)
	defer channel.Close()

	toClient, toPeer := relay.CopyBothWays(peerConn, channel)
	log.Printf("[+] Forward %d closed: %d bytes to client, %d bytes to peer", relayID, toClient, toPeer)
}

// relayedConnections closes every relayed connection once the tunnel ends;
// otherwise an idle peer would keep its relay running and the port reserved.
type relayedConnections struct {
	mu     sync.Mutex
	open   map[net.Conn]bool
	relays sync.WaitGroup
}

func newRelayedConnections() *relayedConnections {
	return &relayedConnections{open: make(map[net.Conn]bool)}
}

// relay runs relayConn in the background and closes conn when it returns.
func (c *relayedConnections) relay(conn net.Conn, relayConn func()) {
	c.mu.Lock()
	c.open[conn] = true
	c.mu.Unlock()

	c.relays.Go(func() {
		defer func() {
			conn.Close()
			c.mu.Lock()
			delete(c.open, conn)
			c.mu.Unlock()
		}()
		relayConn()
	})
}

func (c *relayedConnections) closeAllAndWait() {
	c.mu.Lock()
	for conn := range c.open {
		conn.Close()
	}
	c.mu.Unlock()
	c.relays.Wait()
}
