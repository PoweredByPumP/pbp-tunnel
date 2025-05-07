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

func TestLoadClientConfig_ValidComplete(t *testing.T) {
	// Test with a complete valid client configuration
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "client")
	t.Setenv("PBP_TUNNEL_ENDPOINT", "poweredbypump.com")
	t.Setenv("PBP_TUNNEL_PORT", "52135")
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	t.Setenv("PBP_TUNNEL_LOCAL_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_LOCAL_PORT", "8080")
	t.Setenv("PBP_TUNNEL_REMOTE_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_REMOTE_PORT", "8081")
	t.Setenv("PBP_TUNNEL_HOST_KEY_LEVEL", "0")

	clientCfg := LoadClientConfig()
	if clientCfg == nil {
		t.Error("LoadClientConfig: complete valid configuration returned nil")
	} else {
		if clientCfg.Endpoint != "poweredbypump.com" {
			t.Errorf("LoadClientConfig: Endpoint = %q; want %q", clientCfg.Endpoint, "poweredbypump.com")
		}
		if clientCfg.EndpointPort != 52135 {
			t.Errorf("LoadClientConfig: EndpointPort = %d; want %d", clientCfg.EndpointPort, 52135)
		}
		if clientCfg.Username != "user" {
			t.Errorf("LoadClientConfig: Username = %q; want %q", clientCfg.Username, "user")
		}
		if clientCfg.Password != "fake" {
			t.Errorf("LoadClientConfig: Password = %q; want %q", clientCfg.Password, "fake")
		}
		if clientCfg.LocalHost != "localhost" {
			t.Errorf("LoadClientConfig: LocalHost = %q; want %q", clientCfg.LocalHost, "localhost")
		}
		if clientCfg.LocalPort != 8080 {
			t.Errorf("LoadClientConfig: LocalPort = %d; want %d", clientCfg.LocalPort, 8080)
		}
		if clientCfg.RemoteHost != "localhost" {
			t.Errorf("LoadClientConfig: RemoteHost = %q; want %q", clientCfg.RemoteHost, "localhost")
		}
		if clientCfg.RemotePort != 8081 {
			t.Errorf("LoadClientConfig: RemotePort = %d; want %d", clientCfg.RemotePort, 8081)
		}
		if clientCfg.HostKeyLevel != 0 {
			t.Errorf("LoadClientConfig: HostKeyLevel = %d; want %d", clientCfg.HostKeyLevel, 0)
		}
	}
}

func TestLoadClientConfig_MissingEndpoint(t *testing.T) {
	// Test with missing endpoint - should be invalid
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "client")
	t.Setenv("PBP_TUNNEL_PORT", "52135")
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	t.Setenv("PBP_TUNNEL_LOCAL_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_LOCAL_PORT", "8080")
	t.Setenv("PBP_TUNNEL_REMOTE_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_REMOTE_PORT", "8081")

	invalidClientCfg := LoadClientConfig()
	if invalidClientCfg != nil {
		t.Error("LoadClientConfig: configuration without endpoint didn't return nil")
	}
}

func TestLoadClientConfig_InvalidPort(t *testing.T) {
	// Test with invalid port - should be invalid
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "client")
	t.Setenv("PBP_TUNNEL_ENDPOINT", "poweredbypump.com")
	t.Setenv("PBP_TUNNEL_PORT", "0") // Invalid port
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	t.Setenv("PBP_TUNNEL_LOCAL_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_LOCAL_PORT", "8080")
	t.Setenv("PBP_TUNNEL_REMOTE_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_REMOTE_PORT", "8081")

	invalidClientCfg := LoadClientConfig()
	if invalidClientCfg != nil {
		t.Error("LoadClientConfig: configuration with invalid port didn't return nil")
	}
}

func TestLoadClientConfig_MissingUsername(t *testing.T) {
	// Test with missing username - should be invalid
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "client")
	t.Setenv("PBP_TUNNEL_ENDPOINT", "poweredbypump.com")
	t.Setenv("PBP_TUNNEL_PORT", "52135")
	// Missing username
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	t.Setenv("PBP_TUNNEL_LOCAL_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_LOCAL_PORT", "8080")
	t.Setenv("PBP_TUNNEL_REMOTE_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_REMOTE_PORT", "8081")

	invalidClientCfg := LoadClientConfig()
	if invalidClientCfg != nil {
		t.Error("LoadClientConfig: configuration without username didn't return nil")
	}
}

func TestLoadClientConfig_MissingAuth(t *testing.T) {
	// Test with neither password nor private key - should be invalid
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "client")
	t.Setenv("PBP_TUNNEL_ENDPOINT", "poweredbypump.com")
	t.Setenv("PBP_TUNNEL_PORT", "52135")
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	// Neither password nor private key
	t.Setenv("PBP_TUNNEL_LOCAL_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_LOCAL_PORT", "8080")
	t.Setenv("PBP_TUNNEL_REMOTE_HOST", "localhost")
	t.Setenv("PBP_TUNNEL_REMOTE_PORT", "8081")

	invalidClientCfg := LoadClientConfig()
	if invalidClientCfg != nil {
		t.Error("LoadClientConfig: configuration without password or private key didn't return nil")
	}
}

