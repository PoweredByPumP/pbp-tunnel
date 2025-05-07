package server

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
)

const (
	ErrSuccess         uint32 = 0
	ErrPortUnavailable uint32 = 1
	ErrIPNotAllowed    uint32 = 2
	ErrPortOutOfRange  uint32 = 3
	ErrInternal        uint32 = 4
	ErrMask            uint32 = 0x80000000
)

type ForwardServer struct {
	sshConfig      *ssh.ServerConfig
	bindAddress    string
	bindPort       int
	portRangeStart int
	portRangeEnd   int
	allowedIPs     []string
	forwards       map[int]struct{}
	lock           sync.Mutex
}

// ForwardServer maintains state for port forwarding
// sshConfig: SSH server configuration
// bindAddress/Port: where to expose forwarded ports
// portRangeStart/End: allowed range
// allowedIPs: client whitelist
// forwards: map of in-use ports
// lock: protects forwards

// Run starts the SSH reverse-tunnel server
func Run(spOverride *config.ServerParameters) error {
	var sp config.ServerParameters
	if spOverride == nil {
		flag.StringVar(&sp.BindAddress, config.SpKeyBindAddress, config.SpDefaultBindAddress, "bind address")
		flag.IntVar(&sp.BindPort, config.SpKeyBindPort, config.SpDefaultBindPort, "bind port")
		flag.IntVar(&sp.PortRangeStart, config.SpKeyPortRangeStart, config.SpDefaultPortRangeStart, "start port range")
		flag.IntVar(&sp.PortRangeEnd, config.SpKeyPortRangeEnd, config.SpDefaultPortRangeEnd, "end port range")
		flag.StringVar(&sp.Username, config.SpKeyUsername, config.SpDefaultUsername, "SSH username")
		flag.StringVar(&sp.Password, config.SpKeyPassword, config.SpDefaultPassword, "SSH password")
		flag.StringVar(&sp.PrivateRsaPath, config.SpKeyPrivateRsaPath, config.SpDefaultPrivateRsa, "path to RSA key")
		flag.StringVar(&sp.PrivateEcdsaPath, config.SpKeyPrivateEcdsaPath, config.SpDefaultPrivateEcdsa, "path to ECDSA key")
		flag.StringVar(&sp.PrivateEd25519Path, config.SpKeyPrivateEd25519Path, config.SpDefaultPrivateEd25519, "path to Ed25519 key")
		flag.StringVar(&sp.AuthorizedKeysPath, config.SpKeyAuthorizedKeysPath, config.SpDefaultAuthorizedKeys, "path to authorized_keys")
		flag.Var(&sp.AllowedIPs, config.SpKeyAllowedIPS, "comma-separated list of allowed IPs")
		flag.Parse()
	} else {
		sp = *spOverride
	}

	// 1) Validate configuration
	if err := sp.Validate(); err != nil {
		return fmt.Errorf("invalid server parameters: %w", err)
	}
	// 2) Build SSH config
	sshCfg, addr, err := config.GetServerConfig(&sp)
	if err != nil {
		return fmt.Errorf("failed to build server config: %w", err)
	}
	// 3) Listen
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	defer ln.Close()
	log.Printf("[+] SSH server listening on %s", addr)

	srv := &ForwardServer{
		sshConfig:      sshCfg,
		bindAddress:    sp.BindAddress,
		bindPort:       sp.BindPort,
		portRangeStart: sp.PortRangeStart,
		portRangeEnd:   sp.PortRangeEnd,
		allowedIPs:     sp.AllowedIPs,
		forwards:       make(map[int]struct{}),
	}
	// 4) Accept loop
	for {
		nc, err := ln.Accept()
		if err != nil {
			log.Printf("[-] Accept error: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go srv.handleSSHConnection(nc)
	}
}

// handleSSHConnection manages SSH handshake and channels
func (s *ForwardServer) handleSSHConnection(nc net.Conn) {
	defer nc.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(nc, s.sshConfig)
	if err != nil {
		log.Printf("[-] SSH handshake failed: %v", err)
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)

	rAddr := sshConn.RemoteAddr().String()
	host, _, _ := net.SplitHostPort(rAddr)
	log.Printf("[+] New SSH connection from %s", rAddr)
	// initial IP check
	if len(s.allowedIPs) > 0 && !isAllowed(host, s.allowedIPs) {
		log.Printf("[-] SSH client %s not allowed", host)
		return
	}
	// channel loop
	for newCh := range chans {
		if newCh.ChannelType() != "direct-tcpip" {
			newCh.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		ch, reqs2, err := newCh.Accept()
		if err != nil {
			log.Printf("[-] Accept channel failed: %v", err)
			continue
		}
		go ssh.DiscardRequests(reqs2)
		s.handleChannel(sshConn, ch)
	}
}

// handleChannel manages port-forward handshake, assignment, and data forwarding
func (s *ForwardServer) handleChannel(sshConn *ssh.ServerConn, channel ssh.Channel) {
	defer channel.Close()
	var hb [4]byte

	// 1) Handshake and whitelist
	host, _, _ := net.SplitHostPort(sshConn.RemoteAddr().String())
	clientWL, err := processHandshake(channel, host, s.allowedIPs)
	if err != nil {
		log.Printf("[-] Handshake error: %v", err)
		return
	}
	log.Printf("[+] Whitelist accepted: %v", clientWL)

	// 2) Read requested port
	if _, err := io.ReadFull(channel, hb[:]); err != nil {
		log.Printf("[-] Read requested port failed: %v", err)
		return
	}
	reqPort := int(binary.BigEndian.Uint32(hb[:]))
	log.Printf("[*] Client requested port %d", reqPort)

	// 3) Assign port
	port, mask := assignPort(reqPort, s.portRangeStart, s.portRangeEnd, s.forwards, &s.lock)
	if mask != 0 {
		binary.BigEndian.PutUint32(hb[:], mask)
		channel.Write(hb[:])
		log.Printf("[-] Port assignment failed: mask %08x", mask)
		return
	}
	log.Printf("[+] Assigned port %d", port)

	// 4) Bind listener for forwarded connections
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.bindAddress, port))
	if err != nil {
		binary.BigEndian.PutUint32(hb[:], ErrMask|ErrInternal)
		channel.Write(hb[:])
		log.Printf("[-] Bind port %d failed: %v", port, err)
		return
	}
	defer ln.Close()

	// 5) Notify client of assigned port
	binary.BigEndian.PutUint32(hb[:], uint32(port))
	channel.Write(hb[:])
	log.Printf("[+] Notified client of port %d", port)

	// 6) Serve until client disconnects
	done := make(chan struct{})
	go func() {
		_ = sshConn.Wait()
		ln.Close()
		close(done)
	}()

	var wg sync.WaitGroup
	var doWaitForConnection = true
	for id := 0; ; id++ {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-done:
				// client disconnected
				goto RELEASE

			default:
				log.Printf("[-] Forward accept error: %v", err)
				if strings.Contains(err.Error(), "use of closed network connection") {
					// listener closed
					doWaitForConnection = false
				}

				goto RELEASE
			}
		}
		// whitelist forwarded peer
		peer, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		accepted := len(clientWL) == 0
		for _, entry := range clientWL {
			if strings.Contains(entry, "/") {
				if _, cidr, err := net.ParseCIDR(entry); err == nil && cidr.Contains(net.ParseIP(peer)) {
					accepted = true
					break
				}
			} else if entry == peer {
				accepted = true
				break
			}
		}
		if !accepted {
			log.Printf("[-] Connection from %s rejected by whitelist", peer)
			conn.Close()
			continue
		}

		wg.Add(1)
		go func(c net.Conn, idx int) {
			defer wg.Done()
			defer c.Close()

			ch2, reqs3, err := sshConn.OpenChannel("direct-tcpip", nil)
			if err != nil {
				log.Printf("[-] Open back-channel failed: %v", err)
				return
			}
			go ssh.DiscardRequests(reqs3)

			var cc sync.WaitGroup
			cc.Add(2)
			// service -> client
			go func() {
				defer cc.Done()
				n, _ := io.Copy(ch2, c)
				log.Printf("[*] Copied %d bytes to client for forward %d", n, idx)
				ch2.CloseWrite()
			}()
			// client -> service
			go func() {
				defer cc.Done()
				n, _ := io.Copy(c, ch2)
				log.Printf("[*] Copied %d bytes to service for forward %d", n, idx)
			}()
			cc.Wait()
			log.Printf("[+] Forward %d closed", idx)
		}(conn, port)
	}

