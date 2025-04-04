package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
	"os"
	"time"
)

func GetClientConfig(cp ClientParameters) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod
	var hostKeyCallback ssh.HostKeyCallback

	if cp.PrivateKeyPath != "" {
		privateKey, err := os.ReadFile(cp.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if cp.HostKeyPath != "" && cp.HostKeyLevel > 0 {
		hostKey, err := cp.GetHostKey(cp.HostKeyPath)
		if err != nil {
			log.Fatalf("Failed to read host key file: %v", err)
		}

		hostKeyCallback = ssh.FixedHostKey(hostKey)
	} else {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	authMethods = append(authMethods, ssh.Password(cp.Password))

	return &ssh.ClientConfig{
		User:            cp.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}, nil
}

func GetServerConfig(sp ServerParameters) (*ssh.ServerConfig, error) {
	authorizedKeysBytes, err := sp.GetAuthorizedKeysBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get authorized keys bytes: %v", err)
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

	sshConfig := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == sp.Username && string(password) == sp.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("password authentication failed for user %q", conn.User())
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			if authorizedKeysMap[string(pubKey.Marshal())] {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("unknown public key for user %q", conn.User())
		},
		MaxAuthTries: 2,
		AuthLogCallback: func(conn ssh.ConnMetadata, method string, err error) {
			log.Printf("User %s tried to authenticate with method %s. Error (if any): %v", conn.User(), method, err)
		},
		ServerVersion: "SSH-2.0",
		Config: ssh.Config{
			Ciphers: []string{
				"aes128-ctr", "aes192-ctr", "aes256-ctr",
				"aes128-gcm@openssh.com", "aes256-gcm@openssh.com",
			},
			KeyExchanges: []string{
				"curve25519-sha256", "curve25519-sha256@libssh.org",
				"diffie-hellman-group14-sha256",
			},
		},
	}

	if sp.PrivateKeyPath != "" {
		privateKeyBytes, err := sp.GetPrivateKeyBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to get private key bytes: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(privateKeyBytes)
		if err != nil {
			log.Fatalf("An error occurred while parsing private key: %v", err)
		}

		sshConfig.AddHostKey(signer)
	}

	return sshConfig, nil
}
