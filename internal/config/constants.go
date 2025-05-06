package config

import (
	"fmt"
	"strings"
)

// StringArray is a flag.Stringer implementation for multiple values
// used for JSON unmarshalling and environment parsing
// Represents a list of IPs allowed for forwarding
// (e.g., ["10.0.0.0/8", "192.168.1.1"])
type StringArray []string

func (s *StringArray) String() string {
	return strings.Join(*s, ",")
}

func (s *StringArray) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// AppConfig is the root JSON structure for full config files
// Type indicates "client" or "server"
type AppConfig struct {
	Type   string            `json:"type"`
	Client *ClientParameters `json:"client,omitempty"`
	Server *ServerParameters `json:"server,omitempty"`
}

// ClientParameters holds configuration for the SSH client
// Fields may be set via JSON file or environment variables
// Endpoint and EndpointPort specify the SSH server to connect to
type ClientParameters struct {
	Endpoint       string      `json:"endpoint,omitempty"`
	EndpointPort   int         `json:"port,omitempty"`
	Username       string      `json:"username,omitempty"`
	Password       string      `json:"password,omitempty"`
	PrivateKeyPath string      `json:"identity,omitempty"`
	HostKeyPath    string      `json:"host_key,omitempty"`
	LocalHost      string      `json:"local_host,omitempty"`
	LocalPort      int         `json:"local_port,omitempty"`
	RemoteHost     string      `json:"remote_host,omitempty"`
	RemotePort     int         `json:"remote_port,omitempty"`
	HostKeyLevel   int         `json:"host_key_level,omitempty"`
	AllowedIPs     StringArray `json:"allowed_ips,omitempty"`
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
	if cp.RemotePort <= 0 || cp.RemotePort > 65535 {
		return fmt.Errorf("remote_port must be between 1 and 65535")
	}
	return nil
}

// ServerParameters holds configuration for the SSH server
// BindAddress and BindPort specify where forwarded connections land
// PortRangeStart/End restrict which ports may be assigned
// Multiple host key files may be provided
// AllowedIPs lists source IPs permitted to use the reverse tunnel
// AuthorizedKeysPath specifies the path to client public keys
// Username/Password define SSH login credentials
// PrivateRsaPath, PrivateEcdsaPath, PrivateEd25519Path are host key files

type ServerParameters struct {
	BindAddress        string      `json:"bind,omitempty"`
	BindPort           int         `json:"port,omitempty"`
	PortRangeStart     int         `json:"port_range_start,omitempty"`
	PortRangeEnd       int         `json:"port_range_end,omitempty"`
	Username           string      `json:"username,omitempty"`
	Password           string      `json:"password,omitempty"`
	PrivateRsaPath     string      `json:"private_rsa,omitempty"`
	PrivateEcdsaPath   string      `json:"private_ecdsa,omitempty"`
	PrivateEd25519Path string      `json:"private_ed25519,omitempty"`
	AuthorizedKeysPath string      `json:"authorized_keys,omitempty"`
	AllowedIPs         StringArray `json:"allowed_ips,omitempty"`
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
	if sp.Username == "" && sp.Password == "" {
		return fmt.Errorf("username or password must be set for SSH server")
	}
	if sp.PrivateRsaPath == "" && sp.PrivateEcdsaPath == "" && sp.PrivateEd25519Path == "" {
		return fmt.Errorf("at least one host key path must be provided")
	}
	return nil
}
