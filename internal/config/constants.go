package config

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
