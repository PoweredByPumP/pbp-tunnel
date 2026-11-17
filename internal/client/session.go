package client

import (
	"fmt"
	"log"
	"sync"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

// session is one SSH connection to the server and the tunnel opened through it.
type session struct {
	sshClient    *ssh.Client
	localAddress string
	forwards     sync.WaitGroup
}

// runSession opens the tunnel, relays the forwarded connections until the SSH
// connection ends, then waits for the relays to finish.
func runSession(sshClient *ssh.Client, request protocol.TunnelRequest, localAddress string) error {
	defer sshClient.Close()
	s := &session{sshClient: sshClient, localAddress: localAddress}

	// Registered before the tunnel opens, so that no forwarded connection is missed.
	forwardedChannels := sshClient.HandleChannelOpen("direct-tcpip")
	if _, err := s.openTunnel(request); err != nil {
		return err
	}

	relayLoopDone := make(chan struct{})
	go func() {
		defer close(relayLoopDone)
		s.relayForwardedConnections(forwardedChannels, request.Transport)
	}()
	err := sshClient.Wait()
	// No relay may start once forwards.Wait has begun.
	<-relayLoopDone
	s.forwards.Wait()
	return err
}

// openTunnel sends the tunnel request and returns the port the server exposed.
// The request channel stays open as long as the tunnel and closes with the SSH connection.
func (s *session) openTunnel(request protocol.TunnelRequest) (exposedPort int, err error) {
	requestChannel, requests, err := s.sshClient.OpenChannel("direct-tcpip", nil)
	if err != nil {
		return 0, fmt.Errorf("open tunnel request channel: %w", err)
	}
	go ssh.DiscardRequests(requests)

	log.Printf("[*] Requesting %s tunnel on remote port %d (allowed peers: %v)", request.Transport, request.Port, request.AllowedPeers)
	exposedPort, err = protocol.RequestTunnel(requestChannel, request)
	if err != nil {
		return 0, fmt.Errorf("tunnel request: %w", err)
	}
	log.Printf("[+] Assigned remote port %d (local %s)", exposedPort, s.localAddress)
	return exposedPort, nil
}

// relayForwardedConnections returns once the SSH connection has ended.
func (s *session) relayForwardedConnections(forwardedChannels <-chan ssh.NewChannel, transport protocol.Transport) {
	forwardID := 0
	for newChannel := range forwardedChannels {
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("[-] Accept forwarded channel: %v", err)
			continue
		}
		go ssh.DiscardRequests(requests)

		forwardID++
		id := forwardID
		log.Printf("[*] Forward #%d incoming", id)
		s.forwards.Go(func() {
			defer channel.Close()
			if transport == protocol.UDP {
				forwardUDP(channel, s.localAddress, id)
			} else {
				forwardTCP(channel, s.localAddress, id)
			}
		})
	}
}
