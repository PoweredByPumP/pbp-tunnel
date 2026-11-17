package config

import (
	"fmt"
	"strings"
)

// Values of ClientParameters.HostKeyLevel, which decides how the client checks
// the identity of the server against the known_hosts file in HostKeyPath.
const (
	HostKeyNotChecked    = 0
	HostKeyWarnIfUnknown = 1
	HostKeyMustBeKnown   = 2
)

type ClientParameters struct {
	Endpoint       string `json:"endpoint"`
	EndpointPort   int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	PrivateKeyPath string `json:"identity"`
	// HostKeyPath is a known_hosts file used to check the server identity.
	HostKeyPath string `json:"host_key"`
	LocalHost   string `json:"local_host"`
	LocalPort   int    `json:"local_port"`
	// RemotePort is the port to expose on the server; 0 lets the server choose.
	RemotePort   int    `json:"remote_port"`
	Protocol     string `json:"protocol"`
	HostKeyLevel int    `json:"host_key_level"`
	// AllowedIPs restricts who may connect to the exposed port; empty allows everyone.
	AllowedIPs []string `json:"allowed_ips"`
}

func DefaultClientParameters() *ClientParameters {
	return &ClientParameters{
		EndpointPort: DefaultPort,
		LocalHost:    "localhost",
		LocalPort:    80,
		Protocol:     "tcp",
		HostKeyLevel: HostKeyMustBeKnown,
	}
}

func (p *ClientParameters) Validate() error {
	if p.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if p.EndpointPort <= 0 || p.EndpointPort > 65535 {
		return fmt.Errorf("endpoint port must be between 1 and 65535")
	}
	if p.Username == "" {
		return fmt.Errorf("username is required")
	}
	if p.PrivateKeyPath == "" && p.Password == "" {
		return fmt.Errorf("either private_key or password must be set")
	}
	if p.LocalHost == "" {
		return fmt.Errorf("local_host is required")
	}
	if p.LocalPort <= 0 || p.LocalPort > 65535 {
		return fmt.Errorf("local_port must be between 1 and 65535")
	}
	if p.RemotePort < 0 || p.RemotePort > 65535 {
		return fmt.Errorf("remote_port must be between 0 and 65535")
	}
	if protocol := strings.ToLower(p.Protocol); protocol != "tcp" && protocol != "udp" {
		return fmt.Errorf("protocol must be either tcp or udp")
	}
	if p.HostKeyLevel < HostKeyNotChecked || p.HostKeyLevel > HostKeyMustBeKnown {
		return fmt.Errorf("host_key_level must be 0 (no check), 1 (warn) or 2 (strict)")
	}
	if p.HostKeyLevel == HostKeyMustBeKnown && p.HostKeyPath == "" {
		return fmt.Errorf("host_key_level 2 checks the server against a known_hosts file: " +
			"set host_key to that file, or host_key_level to 0 to connect without checking")
	}
	return nil
}
