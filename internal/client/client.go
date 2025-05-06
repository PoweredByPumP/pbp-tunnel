package client

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
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

// ClientSession holds state for a running SSH tunnel session
type ClientSession struct {
	Connection        *ssh.Client
	AssignedPort      int
	LocalAddress      string
	Active            bool
	Lock              sync.Mutex
	ConnectionCount   int
	ActiveConnections sync.WaitGroup
}

// Run establishes the SSH connection and manages retries, handshake, and forwarding
func Run(cp *config.ClientParameters) error {
	flag.Parse()
	// Validate configuration
	if err := cp.Validate(); err != nil {
		return fmt.Errorf("invalid client parameters: %w", err)
	}

	const (
		maxRetries = 5
		retryDelay = 5 * time.Second
	)
	retry := 0

	for {
		log.Printf("[*] Connecting to %s:%d (attempt %d/%d)", cp.Endpoint, cp.EndpointPort, retry+1, maxRetries)

		sshCfg, addr, err := config.GetClientConfig(cp)
		if err != nil {
			log.Printf("[-] Config error: %v", err)
		} else {
			clientConn, err := ssh.Dial("tcp", addr, sshCfg)
			if err != nil {
				log.Printf("[-] Dial error: %v", err)
			} else {
				// Run session
				session := &ClientSession{
					Connection:   clientConn,
					LocalAddress: fmt.Sprintf("%s:%d", cp.LocalHost, cp.LocalPort),
					Active:       true,
				}

				if err := session.runSession(cp); err != nil {
					log.Printf("[-] Session error: %v", err)
					clientConn.Close()
					return err
				}

				session.ActiveConnections.Wait()
				clientConn.Close()

				log.Printf("[*] Session closed, retrying in %v...", retryDelay)
				time.Sleep(retryDelay)
				retry = 0
				continue
			}
		}

		if retry < maxRetries {
			retry++
			time.Sleep(retryDelay)
			continue
		}
		return fmt.Errorf("failed to establish SSH connection after %d attempts", maxRetries)
	}
}

// runSession handles the handshake and incoming forwards for a connected SSH session
func (s *ClientSession) runSession(cp *config.ClientParameters) error {
	// 1) Open a channel for handshake
	ch, reqs, err := s.Connection.OpenChannel("direct-tcpip", nil)
	if err != nil {
		return fmt.Errorf("open handshake channel: %w", err)
	}
	defer ch.Close()
	go ssh.DiscardRequests(reqs)

	var hb [4]byte

	// 2) Read handshake response
	if _, err := io.ReadFull(ch, hb[:]); err != nil {
		return fmt.Errorf("handshake read error: %w", err)
	}
	code := binary.BigEndian.Uint32(hb[:])
	switch code {
	case ErrSuccess:
		log.Printf("[+] Handshake OK")
	case ErrIPNotAllowed:
		return fmt.Errorf("server rejected IP: code %d", code)
	default:
		return fmt.Errorf("handshake failed with code %d", code)
	}

	// 3) Send whitelist
	log.Printf("[*] Sending whitelist: %v", cp.AllowedIPs)
	binary.BigEndian.PutUint32(hb[:], uint32(len(cp.AllowedIPs)))
	if _, err := ch.Write(hb[:]); err != nil {
		return fmt.Errorf("send whitelist length: %w", err)
	}
	for _, ip := range cp.AllowedIPs {
		data := []byte(ip)
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(data)))
		ch.Write(l[:])
		ch.Write(data)
		log.Printf("[+] Whitelist entry sent: %s", ip)
	}

	// 4) Read whitelist confirmation
	if _, err := io.ReadFull(ch, hb[:]); err != nil {
		return fmt.Errorf("whitelist confirm read error: %w", err)
	}
	if binary.BigEndian.Uint32(hb[:]) != ErrSuccess {
		return fmt.Errorf("whitelist rejected by server")
	}
	log.Printf("[+] Whitelist accepted by server")

	// 5) Request port
	log.Printf("[*] Requesting remote port %d", cp.RemotePort)
	binary.BigEndian.PutUint32(hb[:], uint32(cp.RemotePort))
	if _, err := ch.Write(hb[:]); err != nil {
		return fmt.Errorf("send port request: %w", err)
	}

	// 6) Read assigned port or error
	if _, err := io.ReadFull(ch, hb[:]); err != nil {
		return fmt.Errorf("read port response error: %w", err)
	}
	val := binary.BigEndian.Uint32(hb[:])
	if val&ErrMask != 0 {
		errCode := val &^ ErrMask
		switch errCode {
		case ErrPortUnavailable:
			return fmt.Errorf("server: no available ports")
		case ErrPortOutOfRange:
			return fmt.Errorf("server: port out of range")
		case ErrInternal:
			return fmt.Errorf("server: internal error")
		default:
			return fmt.Errorf("server error code %d", errCode)
		}
	}
	s.AssignedPort = int(val)
	log.Printf("[+] Assigned remote port %d (local %s)", s.AssignedPort, s.LocalAddress)

	// 7) Handle forwarded connections
	go func() {
		for newCh := range s.Connection.HandleChannelOpen("direct-tcpip") {
			if !s.Active {
				newCh.Reject(ssh.ConnectionFailed, "session closed")
				continue
			}
			ch2, reqs2, err := newCh.Accept()
			if err != nil {
				log.Printf("[-] Accept forwarded channel: %v", err)
				continue
			}
			go ssh.DiscardRequests(reqs2)

			s.Lock.Lock()
			s.ConnectionCount++
			id := s.ConnectionCount
			s.Lock.Unlock()

			s.ActiveConnections.Add(1)
			log.Printf("[*] Forward #%d incoming", id)
			go s.handleForward(ch2, id)
		}
	}()

	// Wait for session end
	return s.Connection.Wait()
}

// handleForward manages a single forwarded connection
func (s *ClientSession) handleForward(ch ssh.Channel, id int) {
	defer ch.Close()
	defer s.ActiveConnections.Done()

	localConn, err := net.Dial("tcp", s.LocalAddress)
	if err != nil {
		log.Printf("[-] Connect to local %s: %v", s.LocalAddress, err)
		return
	}
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(localConn, ch)
		log.Printf("[*] Copied %d bytes to local for forward #%d", n, id)
		localConn.(*net.TCPConn).CloseRead()
	}()
	go func() {
		defer wg.Done()
		n, _ := io.Copy(ch, localConn)
		log.Printf("[*] Copied %d bytes to server for forward #%d", n, id)
		ch.CloseWrite()
	}()
	wg.Wait()
	log.Printf("[+] Forward #%d closed", id)
}
