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
		flag.StringVar(&sp.PrivateKeyPath, SpKeyPrivateKey, SpDefaultPrivateKey, "Path to private key file")
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
		_ = listener.Close()
	}(listener)
	log.Printf("Server bind to address %s", s.formattedAddress)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *ForwardServer) handleConnection(conn net.Conn) {
	sshConn, channels, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		log.Printf("SSH handshake failed: %v", err)
		return
	}

	defer func(sshConn *ssh.ServerConn) {
		_ = sshConn.Close()
	}(sshConn)

	remoteIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		log.Printf("Failed to parse remote address: %v", err)
		_ = conn.Close()
		return
	}

	if !s.isIPAllowed(remoteIP) {
		log.Printf("Connection refused from disallowed IP: %s", remoteIP)
		_ = conn.Close()
		return
	}

	log.Printf("New SSH connection from %s", sshConn.RemoteAddr())
	go ssh.DiscardRequests(reqs)

	for newChannel := range channels {
		if newChannel.ChannelType() != "direct-tcpip" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}

		channel, _, err := newChannel.Accept()
		if err != nil {
			log.Printf("Failed to accept channel: %v", err)
			continue
		}

		portBuf := make([]byte, 4)
		if _, err := io.ReadFull(channel, portBuf); err != nil {
			log.Printf("Failed to read port: %v", err)
			_ = channel.Close()
			continue
		}

		requested := int(binary.BigEndian.Uint32(portBuf))
		port := s.assignPort(requested)
		if port == 0 {
			log.Printf("No available ports")
			_ = channel.Close()
			continue
		}

		respBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(respBuf, uint32(port))
		_, _ = channel.Write(respBuf)
		_ = channel.Close()

		done := make(chan struct{})
		s.forwardsLock.Lock()
		s.forwards[port] = &PortForward{Port: port, Conn: sshConn, Closed: done}
		s.forwardsLock.Unlock()

		go s.listenAndForward(port, sshConn, done)

		go func(p int, c <-chan struct{}) {
			<-c
			s.forwardsLock.Lock()
			delete(s.forwards, p)
			s.forwardsLock.Unlock()
			log.Printf("[-] Port %d closed due to client disconnect", p)
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
		return 0
	}

	if requested != 0 {
		if requested >= s.portRangeStart && requested <= s.portRangeEnd {
			if _, exists := s.forwards[requested]; !exists {
				return requested
			}
		}
	} else {
		for i := s.portRangeStart; i <= s.portRangeEnd; i++ {
			if _, exists := s.forwards[i]; !exists {
				return i
			}
		}
	}
	return 0
}

func (s *ForwardServer) listenAndForward(port int, sshConn *ssh.ServerConn, done chan struct{}) {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Printf("Error listening on port %d: %v", port, err)
		close(done)
		return
	}
	log.Printf("[+] Port %d exposed to external world", port)

	go func() {
		_ = sshConn.Wait()
		_ = listener.Close()
		close(done)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		go func(c net.Conn) {
			channel, reqs, err := sshConn.OpenChannel("direct-tcpip", nil)
			if err != nil {
				log.Printf("Failed to open channel to client: %v", err)
				_ = c.Close()
				return
			}
			go ssh.DiscardRequests(reqs)
			go func() {
				_, _ = io.Copy(channel, c)
			}()
			_, _ = io.Copy(c, channel)
			_ = channel.Close()
			_ = c.Close()
		}(conn)
	}
}
