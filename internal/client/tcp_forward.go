package client

import (
	"log"
	"net"

	"github.com/poweredbypump/pbp-tunnel/internal/relay"
	"golang.org/x/crypto/ssh"
)

// forwardTCP relays one forwarded connection to the local service.
func forwardTCP(channel ssh.Channel, localAddress string, forwardID int) {
	localConn, err := net.Dial("tcp", localAddress)
	if err != nil {
		log.Printf("[-] Connect to local %s: %v", localAddress, err)
		return
	}
	defer localConn.Close()

	toLocal, toServer := relay.CopyBothWays(channel, localConn.(*net.TCPConn))
	log.Printf("[+] Forward #%d closed: %d bytes to local, %d bytes to server", forwardID, toLocal, toServer)
}
