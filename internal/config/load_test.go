package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestLoadClientParameters_DefaultsWithoutAnySource(t *testing.T) {
	useConfigFile(t, "")

	params, err := LoadClientParameters(newSilentFlagSet(), nil)

	if err != nil {
		t.Fatalf("LoadClientParameters: %v", err)
	}
	if !reflect.DeepEqual(params, DefaultClientParameters()) {
		t.Errorf("params = %+v, want the defaults %+v", params, DefaultClientParameters())
	}
}

func TestLoadClientParameters_EachSourceOverridesThePreviousOne(t *testing.T) {
	useConfigFile(t, `{"client": {"endpoint": "from-file", "port": 1111, "username": "from-file", "password": "from-file"}}`)
	t.Setenv("PBP_TUNNEL_PORT", "2222")
	t.Setenv("PBP_TUNNEL_USERNAME", "from-env")

	params, err := LoadClientParameters(newSilentFlagSet(), []string{"--username", "from-flag"})

	if err != nil {
		t.Fatalf("LoadClientParameters: %v", err)
	}
	want := DefaultClientParameters()
	want.Endpoint = "from-file"
	want.Password = "from-file"
	want.EndpointPort = 2222
	want.Username = "from-flag"
	if !reflect.DeepEqual(params, want) {
		t.Errorf("params = %+v, want %+v", params, want)
	}
}

func TestLoadClientParameters_AcceptsConfigFilesWithTheRemovedRemoteHost(t *testing.T) {
	useConfigFile(t, `{"client": {"endpoint": "tunnel.example.com", "remote_host": "localhost"}}`)

	params, err := LoadClientParameters(newSilentFlagSet(), nil)

	if err != nil || params.Endpoint != "tunnel.example.com" {
		t.Errorf("LoadClientParameters = %+v, %v; want the file to load", params, err)
	}
}

func TestLoadServerParameters_EachSourceOverridesThePreviousOne(t *testing.T) {
	useConfigFile(t, `{"server": {"bind": "10.0.0.1", "port_range_start": 1000, "username": "from-file"}}`)
	t.Setenv("PBP_TUNNEL_PORT_RANGE_START", "2000")

	params, err := LoadServerParameters(newSilentFlagSet(), []string{"--password", "from-flag"})

	if err != nil {
		t.Fatalf("LoadServerParameters: %v", err)
	}
	want := DefaultServerParameters()
	want.BindAddress = "10.0.0.1"
	want.Username = "from-file"
	want.PortRangeStart = 2000
	want.Password = "from-flag"
	if !reflect.DeepEqual(params, want) {
		t.Errorf("params = %+v, want %+v", params, want)
	}
}

func TestLoadServerParameters_AllowedIPsAreReplacedNotMerged(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want []string
	}{
		{name: "file only", want: []string{"10.0.0.1"}},
		{name: "environment replaces file", env: "10.0.0.2, 10.0.0.3", want: []string{"10.0.0.2", "10.0.0.3"}},
		{
			name: "flags replace environment",
			env:  "10.0.0.2",
			args: []string{"--allowed-ips", "10.0.0.4", "--allowed-ips", "10.0.0.5"},
			want: []string{"10.0.0.4", "10.0.0.5"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			useConfigFile(t, `{"server": {"allowed_ips": ["10.0.0.1"]}}`)
			t.Setenv("PBP_TUNNEL_ALLOWED_IPS", tc.env)

			params, err := LoadServerParameters(newSilentFlagSet(), tc.args)

			if err != nil {
				t.Fatalf("LoadServerParameters: %v", err)
			}
			if !slices.Equal(params.AllowedIPs, tc.want) {
				t.Errorf("AllowedIPs = %q, want %q", params.AllowedIPs, tc.want)
			}
		})
	}
}

func TestLoadClientParameters_ReportsInvalidSources(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		wantInErr string
	}{
		{
			name:      "number variable that is not a number",
			setup:     func(t *testing.T) { useConfigFile(t, ""); t.Setenv("PBP_TUNNEL_LOCAL_PORT", "eighty") },
			wantInErr: "PBP_TUNNEL_LOCAL_PORT",
		},
		{
			name:      "config file named by PBP_TUNNEL_CONFIG is missing",
			setup:     func(t *testing.T) { t.Setenv("PBP_TUNNEL_CONFIG", filepath.Join(t.TempDir(), "missing.json")) },
			wantInErr: "read config file",
		},
		{
			name:      "config file is not valid JSON",
			setup:     func(t *testing.T) { useConfigFile(t, `{"client": `) },
			wantInErr: "parse config file",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)

			_, err := LoadClientParameters(newSilentFlagSet(), nil)

			if err == nil || !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("LoadClientParameters error = %v, want it to mention %q", err, tc.wantInErr)
			}
		})
	}
}

func TestReadMode(t *testing.T) {
	tests := []struct {
		name       string
		fileConfig string
		envType    string
		want       Mode
	}{
		{name: "from the config file", fileConfig: `{"type": "server"}`, want: ModeServer},
		{name: "environment wins over the file", fileConfig: `{"type": "server"}`, envType: "client", want: ModeClient},
		{name: "no source", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			useConfigFile(t, tc.fileConfig)
			t.Setenv("PBP_TUNNEL_TYPE", tc.envType)

			mode, err := ReadMode()

			if err != nil || mode != tc.want {
				t.Errorf("ReadMode() = %q, %v; want %q", mode, err, tc.want)
			}
		})
	}
}

// useConfigFile points PBP_TUNNEL_CONFIG to a file holding content, or to no file
// at all when content is empty. It also clears the variables the tests rely on.
func useConfigFile(t *testing.T, content string) {
	t.Helper()
	for _, name := range []string{"PBP_TUNNEL_TYPE", "PBP_TUNNEL_PORT", "PBP_TUNNEL_USERNAME", "PBP_TUNNEL_ALLOWED_IPS"} {
		t.Setenv(name, "")
	}
	if content == "" {
		t.Setenv("PBP_TUNNEL_CONFIG", "")
		t.Chdir(t.TempDir())
		return
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PBP_TUNNEL_CONFIG", path)
}
