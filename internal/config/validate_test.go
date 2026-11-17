package config

import (
	"path/filepath"
	"testing"
)

func TestClientParameters_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cp      *ClientParameters
		wantErr bool
		errMsg  string
	}{
		{"valid-parameters", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, false, ""},

		{"valid-key-auth", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", PrivateKeyPath: "/path/to/key",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, false, ""},

		// Missing endpoint tests
		{"missing-endpoint", &ClientParameters{
			EndpointPort: 22, Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, true, "endpoint is required"},

		{"invalid-endpoint-port-zero", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 0,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, true, "endpoint port must be between 1 and 65535"},

		{"invalid-endpoint-port-high", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 70000,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, true, "endpoint port must be between 1 and 65535"},

		// Auth tests
		{"missing-username", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Password:  "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, true, "username is required"},

		{"missing-auth", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username:  "user",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, true, "either private_key or password must be set"},

		// Local connection tests
		{"missing-local-host", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalPort:  8080,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, true, "local_host is required"},

		{"invalid-local-port-zero", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 0,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, true, "local_port must be between 1 and 65535"},

		{"invalid-local-port-high", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 70000,
			RemoteHost: "remote-host", RemotePort: 9090,
		}, true, "local_port must be between 1 and 65535"},

		// Remote connection tests
		{"missing-remote-host", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemotePort: 9090,
		}, true, "remote_host is required"},

		{"invalid-remote-port-minus-one", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: -1,
		}, true, "remote_port must be between 0 and 65535"},

		{"invalid-remote-port-high", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 70000,
		}, true, "remote_port must be between 0 and 65535"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cp.Validate()
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error %q, got nil", tc.errMsg)
				} else if err.Error() != tc.errMsg {
					t.Errorf("expected error %q, got %q", tc.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestServerParameters_Validate(t *testing.T) {
	tempDir := makeTempDir(t)

	tests := []struct {
		name    string
		sp      *ServerParameters
		wantErr bool
		errMsg  string
	}{
		{"valid-parameters", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, false, ""},
		{"missing-bind-address", &ServerParameters{BindAddress: "", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "bind address is required"},
		{"invalid-bindport", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 0, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "bind port must be between 1 and 65535"},
		{"invalid-range-start", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: -1, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "port_range_start must be between 0 and 65535"},
		{"range-start-too-high", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 70000, PortRangeEnd: 80000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "port_range_start must be between 0 and 65535"},
		{"invalid-range-end", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 3000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "port_range_end must be between port_range_start and 65535"},
		{"range-end-too-high", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 3000, PortRangeEnd: 70000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "port_range_end must be between port_range_start and 65535"},
		{"missing-username", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "username must be set for SSH server"},
		{"missing-password-and-authorized-keys", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "password or authorized_keys must be set for SSH server"},
		{"missing-key", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: ""}, true, "at least one host key path must be provided"},
		{"only-username", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "password or authorized_keys must be set for SSH server"},
		{"only-password", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "username must be set for SSH server"},
		{"only-ecdsa-key", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "", PrivateEcdsaPath: filepath.Join(tempDir, "/id_ecdsa")}, false, ""},
		{"only-ed25519-key", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "", PrivateEd25519Path: filepath.Join(tempDir, "/id_ed25519")}, false, ""},
		{"zero-port-range", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 3000, PortRangeEnd: 3000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sp.Validate()
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error %q, got nil", tc.errMsg)
				} else if err.Error() != tc.errMsg {
					t.Errorf("expected error %q, got %q", tc.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
