package client

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
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
func Run(cpOverride *config.ClientParameters) error {
	var cp config.ClientParameters

	if cpOverride == nil {
		flag.StringVar(&cp.Endpoint, config.CpKeyEndpoint, config.CpDefaultEndpoint, "SSH server endpoint")
		flag.IntVar(&cp.EndpointPort, config.CpKeyEndpointPort, config.CpDefaultEndpointPort, "SSH server port")
		flag.StringVar(&cp.Username, config.CpKeyUsername, config.CpDefaultUsername, "SSH username")
		flag.StringVar(&cp.Password, config.CpKeyPassword, config.CpDefaultPassword, "SSH password")
		flag.StringVar(&cp.PrivateKeyPath, config.CpKeyPrivateKeyPath, config.CpDefaultPrivateKeyPath, "Private key path (optional)")
		flag.StringVar(&cp.HostKeyPath, config.CpKeyHostKeyPath, config.CpDefaultHostKeyPath, "Known host key file (optional)")
		flag.StringVar(&cp.LocalHost, config.CpKeyLocalHost, config.CpDefaultLocalHost, "Local address to forward")
		flag.IntVar(&cp.LocalPort, config.CpKeyLocalPort, config.CpDefaultLocalPort, "Local port to forward")
		flag.StringVar(&cp.RemoteHost, config.CpKeyRemoteHost, config.CpDefaultRemoteHost, "Remote host to expose (unused)")
		flag.IntVar(&cp.RemotePort, config.CpKeyRemotePort, config.CpDefaultRemotePort, "Remote port to request (0 = random)")
		flag.StringVar(&cp.Protocol, config.CpKeyProtocol, config.CpDefaultProtocol, "Forwarding protocol (tcp|udp)")
		flag.IntVar(&cp.HostKeyLevel, config.CpKeyHostKeyLevel, config.CpDefaultHostKeyLevel, "Host key level (0=no check,1=warn,2=strict)")
		flag.Var(&cp.AllowedIPs, config.CpKeyAllowedIPs, "Allowed IPs (comma-separated)")
		flag.Parse()
	} else {
		cp = *cpOverride
	}

	// Validate configuration
	if err := cp.Validate(); err != nil {
		return fmt.Errorf("invalid client parameters: %w", err)
	}

	const (
		maxRetries = 5
		retryDelay = 5 * time.Second
	)
	retry := 1

	for {
		log.Printf("[*] Connecting to %s:%d (attempt %d/%d)", cp.Endpoint, cp.EndpointPort, retry, maxRetries)

		sshCfg, addr, err := config.GetClientConfig(&cp)
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

				if err := session.runSession(&cp); err != nil {
					log.Printf("[-] Session error: %v", err)
					clientConn.Close()
					if !strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") {
						return err
					}
				}

				session.ActiveConnections.Wait()
				clientConn.Close()

				log.Printf("[*] Session closed, retrying in %v...", retryDelay)
				time.Sleep(retryDelay)
				retry = 1
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

// runSession requests the tunnel, then serves forwarded connections until the SSH connection ends.
func (s *ClientSession) runSession(cp *config.ClientParameters) error {
	transport, err := protocol.ParseTransport(cp.Protocol)
	if err != nil {
		return err
	}

	controlChannel, requests, err := s.Connection.OpenChannel("direct-tcpip", nil)
	if err != nil {
		return fmt.Errorf("open tunnel request channel: %w", err)
	}
	defer controlChannel.Close()
	go ssh.DiscardRequests(requests)

	log.Printf("[*] Requesting %s tunnel on remote port %d (allowed peers: %v)", transport, cp.RemotePort, cp.AllowedIPs)
	exposedPort, err := protocol.RequestTunnel(controlChannel, protocol.TunnelRequest{
		AllowedPeers: cp.AllowedIPs,
		Transport:    transport,
		Port:         cp.RemotePort,
	})
	if err != nil {
		return fmt.Errorf("tunnel request: %w", err)
	}
	s.AssignedPort = exposedPort
	log.Printf("[+] Assigned remote port %d (local %s)", s.AssignedPort, s.LocalAddress)

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
			if transport == protocol.UDP {
				go s.handleForwardUDP(ch2, id)
			} else {
				go s.handleForwardTCP(ch2, id)
			}
		}
	}()

	// Wait for session end
	return s.Connection.Wait()
}

// handleForward manages a single forwarded connection
func (s *ClientSession) handleForwardTCP(ch ssh.Channel, id int) {
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

func (s *ClientSession) handleForwardUDP(ch ssh.Channel, id int) {
	defer ch.Close()
	defer s.ActiveConnections.Done()

	localAddr, err := net.ResolveUDPAddr("udp", s.LocalAddress)
	if err != nil {
		log.Printf("[-] Resolve local UDP %s: %v", s.LocalAddress, err)
		return
	}

	localConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		log.Printf("[-] Connect to local UDP %s: %v", s.LocalAddress, err)
		return
	}
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer localConn.Close()
		for {
			packet, err := protocol.ReadDatagram(ch)
			if err != nil {
				if err != io.EOF {
					log.Printf("[-] UDP read from server for forward #%d: %v", id, err)
				}
				return
			}
			if _, err := localConn.Write(packet); err != nil {
				log.Printf("[-] UDP write to local for forward #%d: %v", id, err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, protocol.MaxDatagramSize)
		for {
			n, err := localConn.Read(buf)
			if err != nil {
				if !strings.Contains(err.Error(), "closed") {
					log.Printf("[-] UDP read from local for forward #%d: %v", id, err)
				}
				return
			}
			if err := protocol.WriteDatagram(ch, buf[:n]); err != nil {
				log.Printf("[-] UDP write to server for forward #%d: %v", id, err)
				return
			}
		}
	}()

	wg.Wait()
	log.Printf("[+] UDP forward #%d closed", id)
}
