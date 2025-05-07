package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const envPrefix = "PBP_TUNNEL_"

// GetEnvValue fetches an environment variable PBP_TUNNEL_<KEY> or returns defaultValue if unset.
// KEY should match the JSON tag in caps (e.g., "ENDPOINT", "REMOTE_HOST", etc.)
func GetEnvValue(key, defaultValue string) string {
	formatedKey := strings.ReplaceAll(strings.ToUpper(key), "-", "_")

	envKey := envPrefix + formatedKey

	v, ok := os.LookupEnv(envKey)
	if ok && v != "" {
		return v
	} else if ok {
		return ""
	}

	return defaultValue
}

// LoadEnvConfig populates AppConfig fields from environment variables only.
func LoadEnvConfig() *AppConfig {
	configuration := &AppConfig{}
	configuration.Client = &ClientParameters{}
	configuration.Server = &ServerParameters{}

	// Type section
	if v := GetEnvValue("type", ""); v != "" {
		configuration.Type = v
	}

	// Client section
	if v := GetEnvValue(CpKeyEndpoint, ""); v != "" {
		if configuration.Client == nil {
		}
		configuration.Client.Endpoint = v
	}
	if v := GetEnvValue(CpKeyEndpointPort, strconv.Itoa(CpDefaultEndpointPort)); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			configuration.Client.EndpointPort = p
		}
	}
	if v := GetEnvValue(CpKeyUsername, ""); v != "" {
		configuration.Client.Username = v
	}
	if v := GetEnvValue(CpKeyPassword, ""); v != "" {
		configuration.Client.Password = v
	}
	if v := GetEnvValue(CpKeyPrivateKeyPath, ""); v != "" {
		configuration.Client.PrivateKeyPath = v
	}
	if v := GetEnvValue(CpKeyHostKeyPath, ""); v != "" {
		configuration.Client.HostKeyPath = v
	}
	if v := GetEnvValue(CpKeyLocalHost, CpDefaultLocalHost); v != "" {
		configuration.Client.LocalHost = v
	}
	if v := GetEnvValue(CpKeyLocalPort, strconv.Itoa(CpDefaultLocalPort)); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			configuration.Client.LocalPort = p
		}
	}
	if v := GetEnvValue(CpKeyRemoteHost, CpDefaultRemoteHost); v != "" {
		configuration.Client.RemoteHost = v
	}
	if v := GetEnvValue(CpKeyRemotePort, strconv.Itoa(CpDefaultRemotePort)); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			configuration.Client.RemotePort = p
		}
	}
	if v := GetEnvValue(CpKeyHostKeyLevel, strconv.Itoa(CpDefaultHostKeyLevel)); v != "" {
		if lvl, err := strconv.Atoi(v); err == nil {
			configuration.Client.HostKeyLevel = lvl
		}
	}
	if v := GetEnvValue(CpKeyAllowedIPs, ""); v != "" {
		configuration.Client.AllowedIPs = strings.Split(v, ",")
	}

	// Server section
	if v := GetEnvValue(SpKeyBindAddress, SpDefaultBindAddress); v != "" {
		configuration.Server.BindAddress = v
	}
	if v := GetEnvValue(SpKeyBindPort, strconv.Itoa(SpDefaultBindPort)); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			configuration.Server.BindPort = p
		}
	}
	if v := GetEnvValue(SpKeyPortRangeStart, strconv.Itoa(SpDefaultPortRangeStart)); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			configuration.Server.PortRangeStart = p
		}
	}
	if v := GetEnvValue(SpKeyPortRangeEnd, strconv.Itoa(SpDefaultPortRangeEnd)); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			configuration.Server.PortRangeEnd = p
		}
	}
	if v := GetEnvValue(SpKeyUsername, ""); v != "" {
		configuration.Server.Username = v
	}
	if v := GetEnvValue(SpKeyPassword, ""); v != "" {
		configuration.Server.Password = v
	}
	if v := GetEnvValue(SpKeyPrivateRsaPath, ""); v != "" {
		configuration.Server.PrivateRsaPath = v
	}
	if v := GetEnvValue(SpKeyPrivateEcdsaPath, ""); v != "" {
		configuration.Server.PrivateEcdsaPath = v
	}
	if v := GetEnvValue(SpKeyPrivateEd25519Path, ""); v != "" {
		configuration.Server.PrivateEd25519Path = v
	}
	if v := GetEnvValue(SpKeyAuthorizedKeysPath, ""); v != "" {
		configuration.Server.AuthorizedKeysPath = v
	}
	if v := GetEnvValue(SpKeyAllowedIPS, ""); v != "" {
		configuration.Server.AllowedIPs = strings.Split(v, ",")
	}

	return configuration
}

// LoadConfig reads JSON config from file (path from PBP_TUNNEL_CONFIG or "config.json"),
// falling back to environment-only config if file is missing or invalid.
func LoadConfig() *AppConfig {
	envConfig := LoadEnvConfig()
	if envConfig.Type != "" {
		return envConfig
	}

	configFilepath := GetEnvValue("config", "")

	hasDefaultValue := false
	if configFilepath == "" {
		hasDefaultValue = true
		configFilepath = "config.json"
	}

	configBytes, err := os.ReadFile(configFilepath)
	if err != nil {
		if !hasDefaultValue {
			_, _ = fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
			_, _ = fmt.Fprintf(os.Stderr, "Falling back to environment variables.\n")
		}

		return envConfig
	}

	var fileConfig AppConfig
	if err := json.Unmarshal(configBytes, &fileConfig); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)

		return &fileConfig
	}

	return &fileConfig
}

// LoadClientConfig returns the current client configuration from JSON or env.
func LoadClientConfig() *ClientParameters {
	configuration := LoadConfig()

	if err := configuration.Client.Validate(); err != nil {
		return nil
	}

	return configuration.Client
}

// LoadServerConfig returns the current server configuration from JSON or env.
func LoadServerConfig() *ServerParameters {
	configuration := LoadConfig()

	if err := configuration.Server.Validate(); err != nil {
		return nil
	}

	return configuration.Server
}