RELEASE:
	if doWaitForConnection {
		wg.Wait()
	}

	log.Printf("[*] Waiting for lock to release port %d", port)
	s.lock.Lock()

	log.Printf("[*] Client disconnected, freed port %d", port)
	delete(s.forwards, port)

	s.lock.Unlock()
}

// assignPort reserves or picks a port within range using the forwards map under lock.
// It returns the assigned port or 0 and an error mask if no port could be assigned.
func assignPort(reqPort, start, end int, forwards map[int]struct{}, lock *sync.Mutex) (int, uint32) {
	// invalid range
	if start > end {
		return 0, ErrMask | ErrPortUnavailable
	}
	// specific port requested
	if reqPort != 0 {
		if reqPort < start || reqPort > end {
			return 0, ErrMask | ErrPortOutOfRange
		}
		lock.Lock()
		defer lock.Unlock()
		if _, used := forwards[reqPort]; used {
			return 0, ErrMask | ErrPortUnavailable
		}
		forwards[reqPort] = struct{}{}
		return reqPort, 0
	}
	// pick first available
	lock.Lock()
	defer lock.Unlock()
	for p := start; p <= end; p++ {
		if _, used := forwards[p]; !used {
			forwards[p] = struct{}{}
			return p, 0
		}
	}
	return 0, ErrMask | ErrPortUnavailable
}

