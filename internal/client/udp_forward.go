package client

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

// forwardUDP relays the datagrams of one remote peer between the server and the
// local service, until the server closes the channel.
func forwardUDP(channel ssh.Channel, localAddress string, forwardID int) {
	localAddr, err := net.ResolveUDPAddr("udp", localAddress)
	if err != nil {
		log.Printf("[-] Resolve local UDP %s: %v", localAddress, err)
		return
	}
	localConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		log.Printf("[-] Connect to local UDP %s: %v", localAddress, err)
		return
	}
	defer localConn.Close()

	var bothDirections sync.WaitGroup
	bothDirections.Go(func() {
		// Closing the local socket also stops the other direction.
		defer localConn.Close()
		sendDatagramsToLocal(channel, localConn, forwardID)
	})
	bothDirections.Go(func() {
		sendDatagramsToServer(localConn, channel, forwardID)
	})
	bothDirections.Wait()
	log.Printf("[+] UDP forward #%d closed", forwardID)
}

func sendDatagramsToLocal(channel ssh.Channel, localConn *net.UDPConn, forwardID int) {
	for {
		datagram, err := protocol.ReadDatagram(channel)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("[-] UDP read from server for forward #%d: %v", forwardID, err)
			}
			return
		}
		if _, err := localConn.Write(datagram); err != nil {
			log.Printf("[-] UDP write to local for forward #%d: %v", forwardID, err)
			return
		}
	}
}

func sendDatagramsToServer(localConn *net.UDPConn, channel ssh.Channel, forwardID int) {
	datagram := make([]byte, protocol.MaxDatagramSize)
	for {
		n, err := localConn.Read(datagram)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("[-] UDP read from local for forward #%d: %v", forwardID, err)
			}
			return
		}
		if err := protocol.WriteDatagram(channel, datagram[:n]); err != nil {
			log.Printf("[-] UDP write to server for forward #%d: %v", forwardID, err)
			return
		}
	}
}
