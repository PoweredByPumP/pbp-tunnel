package config

import "testing"

func TestClientParameters_Validate(t *testing.T) {
	tests := []struct {
		name    string
		change  func(p *ClientParameters)
		wantErr string
	}{
		{name: "valid with a password", change: func(p *ClientParameters) {}},
		{name: "valid with a private key only", change: func(p *ClientParameters) { p.Password = ""; p.PrivateKeyPath = "id_ed25519" }},
		{name: "udp in upper case", change: func(p *ClientParameters) { p.Protocol = "UDP" }},
		{name: "remote port chosen by the server", change: func(p *ClientParameters) { p.RemotePort = 0 }},
		{name: "missing endpoint", change: func(p *ClientParameters) { p.Endpoint = "" }, wantErr: "endpoint is required"},
		{name: "endpoint port 0", change: func(p *ClientParameters) { p.EndpointPort = 0 }, wantErr: "endpoint port must be between 1 and 65535"},
		{name: "endpoint port too high", change: func(p *ClientParameters) { p.EndpointPort = 70000 }, wantErr: "endpoint port must be between 1 and 65535"},
		{name: "missing username", change: func(p *ClientParameters) { p.Username = "" }, wantErr: "username is required"},
		{name: "no password and no private key", change: func(p *ClientParameters) { p.Password = "" }, wantErr: "either private_key or password must be set"},
		{name: "missing local host", change: func(p *ClientParameters) { p.LocalHost = "" }, wantErr: "local_host is required"},
		{name: "local port 0", change: func(p *ClientParameters) { p.LocalPort = 0 }, wantErr: "local_port must be between 1 and 65535"},
		{name: "local port too high", change: func(p *ClientParameters) { p.LocalPort = 70000 }, wantErr: "local_port must be between 1 and 65535"},
		{name: "negative remote port", change: func(p *ClientParameters) { p.RemotePort = -1 }, wantErr: "remote_port must be between 0 and 65535"},
		{name: "remote port too high", change: func(p *ClientParameters) { p.RemotePort = 70000 }, wantErr: "remote_port must be between 0 and 65535"},
		{name: "unknown protocol", change: func(p *ClientParameters) { p.Protocol = "sctp" }, wantErr: "protocol must be either tcp or udp"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := ClientParameters{
				Endpoint:     "example.com",
				EndpointPort: 22,
				Username:     "user",
				Password:     "pass",
				LocalHost:    "localhost",
				LocalPort:    8080,
				RemotePort:   9090,
				Protocol:     "tcp",
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
