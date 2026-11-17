package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// GenerateConfigTemplate interactively prompts the user and writes a config file
func GenerateConfigTemplate() error {
	mode := ask("GenerateConfigTemplate config for (client/server)", "client")

	var config AppConfig
	config.Type = Mode(mode)

	if mode == "client" {
		config.Client = &ClientParameters{
			HostKeyLevel: askInt("Host key level", 0),
			Endpoint:     ask("Server endpoint", "127.0.0.1"),
			EndpointPort: askInt("Server port", DefaultPort),
			Username:     ask("Username", "user"),
			Password:     ask("Password", "changeme"),
			LocalHost:    ask("Local host to forward", "localhost"),
			LocalPort:    askInt("Local port", 8080),
			RemoteHost:   ask("Remote host to expose", "localhost"),
			RemotePort:   askInt("Remote port to request", 0),
			Protocol:     ask("Forward protocol (tcp/udp)", "tcp"),
		}
	} else if mode == "server" {
		config.Server = &ServerParameters{
			BindAddress:    ask("Bind address", "0.0.0.0"),
			BindPort:       askInt("Bind port", DefaultPort),
			PortRangeStart: askInt("Port range start", 49152),
			PortRangeEnd:   askInt("Port range end", 65535),
			Username:       ask("Username", "user"),
			Password:       ask("Password", "changeme"),
			PrivateRsaPath: ask("Private key path", "id_rsa"),
			AllowedIPs:     nil,
		}
		ips := ask("Allowed IPs (comma separated)", "")
		if ips != "" {
			entries := strings.Split(ips, ",")
			for i := range entries {
				entries[i] = strings.TrimSpace(entries[i])
			}
			config.Server.AllowedIPs = entries
		}
	}

	outFile := ask("Output file path", "config.json")
	if err := writeConfigFile(outFile, config); err != nil {
		return err
	}

	fmt.Printf("Configuration written to %s\n", outFile)
	return nil
}

// writeConfigFile is readable by its owner only, since the file holds a password.
func writeConfigFile(path string, config AppConfig) error {
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func ask(prompt, defaultVal string) string {
	r := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [%s]: ", prompt, defaultVal)
	input, _ := r.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func askInt(prompt string, defaultVal int) int {
	val := ask(prompt, strconv.Itoa(defaultVal))
	i, err := strconv.Atoi(val)
	if err != nil {
		fmt.Printf("Invalid number, using default: %d\n", defaultVal)
		return defaultVal
	}
	return i
}
