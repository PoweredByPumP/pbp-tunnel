package hostkey

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

type Algorithm string

const (
	RSA     Algorithm = "rsa"
	ECDSA   Algorithm = "ecdsa"
	Ed25519 Algorithm = "ed25519"
)

const rsaKeyBits = 4096

// EnsureExists generates a private key at path unless a file is already there,
// and reports whether it created one. An existing file is never overwritten.
func EnsureExists(path string, algorithm Algorithm) (created bool, err error) {
	_, err = os.Stat(path)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	encodedKey, err := generate(algorithm)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := writeNewFile(path, encodedKey); err != nil {
		return false, err
	}
	return true, nil
}

// generate returns a new private key in the OpenSSH PEM format.
func generate(algorithm Algorithm) ([]byte, error) {
	privateKey, err := newPrivateKey(algorithm)
	if err != nil {
		return nil, fmt.Errorf("generate %s key: %w", algorithm, err)
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		return nil, fmt.Errorf("encode %s key: %w", algorithm, err)
	}
	return pem.EncodeToMemory(block), nil
}

func newPrivateKey(algorithm Algorithm) (crypto.PrivateKey, error) {
	switch algorithm {
	case RSA:
		return rsa.GenerateKey(rand.Reader, rsaKeyBits)
	case ECDSA:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case Ed25519:
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		return privateKey, err
	default:
		return nil, fmt.Errorf("unsupported algorithm %q", algorithm)
	}
}

// writeNewFile fails instead of replacing a file created since EnsureExists looked.
func writeNewFile(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
