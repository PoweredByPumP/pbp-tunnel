package config

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// buildSSHClientConfig creates ssh.ClientConfig from ClientParameters\

func buildSSHClientConfig(params *ClientParameters) (*ssh.ClientConfig, error) {
	authMethods := []ssh.AuthMethod{}
	// password auth
	if params.Password != "" {
		authMethods = append(authMethods, ssh.Password(params.Password))
	}
	// key auth
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
	// host key callback
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

// GetClientConfig returns an SSH client config and target address
func GetClientConfig(params *ClientParameters) (*ssh.ClientConfig, string, error) {
	sshCfg, err := buildSSHClientConfig(params)
	if err != nil {
		return nil, "", err
	}
	addr := fmt.Sprintf("%s:%d", params.Endpoint, params.EndpointPort)
	return sshCfg, addr, nil
}

// buildSSHServerConfig creates ssh.ServerConfig from ServerParameters
func buildSSHServerConfig(params *ServerParameters) (*ssh.ServerConfig, error) {
	serverCfg := &ssh.ServerConfig{}
	// allow password auth if set
	if params.Password != "" {
		serverCfg.PasswordCallback = func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == params.Username && string(pass) == params.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		}
	}
	// load host keys
	for _, path := range []string{params.PrivateRsaPath, params.PrivateEcdsaPath, params.PrivateEd25519Path} {
		if path == "" {
			continue
		}
		keyBytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err == nil {
			serverCfg.AddHostKey(signer)
		}
	}
	// (AuthorizedKeysPath support can be added here)
	return serverCfg, nil
}

// GetServerConfig returns an SSH server config and listen address
func GetServerConfig(params *ServerParameters) (*ssh.ServerConfig, string, error) {
	sshCfg, err := buildSSHServerConfig(params)
	if err != nil {
		return nil, "", err
	}
	addr := fmt.Sprintf("%s:%d", params.BindAddress, params.BindPort)
	return sshCfg, addr, nil
}
