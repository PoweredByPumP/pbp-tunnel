package server

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
)

// sshServerConfig accepts the configured user with its password, a key from the
// authorized keys file, or both.
func sshServerConfig(params *config.ServerParameters) (*ssh.ServerConfig, error) {
	sshConfig := &ssh.ServerConfig{
		MaxAuthTries:  2,
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
		AuthLogCallback: func(conn ssh.ConnMetadata, method string, err error) {
			log.Printf("[*] User %s tried to authenticate with method %s. Error (if any): %v", conn.User(), method, err)
		},
	}

	for _, path := range []string{params.PrivateRsaPath, params.PrivateEcdsaPath, params.PrivateEd25519Path} {
		if path == "" {
			continue
		}
		hostKey, err := readHostKey(path)
		if err != nil {
			return nil, err
		}
		sshConfig.AddHostKey(hostKey)
	}

	if params.Password != "" {
		sshConfig.PasswordCallback = func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == params.Username && string(password) == params.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", conn.User())
		}
	}

	if params.AuthorizedKeysPath != "" {
		authorizedKeys, err := readAuthorizedKeys(params.AuthorizedKeysPath)
		if err != nil {
			return nil, err
		}
		sshConfig.PublicKeyCallback = func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() == params.Username && authorizedKeys[string(key.Marshal())] {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("public key rejected for %q", conn.User())
		}
	}

	return sshConfig, nil
}

func readHostKey(path string) (ssh.Signer, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read host key: %w", err)
	}
	hostKey, err := ssh.ParsePrivateKey(content)
	if err != nil {
		return nil, fmt.Errorf("parse host key %s: %w", path, err)
	}
	return hostKey, nil
}

// readAuthorizedKeys returns the keys of an OpenSSH authorized_keys file, indexed
// by their wire encoding. Blank lines and # comments are skipped.
func readAuthorizedKeys(path string) (map[string]bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read authorized keys: %w", err)
	}
	authorizedKeys := make(map[string]bool)
	for index, line := range bytes.Split(content, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		key, _, _, _, err := ssh.ParseAuthorizedKey(line)
		if err != nil {
			return nil, fmt.Errorf("parse authorized keys %s, line %d: %w", path, index+1, err)
		}
		authorizedKeys[string(key.Marshal())] = true
	}
	return authorizedKeys, nil
}
