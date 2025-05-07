package config

import (
	"path/filepath"
	"testing"
)

func TestStringArraySetAndString(t *testing.T) {
	var sa StringArray
	if err := sa.Set("ip1"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := sa.Set("ip2"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	expected := "ip1,ip2"
	if sa.String() != expected {
		t.Errorf("String() = %q; want %q", sa.String(), expected)
	}
}

func TestClientParametersValidate(t *testing.T) {
	tests := []struct {
		name    string
		cp      *ClientParameters
		wantErr bool
		errMsg  string
	}{
		{"valid-password", &ClientParameters{
			Endpoint:     "example.com",
			EndpointPort: 22,
			Username:     "user",
			Password:     "pass",
			LocalHost:    "localhost",
			LocalPort:    8080,
			RemoteHost:   "remote",
			RemotePort:   9090,
		}, false, ""},
		{"missing-endpoint", &ClientParameters{
			Endpoint:     "",
			EndpointPort: 22,
			Username:     "user",
			Password:     "pass",
			LocalHost:    "localhost",
			LocalPort:    8080,
			RemoteHost:   "remote",
			RemotePort:   9090,
		}, true, "endpoint is required"},
		{"invalid-port", &ClientParameters{
			Endpoint:     "example.com",
			EndpointPort: 0,
			Username:     "user",
			Password:     "pass",
			LocalHost:    "localhost",
			LocalPort:    8080,
			RemoteHost:   "remote",
			RemotePort:   9090,
		}, true, "endpoint port must be between 1 and 65535"},
		{"missing-username", &ClientParameters{
			Endpoint:     "example.com",
			EndpointPort: 22,
			Username:     "",
			Password:     "pass",
			LocalHost:    "localhost",
			LocalPort:    8080,
			RemoteHost:   "remote",
			RemotePort:   9090,
		}, true, "username is required"},
		{"missing-auth", &ClientParameters{
			Endpoint:       "example.com",
			EndpointPort:   22,
			Username:       "user",
			Password:       "",
			PrivateKeyPath: "",
			LocalHost:      "localhost",
			LocalPort:      8080,
			RemoteHost:     "remote",
			RemotePort:     9090,
		}, true, "either private_key or password must be set"},
		{"missing-localhost", &ClientParameters{
			Endpoint:     "example.com",
			EndpointPort: 22,
			Username:     "user",
			Password:     "pass",
			LocalHost:    "",
			LocalPort:    8080,
			RemoteHost:   "remote",
			RemotePort:   9090,
		}, true, "local_host is required"},
		{"invalid-localport", &ClientParameters{
			Endpoint:     "example.com",
			EndpointPort: 22,
			Username:     "user",
			Password:     "pass",
			LocalHost:    "localhost",
			LocalPort:    0,
			RemoteHost:   "remote",
			RemotePort:   9090,
		}, true, "local_port must be between 1 and 65535"},
		{"missing-remotehost", &ClientParameters{
			Endpoint:     "example.com",
			EndpointPort: 22,
			Username:     "user",
			Password:     "pass",
			LocalHost:    "localhost",
			LocalPort:    8080,
			RemoteHost:   "",
			RemotePort:   9090,
		}, true, "remote_host is required"},
		{"invalid-remoteport", &ClientParameters{
			Endpoint:     "example.com",
			EndpointPort: 22,
			Username:     "user",
			Password:     "pass",
			LocalHost:    "localhost",
			LocalPort:    8080,
			RemoteHost:   "remote",
			RemotePort:   70000,
		}, true, "remote_port must be between 0 and 65535"},
	}
	for _, tc := range tests {
		err := tc.cp.Validate()
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error %q, got nil", tc.name, tc.errMsg)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%s: expected error %q, got %q", tc.name, tc.errMsg, err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("%s: expected no error, got %v", tc.name, err)
			}
		}
	}
}

func TestServerParametersValidate(t *testing.T) {
	tempDir := makeTempDir(t)

	tests := []struct {
		name    string
		sp      *ServerParameters
		wantErr bool
		errMsg  string
	}{
		{"valid", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, false, ""},
		{"missing-bindaddress", &ServerParameters{BindAddress: "", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "bind address is required"},
		{"invalid-bindport", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 0, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "bind port must be between 1 and 65535"},
		{"invalid-range-start", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: -1, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "port_range_start must be between 0 and 65535"},
		{"invalid-range-end", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 3000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "port_range_end must be between port_range_start and 65535"},
		{"missing-auth", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "", Password: "", PrivateRsaPath: filepath.Join(tempDir, "/id_rsa")}, true, "username or password must be set for SSH server"},
		{"missing-key", &ServerParameters{BindAddress: "0.0.0.0", BindPort: 2022, PortRangeStart: 1000, PortRangeEnd: 2000, Username: "user", Password: "pass", PrivateRsaPath: ""}, true, "at least one host key path must be provided"},
	}
	for _, tc := range tests {
		err := tc.sp.Validate()
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error %q, got nil", tc.name, tc.errMsg)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%s: expected error %q, got %q", tc.name, tc.errMsg, err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("%s: expected no error, got %v", tc.name, err)
			}
		}
	}
}
