package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var envConfig *AppConfig

func GetEnvValue(key, defaultValue string) string {
	key = "PBP_TUNNEL_" + strings.ReplaceAll(strings.ToUpper(key), "-", "_")

	envValue, found := os.LookupEnv(key)
	if found {
		return envValue
	}

	return defaultValue
}

func LoadClientConfig() *ClientParameters {
	config := LoadConfig()
	if config != nil && config.Type == "client" && config.Client != nil {
		return config.Client
	}

	return nil
}

func LoadServerConfig() *ServerParameters {
	config := LoadConfig()
	if config != nil && config.Type == "server" && config.Server != nil {
		return config.Server
	}

	return nil
}

func LoadConfig() *AppConfig {
	if envConfig == nil {
		envConfig = LoadEnvConfig()
	}

	configFilePath := GetEnvValue("config", "./config.json")
	if configFilePath == "" {
		return envConfig
	}

	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		return envConfig
	}

	var config AppConfig
	err = json.Unmarshal(configData, &config)
	if err != nil {
		fmt.Printf("Error parsing config file: %v\n", err)
		return nil
	}

	return &config
}

func LoadEnvConfig() *AppConfig {
	var err error

	config := &AppConfig{
		Type:   GetEnvValue("type", ""),
		Client: &ClientParameters{},
		Server: &ServerParameters{},
	}

	config.Client.Endpoint = GetEnvValue("endpoint", CpDefaultEndpoint)
	config.Client.EndpointPort, err = strconv.Atoi(GetEnvValue("port", strconv.Itoa(CpDefaultEndpointPort)))
	if err != nil {
		fmt.Println("Invalid environment port value, using default")
		config.Client.EndpointPort = CpDefaultEndpointPort
	}

	config.Client.Username = GetEnvValue("username", CpDefaultUsername)
	config.Client.Password = GetEnvValue("password", CpDefaultPassword)
	config.Client.PrivateKeyPath = GetEnvValue("identity", CpDefaultPrivateKeyPath)
	config.Client.HostKeyPath = GetEnvValue("host_key", CpDefaultHostKeyPath)
	config.Client.LocalHost = GetEnvValue("local_host", CpDefaultLocalHost)
	config.Client.LocalPort, err = strconv.Atoi(GetEnvValue("local_port", strconv.Itoa(CpDefaultLocalPort)))
	if err != nil {
		fmt.Println("Invalid environment local port value, using default")
		config.Client.LocalPort = CpDefaultLocalPort
	}

	config.Client.RemoteHost = GetEnvValue("remote_host", CpDefaultRemoteHost)
	config.Client.RemotePort, err = strconv.Atoi(GetEnvValue("remote_port", strconv.Itoa(CpDefaultRemotePort)))
	if err != nil {
		fmt.Println("Invalid environment remote port value, using default")
		config.Client.RemotePort = CpDefaultRemotePort
	}

	config.Client.HostKeyLevel, err = strconv.Atoi(GetEnvValue("host_key_level", strconv.Itoa(CpDefaultHostKeyLevel)))
	if err != nil {
		fmt.Println("Invalid environment host key level value, using default")
		config.Client.HostKeyLevel = CpDefaultHostKeyLevel
	}

	config.Server.BindAddress = GetEnvValue("bind_address", SpDefaultBindAddress)
	config.Server.BindPort, err = strconv.Atoi(GetEnvValue("bind_port", strconv.Itoa(SpDefaultBindPort)))
	if err != nil {
		fmt.Println("Invalid environment bind port value, using default")
		config.Server.BindPort = SpDefaultBindPort
	}

	config.Server.PortRangeStart, err = strconv.Atoi(GetEnvValue("port_range_start", strconv.Itoa(SpDefaultPortRangeStart)))
	if err != nil {
		fmt.Println("Invalid environment port range start value, using default")
		config.Server.PortRangeStart = SpDefaultPortRangeStart
	}

	config.Server.PortRangeEnd, err = strconv.Atoi(GetEnvValue("port_range_end", strconv.Itoa(SpDefaultPortRangeEnd)))
	if err != nil {
		fmt.Println("Invalid environment port range end value, using default")
		config.Server.PortRangeEnd = SpDefaultPortRangeEnd
	}

	config.Server.Username = GetEnvValue("username", SpDefaultUsername)
	config.Server.Password = GetEnvValue("password", SpDefaultPassword)
	config.Server.PrivateRsaPath = GetEnvValue("private_rsa_path", SpDefaultPrivateRsa)
	config.Server.PrivateEcdsaPath = GetEnvValue("private_ecdsa_path", SpDefaultPrivateEcdsa)
	config.Server.PrivateEd25519Path = GetEnvValue("private_ed25519_path", SpDefaultPrivateEd25519)
	config.Server.AuthorizedKeysPath = GetEnvValue("authorized_keys_path", SpDefaultAuthorizedKeys)
	config.Server.AllowedIPs = strings.Split(strings.TrimSpace(GetEnvValue("allowed_ips", "")), ",")

	return config
}
