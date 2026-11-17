package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// GenerateConfigFile asks its questions on out, reads one answer per line from in
// and writes the config file. An empty answer keeps the value shown in brackets.
func GenerateConfigFile(in io.Reader, out io.Writer) error {
	questions := &questionnaire{answers: bufio.NewScanner(in), out: out}

	config := AppConfig{Type: Mode(questions.ask("Generate a config for (client/server)", string(ModeClient)))}
	switch config.Type {
	case ModeClient:
		config.Client = askClientParameters(questions)
	case ModeServer:
		config.Server = askServerParameters(questions)
	default:
		return fmt.Errorf("unknown mode %q, expected client or server", config.Type)
	}

	path := questions.ask("Output file path", defaultConfigFile)
	if err := writeConfigFile(path, config); err != nil {
		return err
	}
	fmt.Fprintf(out, "Configuration written to %s\n", path)
	return nil
}

func askClientParameters(questions *questionnaire) *ClientParameters {
	return &ClientParameters{
		HostKeyLevel: questions.askInt("Host key level", 0),
		Endpoint:     questions.ask("Server endpoint", "127.0.0.1"),
		EndpointPort: questions.askInt("Server port", DefaultPort),
		Username:     questions.ask("Username", "user"),
		Password:     questions.ask("Password", "changeme"),
		LocalHost:    questions.ask("Local host to forward", "localhost"),
		LocalPort:    questions.askInt("Local port", 8080),
		RemotePort:   questions.askInt("Remote port to request", 0),
		Protocol:     questions.ask("Forward protocol (tcp/udp)", "tcp"),
	}
}

func askServerParameters(questions *questionnaire) *ServerParameters {
	return &ServerParameters{
		BindAddress:    questions.ask("Bind address", "0.0.0.0"),
		BindPort:       questions.askInt("Bind port", DefaultPort),
		PortRangeStart: questions.askInt("Port range start", 49152),
		PortRangeEnd:   questions.askInt("Port range end", 65535),
		Username:       questions.ask("Username", "user"),
		Password:       questions.ask("Password", "changeme"),
		PrivateRsaPath: questions.ask("Private key path", "id_rsa"),
		AllowedIPs:     splitCommaSeparated(questions.ask("Allowed IPs (comma separated)", "")),
	}
}

// writeConfigFile is readable by its owner only, since the file holds a password.
func writeConfigFile(path string, config AppConfig) error {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

type questionnaire struct {
	answers *bufio.Scanner
	out     io.Writer
}

// ask returns defaultAnswer for an empty answer or when there are no answers left.
func (q *questionnaire) ask(question, defaultAnswer string) string {
	fmt.Fprintf(q.out, "%s [%s]: ", question, defaultAnswer)
	if !q.answers.Scan() {
		return defaultAnswer
	}
	if answer := strings.TrimSpace(q.answers.Text()); answer != "" {
		return answer
	}
	return defaultAnswer
}

func (q *questionnaire) askInt(question string, defaultAnswer int) int {
	answer := q.ask(question, strconv.Itoa(defaultAnswer))
	number, err := strconv.Atoi(answer)
	if err != nil {
		fmt.Fprintf(q.out, "Invalid number, using default: %d\n", defaultAnswer)
		return defaultAnswer
	}
	return number
}
