package util

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"golang.org/x/crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateRSAPrivateKey verifies that RSA key generation works properly
func TestGenerateRSAPrivateKey(t *testing.T) {
	privateKey, err := GenerateRSAPrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Verify the key is of the expected type
	if privateKey == nil {
		t.Fatal("Generated RSA key is nil")
	}

	// Verify key size (should be 4096 bits)
	if privateKey.N.BitLen() != 4096 {
		t.Errorf("Expected 4096-bit RSA key, got %d bits", privateKey.N.BitLen())
	}

	// Test key functionality
	if err := privateKey.Validate(); err != nil {
		t.Errorf("Generated RSA key is invalid: %v", err)
	}
}

// TestEncodeRSAPrivateKeyToPEM tests the encoding of RSA keys to PEM format
func TestEncodeRSAPrivateKeyToPEM(t *testing.T) {
	privateKey, err := GenerateRSAPrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	pemBytes, err := EncodeRSAPrivateKeyToPEM(privateKey)
	if err != nil {
		t.Fatalf("Failed to encode RSA key to PEM: %v", err)
	}

	// Verify PEM structure
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("Failed to decode PEM block")
	}

	// Verify PEM type
	if block.Type != "RSA PRIVATE KEY" {
		t.Errorf("Expected PEM type 'RSA PRIVATE KEY', got '%s'", block.Type)
	}

	// Verify key can be parsed back
	parsedKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse RSA key from DER: %v", err)
	}

	if parsedKey.N.Cmp(privateKey.N) != 0 {
		t.Error("Parsed key doesn't match the original key")
	}
}

// TestGenerateECDSAPrivateKey verifies that ECDSA key generation works properly
func TestGenerateECDSAPrivateKey(t *testing.T) {
	privateKey, err := GenerateECDSAPrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate ECDSA key: %v", err)
	}

	// Verify the key is of the expected type
	if privateKey == nil {
		t.Fatal("Generated ECDSA key is nil")
	}

	// Verify curve type
	if privateKey.Curve.Params().Name != "P-256" {
		t.Errorf("Expected P-256 curve, got %s", privateKey.Curve.Params().Name)
	}
}

// TestEncodeECDSAPrivateKeyToPEM tests the encoding of ECDSA keys to PEM format
func TestEncodeECDSAPrivateKeyToPEM(t *testing.T) {
	privateKey, err := GenerateECDSAPrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate ECDSA key: %v", err)
	}

	pemBytes, err := EncodeECDSAPrivateKeyToPEM(privateKey)
	if err != nil {
		t.Fatalf("Failed to encode ECDSA key to PEM: %v", err)
	}

	// Verify PEM structure
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("Failed to decode PEM block")
	}

	// Verify PEM type
	if block.Type != "EC PRIVATE KEY" {
		t.Errorf("Expected PEM type 'EC PRIVATE KEY', got '%s'", block.Type)
	}

	// Verify key can be parsed back
	parsedKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse ECDSA key from DER: %v", err)
	}

	if parsedKey.X.Cmp(privateKey.X) != 0 || parsedKey.Y.Cmp(privateKey.Y) != 0 {
		t.Error("Parsed key doesn't match the original key")
	}
}

// TestGenerateED25519PrivateKey verifies that ED25519 key generation works properly
func TestGenerateED25519PrivateKey(t *testing.T) {
	privateKey, err := GenerateED25519PrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	// Verify the key is of the expected type
	if privateKey == nil {
		t.Fatal("Generated Ed25519 key is nil")
	}

	// Verify it's an Ed25519 key
	_, ok := privateKey.(ed25519.PrivateKey)
	if !ok {
		t.Errorf("Generated key is not of type ed25519.PrivateKey")
	}
}

