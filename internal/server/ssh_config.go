package server

import (
	"fmt"
	"log"
	"os"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"golang.org/x/crypto/ssh"
)

// sshServerConfig accepts the configured user with its password, a key from the
// authorized keys file, or both.
func sshServerConfig(params *config.ServerParameters) (*ssh.ServerConfig, error) {
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
