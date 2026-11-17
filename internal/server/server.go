package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/allowlist"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
)

// Server accepts SSH clients and exposes one of its ports for each of them.
type Server struct {
	sshConfig *ssh.ServerConfig
	// bindAddress is where the exposed ports listen.
	bindAddress    string
	allowedClients allowlist.List
	ports          *portPool
}

func Run(params *config.ServerParameters) error {
	server, err := newServer(params)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(params.BindAddress, strconv.Itoa(params.BindPort))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	defer listener.Close()
	log.Printf("[+] SSH server listening on %s", address)
	return server.serve(listener)
}

func newServer(params *config.ServerParameters) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid server parameters: %w", err)
	}
	allowedClients, err := allowlist.Parse(params.AllowedIPs)
	if err != nil {
		return nil, fmt.Errorf("invalid server parameters: allowed IPs: %w", err)
	}
	if err := generateMissingHostKeys(params); err != nil {
		return nil, err
	}
	sshConfig, err := sshServerConfig(params)
	if err != nil {
		return nil, fmt.Errorf("failed to build server config: %w", err)
	}
	return &Server{
		sshConfig:      sshConfig,
		bindAddress:    params.BindAddress,
		allowedClients: allowedClients,
		ports:          newPortPool(params.PortRangeStart, params.PortRangeEnd),
	}, nil
}

// serve handles every client accepted on listener until listener is closed.
func (s *Server) serve(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		if err != nil {
			log.Printf("[-] Accept error: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go s.serveConnection(conn)
	}
}
