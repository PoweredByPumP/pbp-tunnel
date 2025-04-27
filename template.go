package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
)

//go:embed template/config.json.tmpl
var configJsonTemplate string

func GenerateConfigTemplate() {
	mode := ask("GenerateConfigTemplate config for (client/server)", "client")

	var config AppConfig
	config.Type = mode

	if mode == "client" {
		config.Client = &ClientParameters{
			HostKeyLevel: askInt("Host key level", 0),
			Endpoint:     ask("Server endpoint", "127.0.0.1"),
			EndpointPort: askInt("Server port", 52135),
			Username:     ask("Username", "user"),
			Password:     ask("Password", "changeme"),
			LocalHost:    ask("Local host to forward", "localhost"),
			LocalPort:    askInt("Local port", 8080),
			RemoteHost:   ask("Remote host to expose", "localhost"),
			RemotePort:   askInt("Remote port to request", 0),
		}
	} else if mode == "server" {
		config.Server = &ServerParameters{
			BindAddress:    ask("Bind address", "0.0.0.0"),
			BindPort:       askInt("Bind port", 52135),
			PortRangeStart: askInt("Port range start", 49152),
			PortRangeEnd:   askInt("Port range end", 65535),
			Username:       ask("Username", "user"),
			Password:       ask("Password", "changeme"),
			PrivateRsaPath: ask("Private key path", "id_rsa"),
			AllowedIPs:     strings.Split(ask("Allowed IPs (comma separated)", ""), ","),
		}
		for i := range config.Server.AllowedIPs {
			config.Server.AllowedIPs[i] = strings.TrimSpace(config.Server.AllowedIPs[i])
		}
	}

	outFile := ask("Output file path", "config.json")
	f, err := os.Create(outFile)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	tmpl := template.Must(template.New("config").Parse(configJsonTemplate))
	err = tmpl.Execute(f, config)
	if err != nil {
		fmt.Printf("Error generating config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Configuration written to %s\n", outFile)
}

func ask(prompt string, defaultVal string) string {
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
