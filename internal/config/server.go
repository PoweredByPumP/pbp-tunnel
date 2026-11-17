package config

import "fmt"

type ServerParameters struct {
	// BindAddress is where the server listens for SSH clients and exposes their ports.
	BindAddress string `json:"bind"`
	BindPort    int    `json:"port"`
	// PortRangeStart and PortRangeEnd bound the ports a client may expose.
	PortRangeStart     int    `json:"port_range_start"`
	PortRangeEnd       int    `json:"port_range_end"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	PrivateRsaPath     string `json:"private_rsa_path"`
	PrivateEcdsaPath   string `json:"private_ecdsa_path"`
	PrivateEd25519Path string `json:"private_ed25519_path"`
	AuthorizedKeysPath string `json:"authorized_keys_path"`
	// AllowedIPs restricts which clients may open a tunnel; empty allows everyone.
	AllowedIPs []string `json:"allowed_ips"`
}

func DefaultServerParameters() *ServerParameters {
	return &ServerParameters{
		BindAddress:    "0.0.0.0",
		BindPort:       DefaultPort,
		PortRangeStart: 49152,
		PortRangeEnd:   65535,
		PrivateRsaPath: "id_rsa",
	}
}

func (p *ServerParameters) Validate() error {
	if p.BindAddress == "" {
		return fmt.Errorf("bind address is required")
	}
	if p.BindPort <= 0 || p.BindPort > 65535 {
		return fmt.Errorf("bind port must be between 1 and 65535")
	}
	if p.PortRangeStart < 0 || p.PortRangeStart > 65535 {
		return fmt.Errorf("port_range_start must be between 0 and 65535")
	}
	if p.PortRangeEnd < p.PortRangeStart || p.PortRangeEnd > 65535 {
		return fmt.Errorf("port_range_end must be between port_range_start and 65535")
	}
	if p.Username == "" {
		return fmt.Errorf("username must be set for SSH server")
	}
	if p.Password == "" && p.AuthorizedKeysPath == "" {
		return fmt.Errorf("password or authorized_keys must be set for SSH server")
	}
	if p.PrivateRsaPath == "" && p.PrivateEcdsaPath == "" && p.PrivateEd25519Path == "" {
		return fmt.Errorf("at least one host key path must be provided")
	}
	return nil
}
