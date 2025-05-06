package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to reset and capture stdin
type stdinHelper struct {
	old *os.File
	r   *os.File
}

func newStdinHelper(input string) *stdinHelper {
	r, w, _ := os.Pipe()
	w.WriteString(input)
	w.Close()
	return &stdinHelper{old: os.Stdin, r: r}
}

func (h *stdinHelper) restore() {
	os.Stdin = h.old
}

func withStdin(input string, fn func()) {
	help := newStdinHelper(input)
	os.Stdin = help.r
	fn()
	help.restore()
}

const tempDirPrefix = ".pbp-tunnel_test"

// makeTempDir creates a temp directory under the current working directory to avoid permission issues on Windows env
func makeTempDir(t *testing.T) string {
	userDir, _ := os.Getwd()

	dir, err := os.MkdirTemp(userDir, tempDirPrefix)
	if err != nil {
		t.Fatalf("failed to create temp dir under current directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestAsk_DefaultAndCustom(t *testing.T) {
	// Default when blank
	withStdin("\n", func() {
		val := ask("prompt?", "def")
		if val != "def" {
			t.Errorf("ask default = %q; want %q", val, "def")
		}
	})
	// Custom input
	withStdin("custom\n", func() {
		val := ask("prompt?", "def")
		if val != "custom" {
			t.Errorf("ask custom = %q; want %q", val, "custom")
		}
	})
}

func TestAskInt_DefaultAndCustom(t *testing.T) {
	// Default when blank
	withStdin("\n", func() {
		i := askInt("number?", 42)
		if i != 42 {
			t.Errorf("askInt default = %d; want %d", i, 42)
		}
	})
	// Valid integer
	withStdin("7\n", func() {
		i := askInt("number?", 42)
		if i != 7 {
			t.Errorf("askInt custom = %d; want %d", i, 7)
		}
	})
	// Invalid integer falls back
	withStdin("bad\n", func() {
		i := askInt("number?", 42)
		if i != 42 {
			t.Errorf("askInt invalid = %d; want %d", i, 42)
		}
	})
}

func TestGenerateConfigTemplate_ClientDefaults(t *testing.T) {
	// All blank inputs to use defaults
	inputs := strings.Repeat("\n", 11)
	dir := makeTempDir(t)

	oldWd, _ := os.Getwd()
	err := os.Chdir(dir)
	if err != nil {
		t.Fatalf("failed to change working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	withStdin(inputs, func() {
		err := GenerateConfigTemplate()
		if err != nil {
			t.Fatalf("GenerateConfigTemplate error: %v", err)
		}
	})

	// Read and verify output
	out := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Type != "client" {
		t.Errorf("Type = %q; want %q", cfg.Type, "client")
	}
	if cfg.Client == nil {
		t.Fatal("Client section missing")
	}
	// Check a few defaults
	if cfg.Client.Endpoint != "127.0.0.1" {
		t.Errorf("Endpoint = %q; want %q", cfg.Client.Endpoint, "127.0.0.1")
	}
	if cfg.Client.HostKeyLevel != 0 {
		t.Errorf("HostKeyLevel = %d; want %d", cfg.Client.HostKeyLevel, 0)
	}
	if cfg.Client.LocalPort != 8080 {
		t.Errorf("LocalPort = %d; want %d", cfg.Client.LocalPort, 8080)
	}
}

func TestGenerateConfigTemplate_ServerDefaults(t *testing.T) {
	// First answer 'server', then blanks
	inputs := "server\n" + strings.Repeat("\n", 8)
	dir := makeTempDir(t)
	oldWd, _ := os.Getwd()
	err := os.Chdir(dir)
	if err != nil {
		t.Fatalf("failed to change working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	withStdin(inputs, func() {
		err := GenerateConfigTemplate()
		if err != nil {
			t.Fatalf("GenerateConfigTemplate error: %v", err)
		}
	})

	out := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if cfg.Type != "server" {
		t.Errorf("Type = %q; want %q", cfg.Type, "server")
	}
	if cfg.Server == nil {
		t.Fatal("Server section missing")
	}
	// Check a default bind address
	if cfg.Server.BindAddress != "0.0.0.0" {
		t.Errorf("BindAddress = %q; want %q", cfg.Server.BindAddress, "0.0.0.0")
	}
}
