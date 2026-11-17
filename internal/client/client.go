package client

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/allowlist"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
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
	sshConfig, err := sshClientConfig(params)
	if err != nil {
		return err
	}
	serverAddress := net.JoinHostPort(params.Endpoint, strconv.Itoa(params.EndpointPort))
	localAddress := net.JoinHostPort(params.LocalHost, strconv.Itoa(params.LocalPort))

	for failedAttempts := 0; failedAttempts < maxConnectionAttempts; {
		log.Printf("[*] Connecting to %s (attempt %d/%d)", serverAddress, failedAttempts+1, maxConnectionAttempts)
		sshClient, err := ssh.Dial("tcp", serverAddress, sshConfig)
		if isServerIdentityRejected(err) {
			return fmt.Errorf("server identity rejected: %w", err)
		}
		if err != nil {
			log.Printf("[-] Dial error: %v", err)
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

// isServerIdentityRejected reports a server whose key is missing from, or differs
// from, the known_hosts file. Trying again would only reach the same server.
func isServerIdentityRejected(dialErr error) bool {
	var keyErr *knownhosts.KeyError
	return errors.As(dialErr, &keyErr)
}

// shouldReconnect is false when the server refused the tunnel, since asking again
// would get the same answer. A session ending for any other reason, such as the
// server restarting or the network failing, is worth a new connection.
func shouldReconnect(sessionErr error) bool {
	var refusal protocol.Status
	return !errors.As(sessionErr, &refusal)
}
