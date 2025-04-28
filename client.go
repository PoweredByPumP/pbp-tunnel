package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type ClientSession struct {
	Connection        *ssh.Client
	AssignedPort      int
	LocalAddress      string
	Active            bool
	Lock              sync.Mutex
	ConnectionCount   int
	ActiveConnections sync.WaitGroup
}

func Client(cpOverride *ClientParameters) {
	var cp ClientParameters
	if cpOverride == nil {
		flag.StringVar(&cp.Endpoint, CpKeyEndpoint, CpDefaultEndpoint, "SSH server endpoint")
		flag.IntVar(&cp.EndpointPort, CpKeyEndpointPort, CpDefaultEndpointPort, "SSH server port")
		flag.StringVar(&cp.Username, CpKeyUsername, CpDefaultUsername, "SSH username")
		flag.StringVar(&cp.Password, CpKeyPassword, CpDefaultPassword, "SSH password")
		flag.StringVar(&cp.PrivateKeyPath, CpKeyPrivateKeyPath, CpDefaultPrivateKeyPath, "Private key path (optional)")
		flag.StringVar(&cp.HostKeyPath, CpKeyHostKeyPath, CpDefaultHostKeyPath, "Known host key file (optional)")
		flag.StringVar(&cp.LocalHost, CpKeyLocalHost, CpDefaultLocalHost, "Local address to forward")
		flag.IntVar(&cp.LocalPort, CpKeyLocalPort, CpDefaultLocalPort, "Local port to forward")
		flag.StringVar(&cp.RemoteHost, CpKeyRemoteHost, CpDefaultRemoteHost, "Remote host to expose (unused)")
		flag.IntVar(&cp.RemotePort, CpKeyRemotePort, CpDefaultRemotePort, "Remote port to request (0 = random)")
		flag.IntVar(&cp.HostKeyLevel, CpKeyHostKeyLevel, CpDefaultHostKeyLevel, "Host key level (0=no check,1=warn,2=strict)")
		flag.Parse()
	} else {
		cp = *cpOverride
	}

	if err := cp.Validate(); err != nil {
		log.Fatalf("Invalid client parameters: %v", err)
	}

	retryCount := 0
	const maxRetries = 5
	const retryDelay = 5 * time.Second

	for {
		log.Printf("[*] Connecting to server %s:%d (attempt %d/%d)...", cp.Endpoint, cp.EndpointPort, retryCount+1, maxRetries)
		config, err := GetClientConfig(cp)
		if err != nil {
			log.Printf("[-] Config error: %v", err)
			if retryCount >= maxRetries {
				log.Fatalf("[-] Giving up after %d attempts", maxRetries)
			}
			retryCount++
			time.Sleep(retryDelay)
			continue
		}

		addr := fmt.Sprintf("%s:%d", cp.Endpoint, cp.EndpointPort)
		conn, err := ssh.Dial("tcp", addr, config)
		if err != nil {
			log.Printf("[-] Dial error: %v", err)
			if retryCount >= maxRetries {
				log.Fatalf("[-] Giving up after %d attempts", maxRetries)
			}
			retryCount++
			time.Sleep(retryDelay)
			continue
		}

		log.Printf("[+] Connected to server %s:%d", cp.Endpoint, cp.EndpointPort)
		session := &ClientSession{
			Connection:   conn,
			LocalAddress: fmt.Sprintf("%s:%d", cp.LocalHost, cp.LocalPort),
			Active:       true,
		}

		if err := handleClientSession(session, cp); err != nil {
			log.Fatalf("[-] Something happened when handling client session: %v", err)
		}

		log.Printf("[*] Waiting for active connections to finish...")
		session.ActiveConnections.Wait()
		log.Printf("[+] All connections closed")

		conn.Close()
		log.Printf("[*] Disconnected, retrying in %v...", retryDelay)
		time.Sleep(retryDelay)
		retryCount = 0
	}
}

