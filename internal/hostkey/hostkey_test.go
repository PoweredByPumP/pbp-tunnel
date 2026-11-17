package hostkey

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestEnsureExists_CreatesAKeyTheSSHServerCanLoad(t *testing.T) {
	tests := []struct {
		algorithm         Algorithm
		wantPublicKeyType string
	}{
		{RSA, ssh.KeyAlgoRSA},
		{ECDSA, ssh.KeyAlgoECDSA256},
		{Ed25519, ssh.KeyAlgoED25519},
	}
	for _, tc := range tests {
		t.Run(string(tc.algorithm), func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "host_key")

			created, err := EnsureExists(path, tc.algorithm)

			if err != nil || !created {
				t.Fatalf("EnsureExists = %v, %v; want true, nil", created, err)
			}
			signer, err := ssh.ParsePrivateKey(mustReadFile(t, path))
			if err != nil {
				t.Fatalf("ssh.ParsePrivateKey: %v", err)
			}
			if got := signer.PublicKey().Type(); got != tc.wantPublicKeyType {
				t.Errorf("public key type = %q, want %q", got, tc.wantPublicKeyType)
			}
		})
	}
}

func TestEnsureExists_RestrictsKeyFileToItsOwner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not map file permissions to Unix modes")
	}
	path := filepath.Join(t.TempDir(), "host_key")

	if _, err := EnsureExists(path, Ed25519); err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("key file mode = %o, want 600", mode)
	}
}

func TestEnsureExists_CreatesMissingDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "etc", "pbp-tunnel", "host_key")

	if _, err := EnsureExists(path, Ed25519); err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("key file was not created: %v", err)
	}
}

func TestEnsureExists_KeepsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host_key")
	if err := os.WriteFile(path, []byte("existing key"), 0o600); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureExists(path, Ed25519)

	if err != nil || created {
		t.Errorf("EnsureExists = %v, %v; want false, nil", created, err)
	}
	if content := string(mustReadFile(t, path)); content != "existing key" {
		t.Errorf("existing file was overwritten with %q", content)
	}
}

func TestEnsureExists_RejectsUnsupportedAlgorithm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host_key")

	if _, err := EnsureExists(path, Algorithm("dsa")); err == nil {
		t.Error("EnsureExists accepted an unsupported algorithm")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a file was created for an unsupported algorithm")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}
