package config

import (
	"golang.org/x/crypto/ssh"
	"net"
	"strings"
	"testing"
)

// dummyConn implements ssh.ConnMetadata for testing PasswordCallback
type dummyConn struct{ user string }

func (d *dummyConn) User() string                  { return d.user }
func (d *dummyConn) SessionID() []byte             { return nil }
func (d *dummyConn) ClientVersion() []byte         { return nil }
func (d *dummyConn) ServerVersion() []byte         { return nil }
func (d *dummyConn) RemoteAddr() net.Addr          { return nil }
func (d *dummyConn) LocalAddr() net.Addr           { return nil }
func (d *dummyConn) Permissions() *ssh.Permissions { return nil }

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

		{"invalid-remote-port-zero", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 0,
		}, true, "remote_port must be between 1 and 65535"},

		{"invalid-remote-port-high", &ClientParameters{
			Endpoint: "example.com", EndpointPort: 22,
			Username: "user", Password: "pass",
			LocalHost: "localhost", LocalPort: 8080,
			RemoteHost: "remote-host", RemotePort: 70000,
		}, true, "remote_port must be between 1 and 65535"},
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
	tests := []struct {
		name    string
		sp      *ServerParameters
		wantErr bool
		errMsg  string
	}{
		{"valid-parameters", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "/path/key"}, false, ""},
		{"missing-bind-address", &ServerParameters{BindAddress: "", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "/path/key"}, true, "bind address is required"},
		{"invalid-bindport", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 0, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "/path/key"}, true, "bind port must be between 1 and 65535"},
		{"invalid-range-start", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: -1, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "/path/key"}, true, "port_range_start must be between 0 and 65535"},
		{"range-start-too-high", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 70000, PortRangeEnd: 80000, Username: "user", Password: "pass", PrivateRsaPath: "/path/key"}, true, "port_range_start must be between 0 and 65535"},
		{"invalid-range-end", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 3000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "/path/key"}, true, "port_range_end must be between port_range_start and 65535"},
		{"range-end-too-high", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 3000, PortRangeEnd: 70000, Username: "user", Password: "pass", PrivateRsaPath: "/path/key"}, true, "port_range_end must be between port_range_start and 65535"},
		{"missing-auth", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "", Password: "", PrivateRsaPath: "/path/key"}, true, "username or password must be set for SSH server"},
		{"missing-key", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: ""}, true, "at least one host key path must be provided"},
		{"only-username", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "", PrivateRsaPath: "/path/key"}, false, ""},
		{"only-password", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "", Password: "pass", PrivateRsaPath: "/path/key"}, false, ""},
		{"only-ecdsa-key", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "", PrivateEcdsaPath: "/path/ecdsa"}, false, ""},
		{"only-ed25519-key", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: "", PrivateEd25519Path: "/path/ed25519"}, false, ""},
		{"zero-port-range", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 3000, PortRangeEnd: 3000, Username: "user", Password: "pass", PrivateRsaPath: "/path/key"}, false, ""},
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

func TestGetClientConfig_PasswordAuth(t *testing.T) {
	params := &ClientParameters{
		Username:     "testuser",
		Password:     "secret",
		Endpoint:     "example.com",
		EndpointPort: 2222,
	}
	sshCfg, addr, err := GetClientConfig(params)
	if err != nil {
		t.Fatalf("GetClientConfig returned error: %v", err)
	}
	if addr != "example.com:2222" {
		t.Errorf("addr = %q; want %q", addr, "example.com:2222")
	}
	if sshCfg.User != "testuser" {
		t.Errorf("sshCfg.User = %q; want %q", sshCfg.User, "testuser")
	}
	if len(sshCfg.Auth) != 1 {
		t.Errorf("len(sshCfg.Auth) = %d; want %d", len(sshCfg.Auth), 1)
	}
	// HostKeyCallback should allow any key (insecure)
	if err := sshCfg.HostKeyCallback("host", nil, nil); err != nil {
		t.Errorf("HostKeyCallback returned error: %v", err)
	}
}

func TestGetClientConfig_PrivateKeyPathError(t *testing.T) {
	params := &ClientParameters{
		Username:       "testuser",
		PrivateKeyPath: "/nonexistent/key",
		Endpoint:       "example.com",
		EndpointPort:   22,
	}
	_, _, err := GetClientConfig(params)
	if err == nil {
		t.Fatal("expected error for missing private key, got nil")
	}
	if !strings.Contains(err.Error(), "read private key") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "read private key")
	}
}

func TestGetServerConfig_PasswordCallback(t *testing.T) {
	params := &ServerParameters{
		BindAddress:    "0.0.0.0",
		BindPort:       2022,
		Username:       "admin",
		Password:       "passwd",
		PrivateRsaPath: "", // no host key file
	}
	sshCfg, addr, err := GetServerConfig(params)
	if err != nil {
		t.Fatalf("GetServerConfig returned error: %v", err)
	}
	if addr != "0.0.0.0:2022" {
		t.Errorf("addr = %q; want %q", addr, "0.0.0.0:2022")
	}
	// PasswordCallback should accept correct and reject incorrect password
	cb := sshCfg.PasswordCallback
	if cb == nil {
		t.Fatal("expected PasswordCallback to be set, got nil")
	}
	// Correct
	if perms, err := cb(&dummyConn{user: "admin"}, []byte("passwd")); err != nil || perms != nil {
		t.Errorf("PasswordCallback ok = (%v, %v); want (nil, nil)", perms, err)
	}
	// Incorrect
	if _, err := cb(&dummyConn{user: "admin"}, []byte("wrong")); err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}

func TestGetServerConfig_NoAuth(t *testing.T) {
	params := &ServerParameters{
		BindAddress:    "127.0.0.1",
		BindPort:       8022,
		Username:       "",
		Password:       "",
		PrivateRsaPath: "", // no host key
	}
	sshCfg, _, err := GetServerConfig(params)
	if err != nil {
		t.Fatalf("GetServerConfig returned error: %v", err)
	}
	// When no password, PasswordCallback should be nil (no auth)
	if sshCfg.PasswordCallback != nil {
		t.Errorf("expected PasswordCallback to be nil, got non-nil")
	}
}
