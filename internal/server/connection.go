package server

import (
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

// serveConnection runs the SSH handshake, then answers the tunnel requests of one client.
func (s *Server) serveConnection(rawConn net.Conn) {
	defer rawConn.Close()
	sshConn, newChannels, requests, err := ssh.NewServerConn(rawConn, s.sshConfig)
	if err != nil {
		log.Printf("[-] SSH handshake failed: %v", err)
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(requests)

	log.Printf("[+] New SSH connection from %s", sshConn.RemoteAddr())
	// A request is handled until its tunnel closes, so a client holds one tunnel at a time.
	for newChannel := range newChannels {
		if newChannel.ChannelType() != "direct-tcpip" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			log.Printf("[-] Accept channel failed: %v", err)
			continue
		}
		go ssh.DiscardRequests(channelRequests)
		s.handleTunnelRequest(sshConn, channel)
	}
}
