package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type PortForward struct {
	Port   int
	Conn   *ssh.ServerConn
	Closed chan struct{}
}

type ForwardServer struct {
	address          string
	port             int
	formattedAddress string
	config           *ssh.ServerConfig
	portRangeStart   int
	portRangeEnd     int
	allowedIPs       []string
	forwards         map[int]*PortForward
	forwardsLock     sync.Mutex
}

func Server(spOverride *ServerParameters) *ForwardServer {
	var sp ServerParameters

	if spOverride == nil {
		flag.StringVar(&sp.BindAddress, SpKeyBindAddress, SpDefaultBindAddress, "bind address")
		flag.IntVar(&sp.BindPort, SpKeyBindPort, SpDefaultBindPort, "bind port")
		flag.IntVar(&sp.PortRangeStart, SpKeyPortRangeStart, SpDefaultPortRangeStart, "Start port range of ports")
		flag.IntVar(&sp.PortRangeEnd, SpKeyPortRangeEnd, SpDefaultPortRangeEnd, "End port range of ports")
		flag.StringVar(&sp.Username, SpKeyUsername, SpDefaultUsername, "Username to use for ssh connection")
		flag.StringVar(&sp.Password, SpKeyPassword, SpDefaultPassword, "Password to use for ssh connection")
		flag.StringVar(&sp.PrivateRsaPath, SpKeyPrivateRsa, SpDefaultPrivateRsa, "Path to private RSA key file")
		flag.StringVar(&sp.PrivateEcdsaPath, SpKeyPrivateEcdsa, SpDefaultPrivateRsa, "Path to private ECDSA key file")
		flag.StringVar(&sp.PrivateRsaPath, SpKeyPrivateEd25519, SpDefaultPrivateRsa, "Path to private ED25519 key file")
		flag.StringVar(&sp.AuthorizedKeysPath, SpKeyAuthorizedKeys, SpDefaultAuthorizedKeys, "Path to authorized keys file")
		flag.Var(&sp.AllowedIPs, SpKeyAllowedIPS, "Comma-separated list of allowed IPs")
		flag.Parse()
	} else {
		sp = *spOverride
	}

	err := sp.Validate()
	if err != nil {
		log.Fatalf("Invalid server parameters: %v", err)
	}

	config, err := GetServerConfig(sp)
	if err != nil {
		log.Fatalf("Failed to get server config: %v", err)
	}

	return &ForwardServer{
		address:          sp.BindAddress,
		port:             sp.BindPort,
		formattedAddress: fmt.Sprintf("%s:%d", sp.BindAddress, sp.BindPort),
		config:           config,
		portRangeStart:   sp.PortRangeStart,
		portRangeEnd:     sp.PortRangeEnd,
		allowedIPs:       sp.AllowedIPs,
		forwards:         make(map[int]*PortForward),
	}
}

func (s *ForwardServer) Start() error {
	listener, err := net.Listen("tcp", s.formattedAddress)
	if err != nil {
		return fmt.Errorf("cannot bind address %s: %v", s.formattedAddress, err)
	}
	defer func(listener net.Listener) {
		if err := listener.Close(); err != nil {
			log.Printf("Error closing main listener: %v", err)
		}
	}(listener)
	log.Printf("[+] Server listening on %s", s.formattedAddress)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				log.Printf("Temporary accept error, retrying: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			log.Printf("Accept error (terminating): %v", err)
			return fmt.Errorf("critical accept error: %v", err)
		}
		log.Printf("[+] New TCP connection from %s", conn.RemoteAddr())
		go s.handleConnection(conn)
	}
}

