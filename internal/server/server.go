package server

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/allowlist"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/hostkey"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

type ForwardServer struct {
	sshConfig      *ssh.ServerConfig
	bindAddress    string
	bindPort       int
	portRangeStart int
	portRangeEnd   int
	allowedClients allowlist.List
	forwards       map[int]struct{}
	lock           sync.Mutex
}

// ForwardServer maintains state for port forwarding
// sshConfig: SSH server configuration
// bindAddress/Port: where to expose forwarded ports
// portRangeStart/End: allowed range
// allowedClients: IPs allowed to request a tunnel
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
	allowedClients, err := allowlist.Parse(sp.AllowedIPs)
	if err != nil {
		return fmt.Errorf("invalid server parameters: allowed IPs: %w", err)
	}
	if err := generateMissingHostKeys(&sp); err != nil {
		return err
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
		allowedClients: allowedClients,
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

func generateMissingHostKeys(params *config.ServerParameters) error {
	hostKeys := []struct {
		path      string
		algorithm hostkey.Algorithm
	}{
		{params.PrivateRsaPath, hostkey.RSA},
		{params.PrivateEcdsaPath, hostkey.ECDSA},
		{params.PrivateEd25519Path, hostkey.Ed25519},
	}
	for _, hostKey := range hostKeys {
		if hostKey.path == "" {
			continue
		}
		created, err := hostkey.EnsureExists(hostKey.path, hostKey.algorithm)
		if err != nil {
			return fmt.Errorf("generate %s host key at %s: %w", hostKey.algorithm, hostKey.path, err)
		}
		if created {
			log.Printf("[+] Generated %s host key at %s", hostKey.algorithm, hostKey.path)
		}
	}
	return nil
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

	log.Printf("[+] New SSH connection from %s", sshConn.RemoteAddr())
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

// handleChannel answers a tunnel request, then forwards traffic until the client disconnects.
func (s *ForwardServer) handleChannel(sshConn *ssh.ServerConn, channel ssh.Channel) {
	defer channel.Close()

	client := sshConn.RemoteAddr()
	if !s.allowedClients.Allows(client) {
		_ = protocol.RejectPeer(channel)
		log.Printf("[-] Tunnel request from %s rejected: IP not allowed", client)
		return
	}

	request, err := protocol.ReceiveTunnelRequest(channel)
	if err != nil {
		log.Printf("[-] Tunnel request from %s failed: %v", client, err)
		return
	}
	log.Printf("[*] Tunnel request from %s: transport=%s port=%d allowed peers=%v",
		client, request.Transport, request.Port, request.AllowedPeers)

	port, err := assignPort(request.Port, s.portRangeStart, s.portRangeEnd, s.forwards, &s.lock)
	if err != nil {
		_ = protocol.RefuseTunnel(channel, err)
		log.Printf("[-] Port assignment failed: %v", err)
		return
	}
	log.Printf("[+] Assigned port %d", port)

	if request.Transport == protocol.UDP {
		s.serveUDPForward(sshConn, channel, request.AllowedPeers, port)
	} else {
		s.serveTCPForward(sshConn, channel, request.AllowedPeers, port)
	}

	log.Printf("[*] Waiting for lock to release port %d", port)
	s.lock.Lock()

	log.Printf("[*] Client disconnected, freed port %d", port)
	delete(s.forwards, port)

	s.lock.Unlock()
}

func (s *ForwardServer) serveTCPForward(sshConn *ssh.ServerConn, channel ssh.Channel, allowedPeers allowlist.List, port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.bindAddress, port))
	if err != nil {
		_ = protocol.RefuseTunnel(channel, protocol.StatusInternalError)
		log.Printf("[-] Bind TCP port %d failed: %v", port, err)
		return
	}
	defer ln.Close()

	_ = protocol.AcceptTunnel(channel, port)
	log.Printf("[+] Notified client of TCP port %d", port)

	done := make(chan struct{})
	go func() {
		_ = sshConn.Wait()
		ln.Close()
		close(done)
	}()

	var wg sync.WaitGroup
	var connsLock sync.Mutex
	conns := make(map[net.Conn]struct{})

	// Ensure every external connection is closed once the forward ends;
	// otherwise an idle peer would block wg.Wait() and keep the port reserved.
	defer func() {
		connsLock.Lock()
		for c := range conns {
			c.Close()
		}
		conns = nil
		connsLock.Unlock()
		wg.Wait()
	}()

	for id := 0; ; id++ {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-done:
			default:
				log.Printf("[-] Forward accept error: %v", err)
			}
			return
		}

		if !allowedPeers.Allows(conn.RemoteAddr()) {
			log.Printf("[-] Connection from %s rejected: not in allowed peers", conn.RemoteAddr())
			conn.Close()
			continue
		}

		connsLock.Lock()
		if conns == nil {
			connsLock.Unlock()
			conn.Close()
			return
		}
		conns[conn] = struct{}{}
		connsLock.Unlock()

		wg.Add(1)
		go func(c net.Conn, idx int) {
			defer wg.Done()
			defer func() {
				c.Close()
				connsLock.Lock()
				if conns != nil {
					delete(conns, c)
				}
				connsLock.Unlock()
			}()

			ch2, reqs3, err := sshConn.OpenChannel("direct-tcpip", nil)
			if err != nil {
				log.Printf("[-] Open back-channel failed: %v", err)
				return
			}
			go ssh.DiscardRequests(reqs3)
			defer ch2.Close()

			var cc sync.WaitGroup
			cc.Add(2)
			go func() {
				defer cc.Done()
				n, _ := io.Copy(ch2, c)
				log.Printf("[*] Copied %d bytes to client for forward %d", n, idx)
				ch2.CloseWrite()
			}()
			go func() {
				defer cc.Done()
				n, _ := io.Copy(c, ch2)
				log.Printf("[*] Copied %d bytes to service for forward %d", n, idx)
				if tc, ok := c.(*net.TCPConn); ok {
					tc.CloseWrite()
				}
			}()
			cc.Wait()
			log.Printf("[+] Forward %d closed", idx)
		}(conn, id)
	}
}

