package client

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/allowlist"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
)

const (
	maxConnectionAttempts = 5
	retryDelay            = 5 * time.Second
)

// Run connects to the server, opens the tunnel and reconnects when the connection
// is lost. It returns when the server refuses the tunnel, or stays unreachable for
// maxConnectionAttempts attempts in a row.
func Run(params *config.ClientParameters) error {
	if err := params.Validate(); err != nil {
		return fmt.Errorf("invalid client parameters: %w", err)
	}
	request, err := newTunnelRequest(params)
	if err != nil {
		return err
	}
	serverAddress := net.JoinHostPort(params.Endpoint, strconv.Itoa(params.EndpointPort))
	localAddress := net.JoinHostPort(params.LocalHost, strconv.Itoa(params.LocalPort))

	for failedAttempts := 0; failedAttempts < maxConnectionAttempts; {
		log.Printf("[*] Connecting to %s (attempt %d/%d)", serverAddress, failedAttempts+1, maxConnectionAttempts)
		sshClient, err := connect(params, serverAddress)
		if err != nil {
			log.Printf("[-] %v", err)
			failedAttempts++
			if failedAttempts < maxConnectionAttempts {
				time.Sleep(retryDelay)
			}
			continue
		}
		failedAttempts = 0

		err = runSession(sshClient, request, localAddress)
		if err != nil {
			log.Printf("[-] Session error: %v", err)
		}
		if !shouldReconnect(err) {
			return err
		}
		log.Printf("[*] Session closed, retrying in %v...", retryDelay)
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("failed to establish SSH connection after %d attempts", maxConnectionAttempts)
}

func newTunnelRequest(params *config.ClientParameters) (protocol.TunnelRequest, error) {
	transport, err := protocol.ParseTransport(params.Protocol)
	if err != nil {
		return protocol.TunnelRequest{}, err
	}
	allowedPeers, err := allowlist.Parse(params.AllowedIPs)
	if err != nil {
		return protocol.TunnelRequest{}, fmt.Errorf("allowed IPs: %w", err)
	}
	return protocol.TunnelRequest{AllowedPeers: allowedPeers, Transport: transport, Port: params.RemotePort}, nil
}

func connect(params *config.ClientParameters, serverAddress string) (*ssh.Client, error) {
	sshConfig, err := sshClientConfig(params)
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}
	sshClient, err := ssh.Dial("tcp", serverAddress, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	return sshClient, nil
}

func shouldReconnect(sessionErr error) bool {
	return sessionErr == nil || strings.Contains(sessionErr.Error(), "An existing connection was forcibly closed by the remote host")
}