func (s *ForwardServer) handleConnection(conn net.Conn) {
	clientAddr := conn.RemoteAddr().String()
	log.Printf("[*] Starting SSH handshake with %s", clientAddr)

	sshConn, channels, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		log.Printf("[-] SSH handshake failed with %s: %v", clientAddr, err)
		if err := conn.Close(); err != nil {
			log.Printf("[-] Error closing connection after failed handshake: %v", err)
		}
		return
	}

	defer func(sshConn *ssh.ServerConn) {
		log.Printf("[*] Closing SSH connection from %s", sshConn.RemoteAddr())
		if err := sshConn.Close(); err != nil {
			log.Printf("[-] Error closing SSH connection: %v", err)
		}
	}(sshConn)

	remoteIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		log.Printf("[-] Failed to parse remote address %s: %v", conn.RemoteAddr(), err)
		if err := conn.Close(); err != nil {
			log.Printf("[-] Error closing connection after address parse failure: %v", err)
		}
		return
	}

	if !s.isIPAllowed(remoteIP) {
		log.Printf("[-] Connection refused from disallowed IP: %s", remoteIP)
		if err := conn.Close(); err != nil {
			log.Printf("[-] Error closing connection for disallowed IP: %v", err)
		}
		return
	}

	log.Printf("[+] New SSH connection established from %s (User: %s)", sshConn.RemoteAddr(), sshConn.User())
	go ssh.DiscardRequests(reqs)

	for newChannel := range channels {
		if newChannel.ChannelType() != "direct-tcpip" {
			log.Printf("[-] Rejecting channel of type %s from %s", newChannel.ChannelType(), sshConn.RemoteAddr())
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}

		channel, reqs, err := newChannel.Accept()
		if err != nil {
			log.Printf("[-] Failed to accept channel from %s: %v", sshConn.RemoteAddr(), err)
			continue
		}
		log.Printf("[+] Channel accepted from %s", sshConn.RemoteAddr())
		go ssh.DiscardRequests(reqs)

		portBuf := make([]byte, 4)
		if _, err := io.ReadFull(channel, portBuf); err != nil {
			log.Printf("[-] Failed to read port request from %s: %v", sshConn.RemoteAddr(), err)
			if err := channel.Close(); err != nil {
				log.Printf("[-] Error closing channel after port read failure: %v", err)
			}
			continue
		}

		requested := int(binary.BigEndian.Uint32(portBuf))
		log.Printf("[*] Client %s requested port: %d", sshConn.RemoteAddr(), requested)

		port := s.assignPort(requested)
		if port == 0 {
			log.Printf("[-] No available ports for request from %s", sshConn.RemoteAddr())
			if err := channel.Close(); err != nil {
				log.Printf("[-] Error closing channel after port assignment failure: %v", err)
			}
			continue
		}
		log.Printf("[+] Assigned port %d to client %s", port, sshConn.RemoteAddr())

		respBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(respBuf, uint32(port))
		if _, err := channel.Write(respBuf); err != nil {
			log.Printf("[-] Failed to send port assignment %d to %s: %v", port, sshConn.RemoteAddr(), err)
		}
		if err := channel.Close(); err != nil {
			log.Printf("[-] Error closing initial channel: %v", err)
		}

		done := make(chan struct{})
		s.forwardsLock.Lock()
		s.forwards[port] = &PortForward{Port: port, Conn: sshConn, Closed: done}
		s.forwardsLock.Unlock()

		go s.listenAndForward(port, sshConn, done)

		go func(p int, c <-chan struct{}) {
			<-c
			log.Printf("[*] Cleanup triggered for port %d", p)
			s.forwardsLock.Lock()
			delete(s.forwards, p)
			s.forwardsLock.Unlock()
			log.Printf("[-] Port %d removed from forwards map due to client disconnect", p)
		}(port, done)
	}
}

func (s *ForwardServer) isIPAllowed(ip string) bool {
	if len(s.allowedIPs) == 0 {
		return true
	}

	for _, allowedIP := range s.allowedIPs {
		if ip == allowedIP {
			return true
		}
	}

	return false
}

func (s *ForwardServer) assignPort(requested int) int {
	s.forwardsLock.Lock()
	defer s.forwardsLock.Unlock()

	if requested == s.port {
		log.Printf("[-] Requested port %d conflicts with server control port", requested)
		return 0
	}

	if requested != 0 {
		if requested >= s.portRangeStart && requested <= s.portRangeEnd {
			if _, exists := s.forwards[requested]; !exists {
				log.Printf("[+] Assigning specifically requested port %d", requested)
				return requested
			}
			log.Printf("[-] Requested port %d is already in use", requested)
		} else {
			log.Printf("[-] Requested port %d is outside allowed range (%d-%d)",
				requested, s.portRangeStart, s.portRangeEnd)
		}
	} else {
		log.Printf("[*] No specific port requested, searching for available port")
		for i := s.portRangeStart; i <= s.portRangeEnd; i++ {
			if _, exists := s.forwards[i]; !exists {
				log.Printf("[+] Found available port %d", i)
				return i
			}
		}
		log.Printf("[-] No available ports in range %d-%d", s.portRangeStart, s.portRangeEnd)
	}
	return 0
}

