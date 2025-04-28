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

const (
	// Handshake error codes
	ErrSuccess      uint32 = 0
	ErrIPNotAllowed uint32 = 2

	// Port‐assignment error codes (high bit set)
	ErrPortUnavailable uint32 = 1
	ErrPortOutOfRange  uint32 = 3
	ErrInternal        uint32 = 4
	ErrMask            uint32 = 0x80000000
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
		flag.IntVar(&sp.PortRangeStart, SpKeyPortRangeStart, SpDefaultPortRangeStart, "start port range")
		flag.IntVar(&sp.PortRangeEnd, SpKeyPortRangeEnd, SpDefaultPortRangeEnd, "end port range")
		flag.StringVar(&sp.Username, SpKeyUsername, SpDefaultUsername, "SSH username")
		flag.StringVar(&sp.Password, SpKeyPassword, SpDefaultPassword, "SSH password")
		flag.StringVar(&sp.PrivateRsaPath, SpKeyPrivateRsa, SpDefaultPrivateRsa, "path to RSA key")
		flag.StringVar(&sp.PrivateEcdsaPath, SpKeyPrivateEcdsa, SpDefaultPrivateEcdsa, "path to ECDSA key")
		flag.StringVar(&sp.PrivateEd25519Path, SpKeyPrivateEd25519, SpDefaultPrivateEd25519, "path to Ed25519 key")
		flag.StringVar(&sp.AuthorizedKeysPath, SpKeyAuthorizedKeys, SpDefaultAuthorizedKeys, "path to authorized_keys")
		flag.Var(&sp.AllowedIPs, SpKeyAllowedIPS, "comma-separated list of allowed IPs")
		flag.Parse()
	} else {
		sp = *spOverride
	}

	if err := sp.Validate(); err != nil {
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
	defer listener.Close()

	log.Printf("[+] Server listening on %s", s.formattedAddress)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				log.Printf("Temporary accept error, retrying: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("critical accept error: %v", err)
		}
		log.Printf("[+] New TCP connection from %s", conn.RemoteAddr())
		go s.handleConnection(conn)
	}
}

