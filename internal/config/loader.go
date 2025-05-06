package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const envPrefix = "PBP_TUNNEL"

// GetEnvValue fetches an environment variable PBP_TUNNEL_<KEY> or returns defaultValue if unset.
// Key should match the JSON tag (e.g., "endpoint", "remote_host", etc.)
func GetEnvValue(key, defaultValue string) string {
	envKey := envPrefix + "_" + strings.ReplaceAll(strings.ToUpper(key), "-", "_")
	if v, ok := os.LookupEnv(envKey); ok && v != "" {
		return v
	}
	return defaultValue
}

// LoadEnvConfig populates AppConfig fields from environment variables only.
func LoadEnvConfig() *AppConfig {
	cfg := &AppConfig{}
	// Client section
	if v := GetEnvValue("endpoint", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		cfg.Client.Endpoint = v
	}
	if v := GetEnvValue("port", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Client.EndpointPort = p
		}
	}
	if v := GetEnvValue("username", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		cfg.Client.Username = v
	}
	if v := GetEnvValue("password", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		cfg.Client.Password = v
	}
	if v := GetEnvValue("identity", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		cfg.Client.PrivateKeyPath = v
	}
	if v := GetEnvValue("host_key", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		cfg.Client.HostKeyPath = v
	}
	if v := GetEnvValue("local_host", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		cfg.Client.LocalHost = v
	}
	if v := GetEnvValue("local_port", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Client.LocalPort = p
		}
	}
	if v := GetEnvValue("remote_host", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		cfg.Client.RemoteHost = v
	}
	if v := GetEnvValue("remote_port", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Client.RemotePort = p
		}
	}
	if v := GetEnvValue("host_key_level", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		if lvl, err := strconv.Atoi(v); err == nil {
			cfg.Client.HostKeyLevel = lvl
		}
	}
	if v := GetEnvValue("allowed_ips", ""); v != "" {
		if cfg.Client == nil {
			cfg.Client = &ClientParameters{}
		}
		cfg.Client.AllowedIPs = strings.Split(v, ",")
	}

	// Server section
	if v := GetEnvValue("bind", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		cfg.Server.BindAddress = v
	}
	if v := GetEnvValue("port", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.BindPort = p
		}
	}
	if v := GetEnvValue("port_range_start", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.PortRangeStart = p
		}
	}
	if v := GetEnvValue("port_range_end", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.PortRangeEnd = p
		}
	}
	if v := GetEnvValue("username", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		cfg.Server.Username = v
	}
	if v := GetEnvValue("password", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		cfg.Server.Password = v
	}
	if v := GetEnvValue("private_rsa", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		cfg.Server.PrivateRsaPath = v
	}
	if v := GetEnvValue("private_ecdsa", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		cfg.Server.PrivateEcdsaPath = v
	}
	if v := GetEnvValue("private_ed25519", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		cfg.Server.PrivateEd25519Path = v
	}
	if v := GetEnvValue("authorized_keys", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		cfg.Server.AuthorizedKeysPath = v
	}
	if v := GetEnvValue("allowed_ips", ""); v != "" {
		if cfg.Server == nil {
			cfg.Server = &ServerParameters{}
		}
		cfg.Server.AllowedIPs = strings.Split(v, ",")
	}

	return cfg
}

// LoadConfig reads JSON config from file (path from PBP_TUNNEL_CONFIG or "config.json"),
// falling back to environment-only config if file is missing or invalid.
func LoadConfig() *AppConfig {
	// Reload environment config each time to reflect latest env values
	envCfg := LoadEnvConfig()
	path := GetEnvValue("config", "config.json")
	if path == "" {
		return envCfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return envCfg
	}
	var fileCfg AppConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
		return &fileCfg
	}
	return &fileCfg
}

// LoadClientConfig returns the current client configuration from JSON or env.
func LoadClientConfig() *ClientParameters {
	cfg := LoadConfig()
	cfg.Type = "client"
	return cfg.Client
}

// LoadServerConfig returns the current server configuration from JSON or env.
func LoadServerConfig() *ServerParameters {
	cfg := LoadConfig()
	cfg.Type = "server"
	return cfg.Server
}