func (s *ForwardServer) listenAndForward(port int, sshConn *ssh.ServerConn, done chan struct{}) {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("[*] Setting up listener for port forwarding on %s", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[-] Error listening on port %d: %v", port, err)
		close(done)
		return
	}
	log.Printf("[+] Port %d successfully exposed for client %s", port, sshConn.RemoteAddr())

	// Use a connection counter to track active connections
	var activeConnections sync.WaitGroup
	// Channel to signal listener to stop
	listenerClosed := make(chan struct{})

	go func() {
		// Wait for SSH connection to close
		log.Printf("[*] Monitoring SSH connection status for port %d", port)
		_ = sshConn.Wait()
		log.Printf("[-] SSH connection closed for port %d", port)

		// Close listener to stop accepting new connections
		log.Printf("[*] Closing listener for port %d", port)
		if err := listener.Close(); err != nil {
			log.Printf("[-] Error closing listener for port %d: %v", port, err)
		}
		close(listenerClosed)

		// Wait for all active connections to finish
		log.Printf("[*] Waiting for %d active connections to finish on port %d",
			// This won't be accurate but provides some info
			// We can't safely access the counter
			0, port)
		activeConnections.Wait()
		log.Printf("[+] All connections closed for port %d", port)

		// Signal that we're done with this port
		log.Printf("[*] Cleaning up port %d", port)
		close(done)
	}()

	// Connection counter for logging
	connectionCount := 0

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if listener was closed deliberately
			select {
			case <-listenerClosed:
				log.Printf("[*] Listener for port %d closed normally", port)
			default:
				if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
					log.Printf("[*] Temporary listener error on port %d: %v", port, err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				log.Printf("[-] Listener error on port %d: %v", port, err)
			}
			return
		}

		connectionCount++
		connID := connectionCount
		log.Printf("[+] New incoming connection #%d on port %d from %s", connID, port, conn.RemoteAddr())

		// Increment active connection counter
		activeConnections.Add(1)

		go func(c net.Conn, id int) {
			defer func() {
				activeConnections.Done()
				log.Printf("[-] Connection #%d on port %d closed", id, port)
			}()
			defer c.Close()

			// Open a new channel for each connection
			log.Printf("[*] Opening SSH channel for connection #%d on port %d", id, port)
			channel, reqs, err := sshConn.OpenChannel("direct-tcpip", nil)
			if err != nil {
				log.Printf("[-] Failed to open channel for connection #%d on port %d: %v", id, port, err)
				return
			}
			log.Printf("[+] SSH channel opened for connection #%d on port %d", id, port)
			defer channel.Close()

			go ssh.DiscardRequests(reqs)

			// Use WaitGroup to ensure both copy operations complete
			var wg sync.WaitGroup
			wg.Add(2)

			// Copy data in both directions concurrently
			go func() {
				defer wg.Done()
				bytes, err := io.Copy(channel, c)
				if err != nil {
					log.Printf("[-] Error copying data from client to SSH for connection #%d: %v", id, err)
				}
				log.Printf("[*] Copied %d bytes from client to SSH for connection #%d", bytes, id)

				// Signal EOF to the other goroutine by closing the write end
				if conn, ok := c.(interface{ CloseWrite() error }); ok {
					if err := conn.CloseWrite(); err != nil {
						log.Printf("[-] Error closing write end of TCP connection #%d: %v", id, err)
					}
				}
			}()

			go func() {
				defer wg.Done()
				bytes, err := io.Copy(c, channel)
				if err != nil {
					log.Printf("[-] Error copying data from SSH to client for connection #%d: %v", id, err)
				}
				log.Printf("[*] Copied %d bytes from SSH to client for connection #%d", bytes, id)

				// Signal EOF to the other goroutine by closing the channel write
				if conn, ok := channel.(interface{ CloseWrite() error }); ok {
					if err := conn.CloseWrite(); err != nil {
						log.Printf("[-] Error closing write end of SSH channel for connection #%d: %v", id, err)
					}
				}
			}()

			// Wait for both copies to complete
			wg.Wait()
			log.Printf("[+] Data transfer completed for connection #%d on port %d", id, port)
		}(conn, connID)
	}
}
