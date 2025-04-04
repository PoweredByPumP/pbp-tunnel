package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"os"
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
	CpKeyHostKeyLevel   string = "host-key-level"

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
	CpDefaultHostKeyLevel   int    = 2

	SpKeyBindAddress    string = "bind"
	SpKeyBindPort       string = "port"
	SpKeyPortRangeStart string = "port-range-start"
	SpKeyPortRangeEnd   string = "port-range-end"
	SpKeyUsername       string = "username"
	SpKeyPassword       string = "password"
	SpKeyPrivateKey     string = "private-key"
	SpKeyAuthorizedKeys string = "authorized-keys"
	SpKeyAllowedIPS     string = "allowed-ips"

	SpDefaultBindAddress    string = "0.0.0.0"
	SpDefaultBindPort       int    = DefaultEndpointPort
	SpDefaultPortRangeStart int    = 49152
	SpDefaultPortRangeEnd   int    = 65535
	SpDefaultUsername       string = ""
	SpDefaultPassword       string = ""
	SpDefaultPrivateKey     string = ""
	SpDefaultAuthorizedKeys string = ""
)

type AppConfig struct {
	Type   string            `json:"type"`
	Client *ClientParameters `json:"client,omitempty"`
	Server *ServerParameters `json:"server,omitempty"`
}

type ClientParameters struct {
	Endpoint       string `json:"endpoint,omitempty"`
	EndpointPort   int    `json:"port,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	PrivateKeyPath string `json:"identity,omitempty"`
	HostKeyPath    string `json:"host_key,omitempty"`
	LocalHost      string `json:"local_host,omitempty"`
	LocalPort      int    `json:"local_port,omitempty"`
	RemoteHost     string `json:"remote_host,omitempty"`
	RemotePort     int    `json:"remote_port,omitempty"`
	HostKeyLevel   int    `json:"host_key_level,omitempty"`
}

type ServerParameters struct {
	BindAddress        string     `json:"bind,omitempty"`
	BindPort           int        `json:"port,omitempty"`
	PortRangeStart     int        `json:"port_range_start,omitempty"`
	PortRangeEnd       int        `json:"port_range_end,omitempty"`
	Username           string     `json:"username,omitempty"`
	Password           string     `json:"password,omitempty"`
	PrivateKeyPath     string     `json:"private_key,omitempty"`
	AuthorizedKeysPath string     `json:"authorized_keys,omitempty"`
	AllowedIPs         AllowedIPs `json:"allowed_ips,omitempty"`
}

type AllowedIPs []string

func (a *AllowedIPs) String() string {
	return fmt.Sprintf("%v", *a)
}

func (a *AllowedIPs) Set(value string) error {
	*a = append(*a, value)
	return nil
}

func (cp *ClientParameters) Validate() error {
	var missingParams []string

	if cp.Endpoint == "" {
		missingParams = append(missingParams, CpKeyEndpoint)
	}
	if cp.EndpointPort == 0 {
		missingParams = append(missingParams, CpKeyEndpointPort)
	}
	if cp.Username == "" {
		missingParams = append(missingParams, CpKeyUsername)
	}
	if cp.Password == "" && cp.PrivateKeyPath == "" {
		missingParams = append(missingParams, CpKeyPassword+" or "+CpKeyPrivateKeyPath)
	}
	if cp.LocalHost == "" {
		missingParams = append(missingParams, CpKeyLocalHost)
	}
	if cp.LocalPort == 0 {
		missingParams = append(missingParams, CpKeyLocalPort)
	}
	if cp.RemoteHost == "" {
		missingParams = append(missingParams, CpKeyRemoteHost)
	}

	if len(missingParams) > 0 {
		return fmt.Errorf("missing required parameters: %s", strings.Join(missingParams, ", "))
	}

	if cp.HostKeyPath == "" && cp.HostKeyLevel > 0 {
		fmt.Println("WARNING: host key level is set but host key path is not provided")

		if cp.HostKeyLevel > 1 {
			return fmt.Errorf("host key level is set to %d but host key path is not provided", cp.HostKeyLevel)
		}
	}

	return nil
}

func (cp *ClientParameters) GetFormattedAddress() string {
	return fmt.Sprintf("%s:%d", cp.LocalHost, cp.LocalPort)
}

func (cp *ClientParameters) GetHostKey(hostKeyPath string) (ssh.PublicKey, error) {
	hostKeyBytes, err := os.ReadFile(hostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read host key file: %v", err)
	}

	hostKey, _, _, _, err := ssh.ParseAuthorizedKey(hostKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host key: %v", err)
	}

	return hostKey, nil
}

func (sp *ServerParameters) Validate() error {
	var missingParams []string

	if sp.BindAddress == "" {
		missingParams = append(missingParams, SpKeyBindAddress)
	}
	if sp.BindPort == 0 {
		missingParams = append(missingParams, SpKeyBindPort)
	}
	if sp.PortRangeStart == 0 {
		missingParams = append(missingParams, SpKeyPortRangeStart)
	}
	if sp.PortRangeEnd == 0 {
		missingParams = append(missingParams, SpKeyPortRangeEnd)
	}
	if sp.Username == "" {
		missingParams = append(missingParams, SpKeyUsername)
	}
	if sp.Password == "" {
		missingParams = append(missingParams, SpKeyPassword)
	}
	if sp.PrivateKeyPath == "" {
		missingParams = append(missingParams, SpKeyPrivateKey)
	}

	if len(missingParams) > 0 {
		return fmt.Errorf("missing required parameters: %s", strings.Join(missingParams, ", "))
	}

	return nil
}

func (sp *ServerParameters) GetPrivateKeyBytes() ([]byte, error) {
	var privateKey []byte
	var err error

	if _, err := os.Stat(sp.PrivateKeyPath); os.IsNotExist(err) {
		privateKey, err = generatePrivateKey(sp.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("error generating private key: %v", err)
		}
	}

	privateKey, err = os.ReadFile(sp.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading private key file: %v", err)
	}

	return privateKey, nil
}

func (sp *ServerParameters) GetAuthorizedKeysBytes() ([]byte, error) {
	var authorizedKeys []byte
	var err error

	if sp.AuthorizedKeysPath != "" {
		authorizedKeys, err = os.ReadFile(sp.AuthorizedKeysPath)
		if err != nil {
			return nil, fmt.Errorf("error reading authorized keys file: %v", err)
		}
	}

	return authorizedKeys, nil
}
