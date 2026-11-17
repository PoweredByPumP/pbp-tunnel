package client

import (
	"fmt"
	"os"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshClientConfig authenticates with the password, the private key, or both.
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

	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if params.HostKeyPath != "" {
		callback, err := knownhosts.New(params.HostKeyPath)
		if err == nil {
			hostKeyCallback = callback
		}
	}
	return &ssh.ClientConfig{
		User:            params.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}, nil
}
