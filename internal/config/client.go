package config

import (
	"fmt"
	"strings"
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
	RemotePort int    `json:"remote_port"`
	Protocol   string `json:"protocol"`
	// HostKeyLevel is 0 for no check, 1 to warn and 2 to refuse an unknown server.
	HostKeyLevel int `json:"host_key_level"`
	// AllowedIPs restricts who may connect to the exposed port; empty allows everyone.
	AllowedIPs []string `json:"allowed_ips"`
}

func DefaultClientParameters() *ClientParameters {
	return &ClientParameters{
		EndpointPort: DefaultPort,
		LocalHost:    "localhost",
		LocalPort:    80,
		Protocol:     "tcp",
		HostKeyLevel: 2,
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
	return nil
}
