package config

import "testing"

func TestServerParameters_Validate(t *testing.T) {
	tests := []struct {
		name    string
		change  func(p *ServerParameters)
		wantErr string
	}{
		{name: "valid with a password", change: func(p *ServerParameters) {}},
		{name: "valid with authorized keys only", change: func(p *ServerParameters) { p.Password = ""; p.AuthorizedKeysPath = "authorized_keys" }},
		{name: "only an ECDSA host key", change: func(p *ServerParameters) { p.PrivateRsaPath = ""; p.PrivateEcdsaPath = "host_ecdsa" }},
		{name: "only an Ed25519 host key", change: func(p *ServerParameters) { p.PrivateRsaPath = ""; p.PrivateEd25519Path = "host_ed25519" }},
		{name: "single-port range", change: func(p *ServerParameters) { p.PortRangeStart = 3000; p.PortRangeEnd = 3000 }},
		{name: "missing bind address", change: func(p *ServerParameters) { p.BindAddress = "" }, wantErr: "bind address is required"},
		{name: "bind port 0", change: func(p *ServerParameters) { p.BindPort = 0 }, wantErr: "bind port must be between 1 and 65535"},
		{name: "negative range start", change: func(p *ServerParameters) { p.PortRangeStart = -1 }, wantErr: "port_range_start must be between 0 and 65535"},
		{name: "range start too high", change: func(p *ServerParameters) { p.PortRangeStart = 70000; p.PortRangeEnd = 80000 }, wantErr: "port_range_start must be between 0 and 65535"},
		{name: "range end before range start", change: func(p *ServerParameters) { p.PortRangeStart = 3000; p.PortRangeEnd = 2000 }, wantErr: "port_range_end must be between port_range_start and 65535"},
		{name: "range end too high", change: func(p *ServerParameters) { p.PortRangeEnd = 70000 }, wantErr: "port_range_end must be between port_range_start and 65535"},
		{name: "missing username", change: func(p *ServerParameters) { p.Username = "" }, wantErr: "username must be set for SSH server"},
		{name: "no password and no authorized keys", change: func(p *ServerParameters) { p.Password = "" }, wantErr: "password or authorized_keys must be set for SSH server"},
		{name: "no host key", change: func(p *ServerParameters) { p.PrivateRsaPath = "" }, wantErr: "at least one host key path must be provided"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := ServerParameters{
				BindAddress:    "0.0.0.0",
				BindPort:       2022,
				PortRangeStart: 1000,
				PortRangeEnd:   2000,
				Username:       "user",
				Password:       "pass",
				PrivateRsaPath: "host_rsa",
			}
			tc.change(&params)

			err := params.Validate()

			if tc.wantErr == "" && err != nil {
				t.Errorf("Validate() = %v, want no error", err)
			}
			if tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr) {
				t.Errorf("Validate() = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
