package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvValue(t *testing.T) {
	os.Unsetenv("PBP_TUNNEL_SAMPLEKEY")
	if val := GetEnvValue("samplekey", "def"); val != "def" {
		t.Errorf("GetEnvValue without env = %q; want %q", val, "def")
	}
	os.Setenv("PBP_TUNNEL_SAMPLEKEY", "override")
	defer os.Unsetenv("PBP_TUNNEL_SAMPLEKEY")
	if val := GetEnvValue("samplekey", "def"); val != "override" {
		t.Errorf("GetEnvValue with env = %q; want %q", val, "override")
	}
}

func TestLoadEnvConfig(t *testing.T) {
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_ENDPOINT", "ex.com")
	t.Setenv("PBP_TUNNEL_PORT", "2222")
	t.Setenv("PBP_TUNNEL_LOCAL_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_BIND", "0.0.0.0")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_START", "1000")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_END", "2000")

	cfg := LoadEnvConfig()
	if cfg.Client == nil {
		t.Fatal("Expected client config, got nil")
	}
	if cfg.Client.Endpoint != "ex.com" {
		t.Errorf("Client.Endpoint = %q; want %q", cfg.Client.Endpoint, "ex.com")
	}
	if cfg.Client.EndpointPort != 2222 {
		t.Errorf("Client.EndpointPort = %d; want %d", cfg.Client.EndpointPort, 2222)
	}
	if cfg.Server == nil {
		t.Fatal("Expected server config, got nil")
	}
	if cfg.Server.BindAddress != "0.0.0.0" {
		t.Errorf("Server.BindAddress = %q; want %q", cfg.Server.BindAddress, "0.0.0.0")
	}
	if cfg.Server.PortRangeStart != 1000 {
		t.Errorf("Server.PortRangeStart = %d; want %d", cfg.Server.PortRangeStart, 1000)
	}
	if cfg.Server.PortRangeEnd != 2000 {
		t.Errorf("Server.PortRangeEnd = %d; want %d", cfg.Server.PortRangeEnd, 2000)
	}
}

func TestLoadConfig_FileMissing(t *testing.T) {
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_ENDPOINT", "envhost")
	os.Unsetenv("PBP_TUNNEL_CONFIG")

	cfg := LoadConfig()
	if cfg.Client == nil {
		t.Fatal("Expected client config from env, got nil")
	}
	if cfg.Client.Endpoint != "envhost" {
		t.Errorf("LoadConfig missing file: Endpoint = %q; want %q", cfg.Client.Endpoint, "envhost")
	}
}

func TestLoadConfig_ValidJSON(t *testing.T) {
	// Prepare a temp JSON config file
	tmpDir := makeTempDir(t)
	filePath := filepath.Join(tmpDir, "cfg.json")

	app := AppConfig{
		Type: "client",
		Client: &ClientParameters{
			Endpoint:     "jsonhost",
			EndpointPort: 3333,
		},
	}
	data, err := json.Marshal(app)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("PBP_TUNNEL_CONFIG", filePath)
	os.Unsetenv("PBP_TUNNEL_ENDPOINT")

	cfg := LoadConfig()
	if cfg.Type != "client" {
		t.Errorf("LoadConfig JSON: Type = %q; want %q", cfg.Type, "client")
	}
	if cfg.Client.Endpoint != "jsonhost" {
		t.Errorf("LoadConfig JSON: Endpoint = %q; want %q", cfg.Client.Endpoint, "jsonhost")
	}
	if cfg.Client.EndpointPort != 3333 {
		t.Errorf("LoadConfig JSON: EndpointPort = %d; want %d", cfg.Client.EndpointPort, 3333)
	}
}

func TestLoadClientAndServerConfig(t *testing.T) {
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_ENDPOINT", "cli.example")
	t.Setenv("PBP_TUNNEL_LOCAL_PORT", "4444")

	clientCfg := LoadClientConfig()
	if clientCfg.Endpoint != "cli.example" {
		t.Errorf("LoadClientConfig: Endpoint = %q; want %q", clientCfg.Endpoint, "cli.example")
	}
	if clientCfg.LocalPort != 4444 {
		t.Errorf("LoadClientConfig: LocalPort = %d; want %d", clientCfg.LocalPort, 4444)
	}

	os.Clearenv()
	t.Setenv("PBP_TUNNEL_BIND", "srv.example")
	t.Setenv("PBP_TUNNEL_PORT", "5555")

	serverCfg := LoadServerConfig()
	if serverCfg.BindAddress != "srv.example" {
		t.Errorf("LoadServerConfig: BindAddress = %q; want %q", serverCfg.BindAddress, "srv.example")
	}
	if serverCfg.BindPort != 5555 {
		t.Errorf("LoadServerConfig: BindPort = %d; want %d", serverCfg.BindPort, 5555)
	}
}