// TestEncodeED25519PrivateKeyToPEM tests the encoding of ED25519 keys to PEM format
func TestEncodeED25519PrivateKeyToPEM(t *testing.T) {
	privateKey, err := GenerateED25519PrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	pemBytes, err := EncodeED25519PrivateKeyToPEM(privateKey)
	if err != nil {
		t.Fatalf("Failed to encode Ed25519 key to PEM: %v", err)
	}

	// Verify PEM structure
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("Failed to decode PEM block")
	}

	// Verify PEM type
	if block.Type != "PRIVATE KEY" {
		t.Errorf("Expected PEM type 'PRIVATE KEY', got '%s'", block.Type)
	}

	// Verify key can be parsed back
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse Ed25519 key from DER: %v", err)
	}

	_, ok := parsedKey.(ed25519.PrivateKey)
	if !ok {
		t.Error("Parsed key is not of type ed25519.PrivateKey")
	}
}

// TestSavePrivateKeyPemToFile tests saving a PEM-encoded key to a file
func TestSavePrivateKeyPemToFile(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "key-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFilePath := filepath.Join(tempDir, "test-key.pem")
	testContent := []byte("TEST PRIVATE KEY CONTENT")

	// Test saving the key
	savedBytes, err := savePrivateKeyPemToFile(testFilePath, testContent)
	if err != nil {
		t.Fatalf("savePrivateKeyPemToFile failed: %v", err)
	}

	// Verify returned bytes match the input
	if !bytes.Equal(savedBytes, testContent) {
		t.Error("Returned bytes don't match the input content")
	}

	// Verify file exists
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		t.Fatalf("Key file wasn't created at %s", testFilePath)
	}

	// Verify file contents
	fileContent, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("Failed to read key file: %v", err)
	}
	if !bytes.Equal(fileContent, testContent) {
		t.Error("File content doesn't match the input content")
	}
}

// TestGenerateAndSavePrivateKeyToFile tests the end-to-end function for all key types
func TestGenerateAndSavePrivateKeyToFile(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "key-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test for each supported key type
	keyTypes := []string{"rsa", "ecdsa", "ed25519"}
	for _, keyType := range keyTypes {
		t.Run(keyType, func(t *testing.T) {
			testFilePath := filepath.Join(tempDir, keyType+"-key.pem")

			keyBytes, err := GenerateAndSavePrivateKeyToFile(testFilePath, keyType)
			if err != nil {
				t.Fatalf("Failed to generate and save %s key: %v", keyType, err)
			}

			// Verify file exists
			if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
				t.Fatalf("Key file wasn't created at %s", testFilePath)
			}

			// Verify file contents
			fileContent, err := os.ReadFile(testFilePath)
			if err != nil {
				t.Fatalf("Failed to read key file: %v", err)
			}
			if !bytes.Equal(fileContent, keyBytes) {
				t.Error("File content doesn't match the returned bytes")
			}

			// Verify PEM structure
			block, _ := pem.Decode(fileContent)
			if block == nil {
				t.Fatal("Failed to decode PEM block from file")
			}

			// Verify expected PEM type based on key type
			var expectedType string
			switch keyType {
			case "rsa":
				expectedType = "RSA PRIVATE KEY"
			case "ecdsa":
				expectedType = "EC PRIVATE KEY"
			case "ed25519":
				expectedType = "PRIVATE KEY"
			}

			if block.Type != expectedType {
				t.Errorf("Expected PEM type '%s', got '%s'", expectedType, block.Type)
			}
		})
	}

	// Test with unsupported key type
	t.Run("unsupported", func(t *testing.T) {
		testFilePath := filepath.Join(tempDir, "unsupported-key.pem")
		_, err := GenerateAndSavePrivateKeyToFile(testFilePath, "unsupported")
		if err == nil {
			t.Fatal("Expected error for unsupported key type, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported key type") {
			t.Errorf("Expected 'unsupported key type' error, got: %v", err)
		}
	})
}

// TestErrorCases tests various error conditions
func TestErrorCases(t *testing.T) {
	// Test with invalid file path
	_, err := savePrivateKeyPemToFile("/invalid/path/that/should/not/exist", []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid file path, got nil")
	}
}
