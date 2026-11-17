package config

import (
	"fmt"
	"strings"
)

const DefaultEndpointPort int = 52135

const (
	CpKeyEndpoint       string = "endpoint"
	CpKeyEndpointPort   string = "port"
	CpKeyUsername       string = "username"
	CpKeyPassword       string = "password"
	CpKeyPrivateKeyPath string = "identity"
	CpKeyHostKeyPath    string = "host-key"
	CpKeyLocalHost      string = "local-host"
	CpKeyLocalPort      string = "local-port"
	CpKeyRemoteHost     string = "remote-host"
	CpKeyRemotePort     string = "remote-port"
	CpKeyProtocol       string = "protocol"
	CpKeyHostKeyLevel   string = "host-key-level"
	CpKeyAllowedIPs     string = "allowed-ips"

	CpDefaultEndpoint       string = ""
	CpDefaultEndpointPort          = DefaultEndpointPort
	CpDefaultUsername       string = ""
	CpDefaultPassword       string = ""
	CpDefaultPrivateKeyPath string = ""
	CpDefaultHostKeyPath    string = ""
	CpDefaultLocalHost      string = "localhost"
	CpDefaultLocalPort      int    = 80
	CpDefaultRemoteHost     string = "localhost"
	CpDefaultRemotePort     int    = 0
	CpDefaultProtocol       string = "tcp"
	CpDefaultHostKeyLevel   int    = 2

	SpKeyBindAddress        string = "bind"
	SpKeyBindPort           string = "port"
	SpKeyPortRangeStart     string = "port-range-start"
	SpKeyPortRangeEnd       string = "port-range-end"
	SpKeyUsername           string = "username"
	SpKeyPassword           string = "password"
	SpKeyPrivateRsaPath     string = "private-rsa-path"
	SpKeyPrivateEcdsaPath   string = "private-ecdsa-path"
	SpKeyPrivateEd25519Path string = "private-ed25519-path"
	SpKeyAuthorizedKeysPath string = "authorized-keys-path"
	SpKeyAllowedIPS         string = "allowed-ips"

	SpDefaultBindAddress    string = "0.0.0.0"
	SpDefaultBindPort       int    = DefaultEndpointPort
	SpDefaultPortRangeStart int    = 49152
	SpDefaultPortRangeEnd   int    = 65535
	SpDefaultUsername       string = ""
	SpDefaultPassword       string = ""
	SpDefaultPrivateRsa     string = "id_rsa"
	SpDefaultPrivateEcdsa   string = ""
	SpDefaultPrivateEd25519 string = ""
	SpDefaultAuthorizedKeys string = ""
)

// StringArray is a flag.Stringer implementation for multiple values
// used for JSON unmarshalling and environment parsing
// Represents a list of IPs allowed for forwarding
// (e.g., ["10.0.0.0/8", "192.168.1.1"])
type StringArray []string

func (s *StringArray) String() string {
	return strings.Join(*s, ",")
}

// Set accepts both "--allowed-ips a,b" and a repeated "--allowed-ips a --allowed-ips b".
func (s *StringArray) Set(value string) error {
	*s = append(*s, splitCommaSeparated(value)...)
	return nil
}

func splitCommaSeparated(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
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
