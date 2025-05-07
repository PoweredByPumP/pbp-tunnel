package util

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/ed25519"
	"os"
)

func GenerateAndSavePrivateKeyToFile(filePath, keyType string) ([]byte, error) {
	var keyBytes []byte

	switch keyType {
	case "ecdsa":
		privateKey, err := GenerateECDSAPrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate ECDSA key: %v", err)
		}

		keyBytes, err = EncodeECDSAPrivateKeyToPEM(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encode ECDSA key: %v", err)
		}
	case "ed25519":
		privateKey, err := GenerateED25519PrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate Ed25519 key: %v", err)
		}

		keyBytes, err = EncodeED25519PrivateKeyToPEM(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encode Ed25519 key: %v", err)
		}
	case "rsa":
		privateKey, err := GenerateRSAPrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA key: %v", err)
		}

		keyBytes, err = EncodeRSAPrivateKeyToPEM(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encode RSA key: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}

	return savePrivateKeyPemToFile(filePath, keyBytes)
}

func GenerateRSAPrivateKey() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA privateKey: %v", err)
	}

	return privateKey, nil
}

func EncodeRSAPrivateKeyToPEM(privateKey *rsa.PrivateKey) ([]byte, error) {
	der := x509.MarshalPKCS1PrivateKey(privateKey)

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: der,
	}

	return pem.EncodeToMemory(block), nil
}

func GenerateECDSAPrivateKey() (*ecdsa.PrivateKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ECDSA privateKey: %v", err)
	}

	return privateKey, nil
}

func EncodeECDSAPrivateKeyToPEM(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ECDSA privateKey: %v", err)
	}

	block := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	}

	return pem.EncodeToMemory(block), nil
}

func GenerateED25519PrivateKey() (crypto.PrivateKey, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key: %v", err)
	}

	return privateKey, nil
}

func EncodeED25519PrivateKeyToPEM(privateKey crypto.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Ed25519 key: %v", err)
	}

	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}

	return pem.EncodeToMemory(block), nil
}

func savePrivateKeyPemToFile(filePath string, privateKeyBytes []byte) ([]byte, error) {
	err := os.WriteFile(filePath, privateKeyBytes, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write private key to file: %v", err)
	}

	return privateKeyBytes, nil
}
