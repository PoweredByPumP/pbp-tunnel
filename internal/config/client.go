package config

import (
	"fmt"
	"strings"
)

// ClientParameters holds configuration for the SSH client
// Fields may be set via JSON file or environment variables
// Endpoint and EndpointPort specify the SSH server to connect to
type ClientParameters struct {
	Endpoint       string      `json:"endpoint"`
	EndpointPort   int         `json:"port"`
	Username       string      `json:"username"`
	Password       string      `json:"password"`
	PrivateKeyPath string      `json:"identity"`
	HostKeyPath    string      `json:"host_key"`
	LocalHost      string      `json:"local_host"`
	LocalPort      int         `json:"local_port"`
	RemoteHost     string      `json:"remote_host"`
	RemotePort     int         `json:"remote_port"`
	Protocol       string      `json:"protocol"`
	HostKeyLevel   int         `json:"host_key_level"`
	AllowedIPs     StringArray `json:"allowed_ips"`
}

// Validate ensures the ClientParameters contains all required fields and valid values
func (cp *ClientParameters) Validate() error {
	if cp.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if cp.EndpointPort <= 0 || cp.EndpointPort > 65535 {
		return fmt.Errorf("endpoint port must be between 1 and 65535")
	}
	if cp.Username == "" {
		return fmt.Errorf("username is required")
	}
	if cp.PrivateKeyPath == "" && cp.Password == "" {
		return fmt.Errorf("either private_key or password must be set")
	}
	if cp.LocalHost == "" {
		return fmt.Errorf("local_host is required")
	}
	if cp.LocalPort <= 0 || cp.LocalPort > 65535 {
		return fmt.Errorf("local_port must be between 1 and 65535")
	}
	if cp.RemoteHost == "" {
		return fmt.Errorf("remote_host is required")
	}
	if cp.RemotePort < 0 || cp.RemotePort > 65535 {
		return fmt.Errorf("remote_port must be between 0 and 65535")
	}
	if cp.Protocol == "" {
		cp.Protocol = CpDefaultProtocol
	}
	cp.Protocol = strings.ToLower(cp.Protocol)
	if cp.Protocol != "tcp" && cp.Protocol != "udp" {
		return fmt.Errorf("protocol must be either tcp or udp")
	}
	return nil
}
