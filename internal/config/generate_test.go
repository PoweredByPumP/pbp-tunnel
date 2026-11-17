package config

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateConfigFile_WritesClientAnswers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.json")
	answers := []string{
		"client", "1", "tunnel.example.com", "2222", "alice", `p&ss<w>"rd\`,
		"127.0.0.1", "3000", "40000", "udp", path,
	}

	if err := GenerateConfigFile(answersReader(answers...), io.Discard); err != nil {
		t.Fatalf("GenerateConfigFile: %v", err)
	}

	want := AppConfig{Type: ModeClient, Client: &ClientParameters{
		HostKeyLevel: 1, Endpoint: "tunnel.example.com", EndpointPort: 2222,
		Username: "alice", Password: `p&ss<w>"rd\`,
		LocalHost: "127.0.0.1", LocalPort: 3000, RemotePort: 40000,
		Protocol: "udp",
	}}
	if got := readGeneratedFile(t, path); !reflect.DeepEqual(got, want) {
		t.Errorf("generated config = %+v, want %+v", got.Client, want.Client)
	}
	if content, _ := os.ReadFile(path); !strings.Contains(string(content), `"p&ss<w>\"rd\\"`) {
		t.Errorf("the password is not written as a person would type it:\n%s", content)
	}
}

func TestGenerateConfigFile_WritesServerAnswers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")
	answers := []string{
		"server", "127.0.0.1", "2222", "40000", "40100", "alice", "secret",
		"/etc/pbp-tunnel/host_rsa", "10.0.0.1, 192.168.0.0/16", path,
	}

	if err := GenerateConfigFile(answersReader(answers...), io.Discard); err != nil {
		t.Fatalf("GenerateConfigFile: %v", err)
	}

	want := AppConfig{Type: ModeServer, Server: &ServerParameters{
		BindAddress: "127.0.0.1", BindPort: 2222, PortRangeStart: 40000, PortRangeEnd: 40100,
		Username: "alice", Password: "secret", PrivateRsaPath: "/etc/pbp-tunnel/host_rsa",
		AllowedIPs: []string{"10.0.0.1", "192.168.0.0/16"},
	}}
	if got := readGeneratedFile(t, path); !reflect.DeepEqual(got, want) {
		t.Errorf("generated config = %+v, want %+v", got.Server, want.Server)
	}
}

func TestGenerateConfigFile_EmptyAnswersKeepTheSuggestedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	emptyAnswers := make([]string, 10)

	if err := GenerateConfigFile(answersReader(append(emptyAnswers, path)...), io.Discard); err != nil {
		t.Fatalf("GenerateConfigFile: %v", err)
	}

	got := readGeneratedFile(t, path)
	if got.Type != ModeClient || got.Client.Endpoint != "127.0.0.1" || got.Client.EndpointPort != DefaultPort || got.Client.Protocol != "tcp" {
		t.Errorf("generated config = %+v, want the suggested client values", got.Client)
	}
}

func TestGenerateConfigFile_InvalidNumberKeepsTheSuggestedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	answers := []string{"server", "", "not-a-port", "", "", "", "", "", "", path}
	var questions strings.Builder

	if err := GenerateConfigFile(answersReader(answers...), &questions); err != nil {
		t.Fatalf("GenerateConfigFile: %v", err)
	}

	if got := readGeneratedFile(t, path); got.Server.BindPort != DefaultPort {
		t.Errorf("BindPort = %d, want %d", got.Server.BindPort, DefaultPort)
	}
	if !strings.Contains(questions.String(), "Invalid number") {
		t.Errorf("the user was not told the number was invalid:\n%s", questions.String())
	}
}

func TestGenerateConfigFile_RejectsUnknownMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	err := GenerateConfigFile(answersReader("proxy", path), io.Discard)

	if err == nil {
		t.Error("GenerateConfigFile accepted an unknown mode")
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("a config file was written for an unknown mode")
	}
}

func answersReader(answers ...string) io.Reader {
	return strings.NewReader(strings.Join(answers, "\n") + "\n")
}

func readGeneratedFile(t *testing.T, path string) AppConfig {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config AppConfig
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatalf("generated file is not valid JSON: %v\n%s", err, content)
	}
	return config
}
