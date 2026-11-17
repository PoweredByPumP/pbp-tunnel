package server

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

// exposeUDP relays the datagrams received on the exposed port to the client until
// the SSH connection ends.
func exposeUDP(exposed exposedPort) {
	packetConn, err := net.ListenPacket("udp", exposed.address)
	if err != nil {
		_ = protocol.RefuseTunnel(exposed.requestChannel, protocol.StatusInternalError)
		log.Printf("[-] Bind UDP port %d failed: %v", exposed.port, err)
		return
	}
	defer packetConn.Close()

	_ = protocol.AcceptTunnel(exposed.requestChannel, exposed.port)
	log.Printf("[+] Notified client of UDP port %d", exposed.port)

	go func() {
		_ = exposed.sshConn.Wait()
		packetConn.Close()
	}()

	forward := &udpForward{sshConn: exposed.sshConn, packetConn: packetConn, peerChannels: make(map[string]ssh.Channel)}
	defer forward.closeAllChannels()

	datagram := make([]byte, protocol.MaxDatagramSize)
	for {
		n, peer, err := packetConn.ReadFrom(datagram)
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			log.Printf("[-] UDP read failed on port %d: %v", exposed.port, err)
			continue
		}
		if exposed.allowedPeers.Allows(peer) {
			forward.sendToClient(peer, datagram[:n])
		}
	}
}

// udpForward gives each remote peer its own SSH channel, so that the replies of
// the client go back to the peer that sent the request.
type udpForward struct {
	sshConn    ssh.Conn
	packetConn net.PacketConn

	mu           sync.Mutex
	peerChannels map[string]ssh.Channel
}

func (f *udpForward) sendToClient(peer net.Addr, datagram []byte) {
	channel, err := f.channelFor(peer)
	if err != nil {
		log.Printf("[-] Open UDP back-channel failed: %v", err)
		return
	}
	if err := protocol.WriteDatagram(channel, datagram); err != nil {
		log.Printf("[-] UDP write to client failed (%s): %v", peer, err)
		f.closeChannel(peer, channel)
	}
}

// channelFor returns the channel of peer, opening it on the first datagram.
func (f *udpForward) channelFor(peer net.Addr) (ssh.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if channel, ok := f.peerChannels[peer.String()]; ok {
		return channel, nil
	}
	channel, requests, err := f.sshConn.OpenChannel("direct-tcpip", nil)
	if err != nil {
		return nil, err
	}
	go ssh.DiscardRequests(requests)
	f.peerChannels[peer.String()] = channel
	go f.sendRepliesToPeer(peer, channel)
	return channel, nil
}

func (f *udpForward) sendRepliesToPeer(peer net.Addr, channel ssh.Channel) {
	defer f.closeChannel(peer, channel)
	for {
		reply, err := protocol.ReadDatagram(channel)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("[-] UDP read from client failed (%s): %v", peer, err)
			}
			return
		}
		if _, err := f.packetConn.WriteTo(reply, peer); err != nil {
			log.Printf("[-] UDP write to peer failed (%s): %v", peer, err)
			return
		}
	}
}

// closeChannel forgets the channel of peer, unless a newer channel replaced it.
func (f *udpForward) closeChannel(peer net.Addr, channel ssh.Channel) {
	f.mu.Lock()
	if f.peerChannels[peer.String()] == channel {
		delete(f.peerChannels, peer.String())
	}
	f.mu.Unlock()
	channel.Close()
}

func (f *udpForward) closeAllChannels() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for peer, channel := range f.peerChannels {
		channel.Close()
		delete(f.peerChannels, peer)
	}
}
