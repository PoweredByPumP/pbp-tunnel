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

// On supprime le session.Listener car on ne veut PAS écouter localement (reverse tunnel).

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

	if err := cp.Validate(); err != nil {
		log.Fatalf("Invalid client parameters: %v", err)
	}

	retryCount := 0
	maxRetries := 5
	retryDelay := 5 * time.Second

	for {
		log.Printf("[*] Connecting to server %s:%d (attempt %d/%d)...", cp.Endpoint, cp.EndpointPort, retryCount+1, maxRetries)

		config, err := GetClientConfig(cp)
		if err != nil {
			log.Printf("[-] Failed to get client config: %v", err)
			if retryCount >= maxRetries {
				log.Fatalf("[-] Failed to create client config after %d attempts", maxRetries)
			}
			retryCount++
			time.Sleep(retryDelay)
			continue
		}

		address := fmt.Sprintf("%s:%d", cp.Endpoint, cp.EndpointPort)
		connection, err := ssh.Dial("tcp", address, config)
		if err != nil {
			log.Printf("[-] Failed to connect to server (try %d/%d): %v", retryCount+1, maxRetries, err)
			if retryCount >= maxRetries {
				log.Fatalf("[-] Failed to connect after %d attempts: %v", maxRetries, err)
			}
			retryCount++
			time.Sleep(retryDelay)
			continue
		}

		log.Printf("[+] Connected to server %s:%d", cp.Endpoint, cp.EndpointPort)

		session := &ClientSession{
			Connection:   connection,
			AssignedPort: 0,
			LocalAddress: cp.GetFormattedAddress(),
			Active:       true,
		}

		// Bloque tant que la session est active, ou qu’il y a une erreur
		err = handleClientSession(session, cp)

		// Si handleClientSession sort, c'est qu'il y a eu un souci ou que le SSH s'est coupé
		log.Printf("[-] Session ended with error: %v", err)

		// Attendre la fin des transferts restants
		log.Printf("[*] Waiting for any remaining active connections to terminate")
		session.ActiveConnections.Wait()
		log.Printf("[+] All connections terminated")

		// On ferme la connexion. (Souvent déjà fermée, mais au cas où.)
		_ = connection.Close()

		log.Printf("[*] Disconnected from server, retrying in %v...", retryDelay)
		time.Sleep(retryDelay)
		retryCount = 0
	}
}

func handleClientSession(session *ClientSession, cp ClientParameters) error {
	localAddress := cp.GetFormattedAddress()
	conn := session.Connection

	log.Printf("[*] Requesting port forwarding setup")

	// 1) Ouvrir canal pour demander le port côté serveur
	channel, reqs, err := conn.OpenChannel("direct-tcpip", nil)
	if err != nil {
		return fmt.Errorf("failed to open SSH channel: %v", err)
	}
	defer channel.Close()
	go ssh.DiscardRequests(reqs)

	// 2) Envoi d'un int32 = cp.RemotePort
	reqBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(reqBuf, uint32(cp.RemotePort))

	log.Printf("[*] Sending port request: %d", cp.RemotePort)
	if _, err := channel.Write(reqBuf); err != nil {
		return fmt.Errorf("failed to send port request: %v", err)
	}

	// 3) Lecture de l'int32 renvoyé = assignedPort
	respBuf := make([]byte, 4)
	if _, err := io.ReadFull(channel, respBuf); err != nil {
		return fmt.Errorf("failed to read assigned port: %v", err)
	}
	assignedPort := int(binary.BigEndian.Uint32(respBuf))
	log.Printf("[+] Remote port assigned by server: %d (for local %s)", assignedPort, localAddress)

	session.AssignedPort = assignedPort

	// 4) Lancer la gestion du reverse tunnel
	go func() {
		// Pour chaque nouveau canal direct-tcpip que le serveur ouvre vers le client
		for newChannel := range conn.HandleChannelOpen("direct-tcpip") {
			if !session.Active {
				log.Printf("[*] Session no longer active, rejecting new channel")
				newChannel.Reject(ssh.ConnectionFailed, "session closed")
				continue
			}
			channel, requests, err := newChannel.Accept()
			if err != nil {
				log.Printf("[-] Failed to accept forwarded channel: %v", err)
				continue
			}
			go ssh.DiscardRequests(requests)

			// Incrémentation du compteur
			session.Lock.Lock()
			session.ConnectionCount++
			connID := session.ConnectionCount
			session.Lock.Unlock()

			// Comptage des transferts actifs
			session.ActiveConnections.Add(1)

			// Gérer le forward
			go handleForwardedConnection(session, channel, localAddress, connID)
		}
		log.Printf("[*] Channel handling loop exited")
	}()

	// 5) Bloquer tant que la connexion SSH n'est pas morte
	err = conn.Wait()
	session.Lock.Lock()
	session.Active = false
	session.Lock.Unlock()
	return err
}

func handleForwardedConnection(session *ClientSession, channel ssh.Channel, localAddress string, connID int) {
	defer channel.Close()
	defer session.ActiveConnections.Done()

	log.Printf("[+] Accepted connection #%d from server, forwarding to local %s", connID, localAddress)

	session.Lock.Lock()
	active := session.Active
	session.Lock.Unlock()
	if !active {
		log.Printf("[*] Session no longer active, closing connection #%d", connID)
		return
	}

	// Se connecter au service local
	localConn, err := net.DialTimeout("tcp", localAddress, 10*time.Second)
	if err != nil {
		log.Printf("[-] Connection #%d to local service failed for %s: %v", connID, localAddress, err)
		return
	}
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Serveur -> Local
	go func() {
		defer wg.Done()
		bytes, err := io.Copy(localConn, channel)
		if err != nil {
			log.Printf("[-] Error copying data from server to local for connection #%d: %v", connID, err)
		}
		log.Printf("[*] Copied %d bytes from server to local for connection #%d", bytes, connID)
		if tcpConn, ok := localConn.(*net.TCPConn); ok {
			_ = tcpConn.CloseRead()
		}
	}()

	// Local -> Serveur
	go func() {
		defer wg.Done()
		bytes, err := io.Copy(channel, localConn)
		if err != nil {
			log.Printf("[-] Error copying data from local to server for connection #%d: %v", connID, err)
		}
		log.Printf("[*] Copied %d bytes from local to server for connection #%d", bytes, connID)
		_ = channel.CloseWrite()
	}()

	wg.Wait()
	log.Printf("[+] Data transfer completed for connection #%d", connID)
}