func handleClientSession(session *ClientSession, cp ClientParameters) error {
	conn := session.Connection

	channel, reqs, err := conn.OpenChannel("direct-tcpip", nil)
	if err != nil {
		return fmt.Errorf("failed to open channel: %v", err)
	}
	defer channel.Close()
	go ssh.DiscardRequests(reqs)

	// 1) Handshake
	var hb [4]byte
	if _, err := io.ReadFull(channel, hb[:]); err != nil {
		return fmt.Errorf("failed to read handshake code: %v", err)
	}
	code := binary.BigEndian.Uint32(hb[:])
	switch code {
	case ErrSuccess:
		// proceed
	case ErrIPNotAllowed:
		return fmt.Errorf("error: your IP is not allowed by server whitelist (code %d)", code)
	default:
		return fmt.Errorf("error: server handshake error (code %d)", code)
	}

	// 2) Port request
	var reqBuf [4]byte
	binary.BigEndian.PutUint32(reqBuf[:], uint32(cp.RemotePort))
	log.Printf("[*] Sending port request: %d", cp.RemotePort)
	if _, err := channel.Write(reqBuf[:]); err != nil {
		return fmt.Errorf("failed to send port request: %v", err)
	}

	// 3) Read assignment or error
	var resp [4]byte
	if _, err := io.ReadFull(channel, resp[:]); err != nil {
		return fmt.Errorf("failed to read port response: %v", err)
	}
	val := binary.BigEndian.Uint32(resp[:])
	if (val & ErrMask) != 0 {
		errCode := val &^ ErrMask
		switch errCode {
		case ErrPortUnavailable:
			return fmt.Errorf("error: no available ports on server (code %d)", errCode)
		case ErrPortOutOfRange:
			return fmt.Errorf("error: requested port %d is outside allowed range (code %d)", cp.RemotePort, errCode)
		case ErrInternal:
			return fmt.Errorf("error: internal server error occurred (code %d)", errCode)
		default:
			return fmt.Errorf("error: unknown server error (code %d)", errCode)
		}
	}

	// 4) Success
	session.AssignedPort = int(val)
	log.Printf("[+] Remote port assigned: %d (for local %s)", session.AssignedPort, session.LocalAddress)

	// 5) Forward loop
	go func() {
		for newCh := range session.Connection.HandleChannelOpen("direct-tcpip") {
			if !session.Active {
				newCh.Reject(ssh.ConnectionFailed, "session closed")
				continue
			}
			ch, reqs2, err := newCh.Accept()
			if err != nil {
				log.Printf("[-] Failed to accept forwarded channel: %v", err)
				continue
			}
			go ssh.DiscardRequests(reqs2)

			session.Lock.Lock()
			session.ConnectionCount++
			id := session.ConnectionCount
			session.Lock.Unlock()

			session.ActiveConnections.Add(1)
			go handleForwardedConnection(session, ch, session.LocalAddress, id)
		}
	}()

	err = session.Connection.Wait()
	session.Lock.Lock()
	session.Active = false
	session.Lock.Unlock()
	return err
}

func handleForwardedConnection(session *ClientSession, channel ssh.Channel, localAddr string, id int) {
	defer channel.Close()
	defer session.ActiveConnections.Done()

	log.Printf("[+] Forwarding connection #%d to local %s", id, localAddr)
	localConn, err := net.DialTimeout("tcp", localAddr, 10*time.Second)
	if err != nil {
		log.Printf("[-] Connection #%d failed: %v", id, err)
		return
	}
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		n, _ := io.Copy(localConn, channel)
		log.Printf("[*] Copied %d bytes from server for connection #%d", n, id)
		if tcp, ok := localConn.(*net.TCPConn); ok {
			tcp.CloseRead()
		}
	}()

	go func() {
		defer wg.Done()
		n, _ := io.Copy(channel, localConn)
		log.Printf("[*] Copied %d bytes to server for connection #%d", n, id)
		channel.CloseWrite()
	}()

	wg.Wait()
	log.Printf("[+] Connection #%d closed", id)
}