func (s *ForwardServer) serveUDPForward(sshConn *ssh.ServerConn, channel ssh.Channel, allowedPeers allowlist.List, port int) {
	pc, err := net.ListenPacket("udp", fmt.Sprintf("%s:%d", s.bindAddress, port))
	if err != nil {
		_ = protocol.RefuseTunnel(channel, protocol.StatusInternalError)
		log.Printf("[-] Bind UDP port %d failed: %v", port, err)
		return
	}
	defer pc.Close()

	_ = protocol.AcceptTunnel(channel, port)
	log.Printf("[+] Notified client of UDP port %d", port)

	done := make(chan struct{})
	go func() {
		_ = sshConn.Wait()
		pc.Close()
		close(done)
	}()

	type udpPeerSession struct {
		ch ssh.Channel
	}

	sessions := make(map[string]*udpPeerSession)
	var sessionsLock sync.Mutex

	closeSession := func(key string) {
		sessionsLock.Lock()
		sess, ok := sessions[key]
		if ok {
			delete(sessions, key)
		}
		sessionsLock.Unlock()
		if ok {
			sess.ch.Close()
		}
	}

	buf := make([]byte, protocol.MaxDatagramSize)
	for {
		n, peerAddr, err := pc.ReadFrom(buf)
		if err != nil {
			select {
			case <-done:
				sessionsLock.Lock()
				for key, sess := range sessions {
					delete(sessions, key)
					sess.ch.Close()
				}
				sessionsLock.Unlock()
				return
			default:
				log.Printf("[-] UDP read failed on port %d: %v", port, err)
				continue
			}
		}

		if !allowedPeers.Allows(peerAddr) {
			continue
		}

		key := peerAddr.String()
		sessionsLock.Lock()
		sess, ok := sessions[key]
		if !ok {
			ch2, reqs3, openErr := sshConn.OpenChannel("direct-tcpip", nil)
			if openErr != nil {
				sessionsLock.Unlock()
				log.Printf("[-] Open UDP back-channel failed: %v", openErr)
				continue
			}
			go ssh.DiscardRequests(reqs3)
			sess = &udpPeerSession{ch: ch2}
			sessions[key] = sess
			go func(peer net.Addr, peerKey string, ch ssh.Channel) {
				defer closeSession(peerKey)
				for {
					packet, readErr := protocol.ReadDatagram(ch)
					if readErr != nil {
						if readErr != io.EOF {
							log.Printf("[-] UDP read from client failed (%s): %v", peerKey, readErr)
						}
						return
					}
					if _, writeErr := pc.WriteTo(packet, peer); writeErr != nil {
						log.Printf("[-] UDP write to peer failed (%s): %v", peerKey, writeErr)
						return
					}
				}
			}(peerAddr, key, ch2)
		}
		sessionsLock.Unlock()

		if err := protocol.WriteDatagram(sess.ch, buf[:n]); err != nil {
			log.Printf("[-] UDP write to client failed (%s): %v", key, err)
			closeSession(key)
		}
	}
}

// assignPort reserves requestedPort, or the first free port of the range when
// requestedPort is 0. A refusal is returned as a protocol.Status.
func assignPort(requestedPort, start, end int, forwards map[int]struct{}, lock *sync.Mutex) (int, error) {
	if start > end {
		return 0, protocol.StatusPortUnavailable
	}
	if requestedPort != 0 {
		if requestedPort < start || requestedPort > end {
			return 0, protocol.StatusPortOutOfRange
		}
		lock.Lock()
		defer lock.Unlock()
		if _, used := forwards[requestedPort]; used {
			return 0, protocol.StatusPortUnavailable
		}
		forwards[requestedPort] = struct{}{}
		return requestedPort, nil
	}
	lock.Lock()
	defer lock.Unlock()
	for port := start; port <= end; port++ {
		if _, used := forwards[port]; !used {
			forwards[port] = struct{}{}
			return port, nil
		}
	}
	return 0, protocol.StatusPortUnavailable
}
