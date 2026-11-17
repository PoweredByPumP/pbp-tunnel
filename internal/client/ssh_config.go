package client

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshClientConfig authenticates with the password, the private key, or both, and
// checks the server identity as HostKeyLevel asks.
func sshClientConfig(params *config.ClientParameters) (*ssh.ClientConfig, error) {
	authMethods := []ssh.AuthMethod{}

	if params.Password != "" {
		authMethods = append(authMethods, ssh.Password(params.Password))
	}

	if params.PrivateKeyPath != "" {
		key, err := os.ReadFile(params.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	checkServerIdentity, err := hostKeyCallback(params.HostKeyLevel, params.HostKeyPath)
	if err != nil {
		return nil, err
	}
	return &ssh.ClientConfig{
		User:            params.Username,
		Auth:            authMethods,
		HostKeyCallback: checkServerIdentity,
	}, nil
}

func hostKeyCallback(level int, knownHostsPath string) (ssh.HostKeyCallback, error) {
	if level == config.HostKeyNotChecked {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	if knownHostsPath == "" {
		if level == config.HostKeyMustBeKnown {
			return nil, fmt.Errorf("host_key_level 2 needs a known_hosts file in host_key")
		}
		return warnWithoutChecking, nil
	}

	checkKnownHosts, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("read known hosts: %w", err)
	}
	if level == config.HostKeyMustBeKnown {
		return checkKnownHosts, nil
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := checkKnownHosts(hostname, remote, key); err != nil {
			log.Printf("[!] Server identity not verified, connecting anyway (host_key_level 1): %v", err)
		}
		return nil
	}, nil
}

func warnWithoutChecking(hostname string, remote net.Addr, key ssh.PublicKey) error {
	log.Printf("[!] Server identity not verified: no known_hosts file is set (host_key_level 1)")
	return nil
}