// processHandshake performs the SSH handshake steps for IP and whitelist.
// It sends ErrIPNotAllowed or ErrSuccess, reads whitelist count and entries, then confirms with ErrSuccess.
func processHandshake(rw io.ReadWriter, remoteHost string, allowed []string) ([]string, error) {
	var hb [4]byte
	// 1) IP check
	if len(allowed) > 0 && !isAllowed(remoteHost, allowed) {
		binary.BigEndian.PutUint32(hb[:], ErrIPNotAllowed)
		rw.Write(hb[:])
		return nil, fmt.Errorf("IP %s not allowed", remoteHost)
	}
	// IP OK
	binary.BigEndian.PutUint32(hb[:], ErrSuccess)
	rw.Write(hb[:])

	// 2) Read whitelist count
	if _, err := io.ReadFull(rw, hb[:]); err != nil {
		return nil, fmt.Errorf("read whitelist count: %w", err)
	}
	count := int(binary.BigEndian.Uint32(hb[:]))

	// 3) Read entries
	wl := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if _, err := io.ReadFull(rw, hb[:]); err != nil {
			return nil, fmt.Errorf("read whitelist entry length: %w", err)
		}
		length := int(binary.BigEndian.Uint32(hb[:]))
		buf := make([]byte, length)
		if _, err := io.ReadFull(rw, buf); err != nil {
			return nil, fmt.Errorf("read whitelist entry: %w", err)
		}
		wl = append(wl, string(buf))
	}

	// 4) Confirm whitelist
	binary.BigEndian.PutUint32(hb[:], ErrSuccess)
	rw.Write(hb[:])
	return wl, nil
}

// isAllowed checks if ip matches allowed list entries (exact or CIDR)
func isAllowed(ip string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	parsed := net.ParseIP(ip)
	for _, a := range allowed {
		if strings.Contains(a, "/") {
			if _, cidr, err := net.ParseCIDR(a); err == nil && cidr.Contains(parsed) {
				return true
			}
		} else if a == ip {
			return true
		}
	}
	return false
}
