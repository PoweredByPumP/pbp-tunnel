package config

import (
	"fmt"
)

// ServerParameters holds configuration for the SSH server
// BindAddress and BindPort specify where forwarded connections land
// PortRangeStart/End restrict which ports may be assigned
// Multiple host key files may be provided
// AllowedIPs lists source IPs permitted to use the reverse tunnel
// AuthorizedKeysPath specifies the path to client public keys
// Username/Password define SSH login credentials
// PrivateRsaPath, PrivateEcdsaPath, PrivateEd25519Path are host key files

type ServerParameters struct {
	BindAddress        string      `json:"bind"`
	BindPort           int         `json:"port"`
	PortRangeStart     int         `json:"port_range_start"`
	PortRangeEnd       int         `json:"port_range_end"`
	Username           string      `json:"username"`
	Password           string      `json:"password"`
	PrivateRsaPath     string      `json:"private_rsa_path"`
	PrivateEcdsaPath   string      `json:"private_ecdsa_path"`
	PrivateEd25519Path string      `json:"private_ed25519_path"`
	AuthorizedKeysPath string      `json:"authorized_keys_path"`
	AllowedIPs         StringArray `json:"allowed_ips"`
}

// Validate ensures the ServerParameters contains all required fields and valid values
func (sp *ServerParameters) Validate() error {
	if sp.BindAddress == "" {
		return fmt.Errorf("bind address is required")
	}
	if sp.BindPort <= 0 || sp.BindPort > 65535 {
		return fmt.Errorf("bind port must be between 1 and 65535")
	}
	if sp.PortRangeStart < 0 || sp.PortRangeStart > 65535 {
		return fmt.Errorf("port_range_start must be between 0 and 65535")
	}
	if sp.PortRangeEnd < sp.PortRangeStart || sp.PortRangeEnd > 65535 {
		return fmt.Errorf("port_range_end must be between port_range_start and 65535")
	}
	if sp.Username == "" {
		return fmt.Errorf("username must be set for SSH server")
	}
	if sp.Password == "" && sp.AuthorizedKeysPath == "" {
		return fmt.Errorf("password or authorized_keys must be set for SSH server")
	}
	if sp.PrivateRsaPath == "" && sp.PrivateEcdsaPath == "" && sp.PrivateEd25519Path == "" {
		return fmt.Errorf("at least one host key path must be provided")
	}
	return nil
}