func TestLoadServerConfig_ValidComplete(t *testing.T) {
	// Test with a complete valid server configuration
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "server")
	t.Setenv("PBP_TUNNEL_BIND", "0.0.0.0")
	t.Setenv("PBP_TUNNEL_PORT", "52135")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_START", "49152")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_END", "65535")
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	t.Setenv("PBP_TUNNEL_PRIVATE_RSA", "id_rsa")

	serverCfg := LoadServerConfig()
	if serverCfg == nil {
		t.Error("LoadServerConfig: complete valid configuration returned nil")
	} else {
		if serverCfg.BindAddress != "0.0.0.0" {
			t.Errorf("LoadServerConfig: BindAddress = %q; want %q", serverCfg.BindAddress, "0.0.0.0")
		}
		if serverCfg.BindPort != 52135 {
			t.Errorf("LoadServerConfig: BindPort = %d; want %d", serverCfg.BindPort, 52135)
		}
		if serverCfg.PortRangeStart != 49152 {
			t.Errorf("LoadServerConfig: PortRangeStart = %d; want %d", serverCfg.PortRangeStart, 49152)
		}
		if serverCfg.PortRangeEnd != 65535 {
			t.Errorf("LoadServerConfig: PortRangeEnd = %d; want %d", serverCfg.PortRangeEnd, 65535)
		}
		if serverCfg.Username != "user" {
			t.Errorf("LoadServerConfig: Username = %q; want %q", serverCfg.Username, "user")
		}
		if serverCfg.Password != "fake" {
			t.Errorf("LoadServerConfig: Password = %q; want %q", serverCfg.Password, "fake")
		}
		if serverCfg.PrivateRsaPath != "id_rsa" {
			t.Errorf("LoadServerConfig: PrivateRsaPath = %q; want %q", serverCfg.PrivateRsaPath, "id_rsa")
		}
	}
}

func TestLoadServerConfig_MissingBindAddress(t *testing.T) {
	// Test with missing bind address - should be invalid
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "server")
	t.Setenv("PBP_TUNNEL_PORT", "52135")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_START", "49152")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_END", "65535")
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	t.Setenv("PBP_TUNNEL_PRIVATE_RSA", "id_rsa")

	t.Setenv("PBP_TUNNEL_BIND", "") // Missing bind address

	invalidServerCfg := LoadServerConfig()
	if invalidServerCfg != nil {
		t.Error("LoadServerConfig: configuration without bind address didn't return nil")
	}
}

func TestLoadServerConfig_InvalidPort(t *testing.T) {
	// Test with invalid port - should be invalid
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "server")
	t.Setenv("PBP_TUNNEL_BIND", "0.0.0.0")
	t.Setenv("PBP_TUNNEL_PORT", "0") // Invalid port
	t.Setenv("PBP_TUNNEL_PORT_RANGE_START", "49152")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_END", "65535")
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	t.Setenv("PBP_TUNNEL_PRIVATE_RSA", "id_rsa")

	invalidServerCfg := LoadServerConfig()
	if invalidServerCfg != nil {
		t.Error("LoadServerConfig: configuration with invalid port didn't return nil")
	}
}

func TestLoadServerConfig_InvalidPortRange(t *testing.T) {
	// Test with invalid port range (end < start) - should be invalid
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "server")
	t.Setenv("PBP_TUNNEL_BIND", "0.0.0.0")
	t.Setenv("PBP_TUNNEL_PORT", "52135")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_START", "65000")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_END", "60000") // End < Start
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	t.Setenv("PBP_TUNNEL_PRIVATE_RSA", "id_rsa")

	invalidServerCfg := LoadServerConfig()
	if invalidServerCfg != nil {
		t.Error("LoadServerConfig: configuration with invalid port range didn't return nil")
	}
}

func TestLoadServerConfig_NoHostKey(t *testing.T) {
	// Test with no host key specified - should be invalid
	os.Clearenv()
	t.Setenv("PBP_TUNNEL_TYPE", "server")
	t.Setenv("PBP_TUNNEL_BIND", "0.0.0.0")
	t.Setenv("PBP_TUNNEL_PORT", "52135")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_START", "49152")
	t.Setenv("PBP_TUNNEL_PORT_RANGE_END", "65535")
	t.Setenv("PBP_TUNNEL_USERNAME", "user")
	t.Setenv("PBP_TUNNEL_PASSWORD", "fake")
	// No host key (neither RSA, ECDSA, nor ED25519)

	invalidServerCfg := LoadServerConfig()
	if invalidServerCfg != nil {
		t.Error("LoadServerConfig: configuration without host key didn't return nil")
	}
}
