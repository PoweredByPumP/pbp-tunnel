package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

func Client(cpOverride *ClientParameters) {
	var cp ClientParameters

	if cpOverride == nil {
		flag.StringVar(&cp.Endpoint, CpKeyEndpoint, CpDefaultEndpoint, "Endpoint to connect to")
		flag.IntVar(&cp.EndpointPort, CpKeyEndpointPort, CpDefaultEndpointPort, "Port of the Endpoint (default: 22)")
		flag.StringVar(&cp.Username, CpKeyUsername, CpDefaultUsername, "SSH Username")
		flag.StringVar(&cp.Password, CpKeyPassword, CpDefaultPassword, "SSH Password")
		flag.StringVar(&cp.PrivateKeyPath, CpKeyPrivateKeyPath, CpDefaultPrivateKeyPath, "Private key path (optional) (default: null)")
		flag.StringVar(&cp.HostKeyPath, CpKeyHostKeyPath, CpDefaultHostKeyPath, "Known host key file (optional) (default: null)")
		flag.StringVar(&cp.LocalHost, CpKeyLocalHost, CpDefaultLocalHost, "Local address (default: localhost)")
		flag.IntVar(&cp.LocalPort, CpKeyLocalPort, CpDefaultLocalPort, "Local port to forward (default: 80)")
		flag.StringVar(&cp.RemoteHost, CpKeyRemoteHost, CpDefaultRemoteHost, "Remote address (unused) (default: localhost)")
		flag.IntVar(&cp.RemotePort, CpKeyRemotePort, CpDefaultRemotePort, "Remote port to request (0 = random) (default: 0)")
		flag.IntVar(&cp.HostKeyLevel, CpKeyHostKeyLevel, CpDefaultHostKeyLevel, "Host key level (0 = no check, 1 = warn, 2 = strict) (default: 2)")
		flag.Parse()
	} else {
		cp = *cpOverride
	}

	err := cp.Validate()
	if err != nil {
		log.Fatalf("Invalid client parameters: %v", err)
	}

	retryCount := 0
	for {
		config, err := GetClientConfig(cp)
		address := fmt.Sprintf("%s:%d", cp.Endpoint, cp.EndpointPort)

		connection, err := ssh.Dial("tcp", address, config)
		if err != nil {
			log.Printf("Failed to connect to server (try %d): %v", retryCount, err)

			if retryCount >= 3 {
				log.Fatalf("Failed to connect after 3 attempts: %v", err)
			}

			retryCount++
			time.Sleep(5 * time.Second)
			continue
		}

		log.Println("Connected to server")
		handleClientSession(connection, cp)

		log.Println("Disconnected from server, retrying...")
	}
}

func handleClientSession(conn *ssh.Client, cp ClientParameters) {
	address := cp.GetFormattedAddress()

	defer conn.Close()

	channel, reqs, err := conn.OpenChannel("direct-tcpip", nil)
	if err != nil {
		log.Printf("Failed to open SSH channel: %v", err)
		return
	}

	go ssh.DiscardRequests(reqs)

	reqBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(reqBuf, uint32(cp.RemotePort))
	if _, err := channel.Write(reqBuf); err != nil {
		log.Printf("Failed to send port request: %v", err)
		channel.Close()
		return
	}

	respBuf := make([]byte, 4)
	if _, err := io.ReadFull(channel, respBuf); err != nil {
		log.Printf("Failed to read assigned port: %v", err)
		channel.Close()
		return
	}
	assignedPort := int(binary.BigEndian.Uint32(respBuf))
	log.Printf("Remote port assigned by server for %s → remote:%d", address, assignedPort)
	channel.Close()

	for newChannel := range conn.HandleChannelOpen("direct-tcpip") {
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Failed to accept forwarded channel: %v", err)
			continue
		}

		go ssh.DiscardRequests(requests)

		go func(channel ssh.Channel) {
			defer channel.Close()
			localConn, err := net.Dial("tcp", address)
			if err != nil {
				log.Printf("Connection to local service failed for %s: %v", address, err)
				return
			}

			defer localConn.Close()

			log.Printf("Accepted connection from server, forwarding to local %s", address)

			go io.Copy(channel, localConn)
			io.Copy(localConn, channel)
		}(channel)
	}
}
