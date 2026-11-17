package server

import (
	"fmt"
	"log"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/hostkey"
)

func generateMissingHostKeys(params *config.ServerParameters) error {
	hostKeys := []struct {
		path      string
		algorithm hostkey.Algorithm
	}{
		{params.PrivateRsaPath, hostkey.RSA},
		{params.PrivateEcdsaPath, hostkey.ECDSA},
		{params.PrivateEd25519Path, hostkey.Ed25519},
	}
	for _, hostKey := range hostKeys {
		if hostKey.path == "" {
			continue
		}
		created, err := hostkey.EnsureExists(hostKey.path, hostKey.algorithm)
		if err != nil {
			return fmt.Errorf("generate %s host key at %s: %w", hostKey.algorithm, hostKey.path, err)
		}
		if created {
			log.Printf("[+] Generated %s host key at %s", hostKey.algorithm, hostKey.path)
		}
	}
	return nil
}