func (s *ForwardServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	clientAddr := conn.RemoteAddr().String()
	log.Printf("[*] Starting SSH handshake with %s", clientAddr)

	sshConn, channels, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		log.Printf("[-] SSH handshake failed with %s: %v", clientAddr, err)
		return
	}
	defer func() {
		log.Printf("[*] Closing SSH connection from %s", sshConn.RemoteAddr())
		sshConn.Close()
	}()
	go ssh.DiscardRequests(reqs)

	remoteIP, _, _ := net.SplitHostPort(clientAddr)
	ipAllowed := s.isIPAllowed(remoteIP)

	for newChannel := range channels {
		if newChannel.ChannelType() != "direct-tcpip" {
			newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, reqs2, err := newChannel.Accept()
		if err != nil {
			log.Printf("[-] Failed to accept channel: %v", err)
			continue
		}
		go ssh.DiscardRequests(reqs2)

		// 1) Handshake
		var hb [4]byte
		if !ipAllowed {
			log.Printf("[-] Connection refused from disallowed IP: %s", remoteIP)
			binary.BigEndian.PutUint32(hb[:], ErrIPNotAllowed)
			channel.Write(hb[:])
			channel.Close()
			continue
		}
		binary.BigEndian.PutUint32(hb[:], ErrSuccess)
		channel.Write(hb[:])

		// 2) Read port request
		var reqBuf [4]byte
		if _, err := io.ReadFull(channel, reqBuf[:]); err != nil {
			channel.Close()
			continue
		}
		requested := int(binary.BigEndian.Uint32(reqBuf[:]))
		log.Printf("[*] Client %s requested port: %d", sshConn.RemoteAddr(), requested)

		// 3) Range check
		if requested != 0 && (requested < s.portRangeStart || requested > s.portRangeEnd) {
			log.Printf("[-] Requested port %d is outside allowed range", requested)
			var ob [4]byte
			binary.BigEndian.PutUint32(ob[:], ErrMask|ErrPortOutOfRange)
			channel.Write(ob[:])
			channel.Close()
			continue
		}

		// 4) Assign port
		port := s.assignPort(requested)
		if port == 0 {
			log.Printf("[-] No available ports in range %d-%d", s.portRangeStart, s.portRangeEnd)
			var ob [4]byte
			binary.BigEndian.PutUint32(ob[:], ErrMask|ErrPortUnavailable)
			channel.Write(ob[:])
			channel.Close()
			continue
		}

		// 5) Try binding
		bindAddr := fmt.Sprintf("0.0.0.0:%d", port)
		ln, err := net.Listen("tcp", bindAddr)
		if err != nil {
			log.Printf("[-] Bind error on port %d: %v", port, err)
			var ob [4]byte
			binary.BigEndian.PutUint32(ob[:], ErrMask|ErrInternal)
			channel.Write(ob[:])
			channel.Close()
			continue
		}
		log.Printf("[+] Assigned port %d", port)

		// 6) Inform client
		var pb [4]byte
		binary.BigEndian.PutUint32(pb[:], uint32(port))
		channel.Write(pb[:])
		channel.Close()

		// 7) Forward loop
		done := make(chan struct{})
		s.forwardsLock.Lock()
		s.forwards[port] = &PortForward{Port: port, Conn: sshConn, Closed: done}
		s.forwardsLock.Unlock()

		go s.serveForward(port, sshConn, ln, done)
		go func(p int, ch <-chan struct{}) {
			<-ch
			log.Printf("[*] Cleanup for port %d", p)
			s.forwardsLock.Lock()
			delete(s.forwards, p)
			s.forwardsLock.Unlock()
		}(port, done)
	}
}

func (s *ForwardServer) serveForward(port int, sshConn *ssh.ServerConn, listener net.Listener, done chan struct{}) {
	defer listener.Close()
	defer close(done)

	var active sync.WaitGroup
	closed := make(chan struct{})

	go func() {
		sshConn.Wait()
		close(closed)
	}()

	go func() {
		<-closed
		listener.Close()
		active.Wait()
	}()

	connID := 0
	for {
		c, err := listener.Accept()
		if err != nil {
			select {
			case <-closed:
				return
			default:
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return
			}
		}

		connID++
		active.Add(1)
		go func(c net.Conn, id int) {
			defer active.Done()
			defer c.Close()

			ch, reqs, err := sshConn.OpenChannel("direct-tcpip", nil)
			if err != nil {
				return
			}
			defer ch.Close()
			go ssh.DiscardRequests(reqs)

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				io.Copy(ch, c)
				if tcp, ok := c.(interface{ CloseWrite() error }); ok {
					tcp.CloseWrite()
				}
			}()
			go func() {
				defer wg.Done()
				io.Copy(c, ch)
				if tcp, ok := ch.(interface{ CloseWrite() error }); ok {
					tcp.CloseWrite()
				}
			}()
			wg.Wait()
		}(c, connID)
	}
}

func (s *ForwardServer) isIPAllowed(ip string) bool {
	if len(s.allowedIPs) == 0 {
		return true
	}
	for _, a := range s.allowedIPs {
		if ip == a {
			return true
		}
	}
	return false
}

func (s *ForwardServer) assignPort(requested int) int {
	s.forwardsLock.Lock()
	defer s.forwardsLock.Unlock()

	if requested != 0 {
		if requested >= s.portRangeStart && requested <= s.portRangeEnd {
			if _, used := s.forwards[requested]; !used {
				return requested
			}
		}
		return 0
	}
	for p := s.portRangeStart; p <= s.portRangeEnd; p++ {
		if _, used := s.forwards[p]; !used {
			return p
		}
	}
	return 0
}
