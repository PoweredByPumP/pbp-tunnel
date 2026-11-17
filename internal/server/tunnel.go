package server

import (
	"log"
	"net"
	"strconv"

	"github.com/poweredbypump/pbp-tunnel/internal/allowlist"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

// exposedPort is the port reserved for one client's tunnel.
type exposedPort struct {
	sshConn *ssh.ServerConn
	// requestChannel carries the tunnel request, which is answered once the port listens.
	requestChannel ssh.Channel
	address        string
	port           int
	allowedPeers   allowlist.List
}

// handleTunnelRequest answers a tunnel request, then forwards traffic until the
// client disconnects.
func (s *Server) handleTunnelRequest(sshConn *ssh.ServerConn, channel ssh.Channel) {
	defer channel.Close()

	client := sshConn.RemoteAddr()
	if !s.allowedClients.Allows(client) {
		_ = protocol.RejectPeer(channel)
		log.Printf("[-] Tunnel request from %s rejected: IP not allowed", client)
		return
	}

	request, err := protocol.ReceiveTunnelRequest(channel)
	if err != nil {
		log.Printf("[-] Tunnel request from %s failed: %v", client, err)
		return
	}
	log.Printf("[*] Tunnel request from %s: transport=%s port=%d allowed peers=%v",
		client, request.Transport, request.Port, request.AllowedPeers)

	port, err := s.ports.Reserve(request.Port)
	if err != nil {
		_ = protocol.RefuseTunnel(channel, err)
		log.Printf("[-] Port assignment failed: %v", err)
		return
	}
	log.Printf("[+] Assigned port %d", port)
	defer func() {
		s.ports.Release(port)
		log.Printf("[*] Client disconnected, freed port %d", port)
	}()

	exposed := exposedPort{
		sshConn:        sshConn,
		requestChannel: channel,
		address:        net.JoinHostPort(s.bindAddress, strconv.Itoa(port)),
		port:           port,
		allowedPeers:   request.AllowedPeers,
	}
	if request.Transport == protocol.UDP {
		exposeUDP(exposed)
	} else {
		exposeTCP(exposed)
	}
}
