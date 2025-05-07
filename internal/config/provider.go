package config

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// buildSSHClientConfig creates ssh.ClientConfig from ClientParameters\

func buildSSHClientConfig(params *ClientParameters) (*ssh.ClientConfig, error) {
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

	if params.Password != "" {
		serverCfg.PasswordCallback = func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == params.Username && string(pass) == params.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		}
	}

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

	if params.AuthorizedKeysPath != "" {
		authorizedKeysBytes, err := os.ReadFile(params.AuthorizedKeysPath)
		if err != nil {
			return nil, fmt.Errorf("read authorized keys: %w", err)
		}

		authorizedKeysMap := map[string]bool{}
		for len(authorizedKeysBytes) > 0 {
			pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
			if err != nil {
				log.Fatalf("An error occurred while parsing authorized keys: %v", err)
			}
			authorizedKeysMap[string(pubKey.Marshal())] = true
			authorizedKeysBytes = rest
		}

		serverCfg.PublicKeyCallback = func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if c.User() == params.Username && authorizedKeysMap[string(key.Marshal())] {
				return &ssh.Permissions{}, nil
			}

			return nil, fmt.Errorf("public key rejected for %q", c.User())
		}
	}

	serverCfg.MaxAuthTries = 2
	serverCfg.AuthLogCallback = func(conn ssh.ConnMetadata, method string, err error) {
		log.Printf("[*] User %s tried to authenticate with method %s. Error (if any): %v", conn.User(), method, err)
	}
	serverCfg.ServerVersion = "SSH-2.0"
	serverCfg.Config = ssh.Config{
		Ciphers: []string{
			"aes128-ctr", "aes192-ctr", "aes256-ctr",
			"aes128-gcm@openssh.com", "aes256-gcm@openssh.com",
		},
		KeyExchanges: []string{
			"curve25519-sha256", "curve25519-sha256@libssh.org",
			"diffie-hellman-group14-sha256",
		},
	}

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
