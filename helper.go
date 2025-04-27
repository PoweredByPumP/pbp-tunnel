package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/mattn/go-isatty"
	"golang.org/x/term"
	"os"
)

var isColorEnabled = isatty.IsTerminal(os.Stdout.Fd()) || term.IsTerminal(int(os.Stdout.Fd()))

const (
	colorBlue   = "\033[1;34m"
	colorCyan   = "\033[1;36m"
	colorGreen  = "\033[1;32m"
	colorRed    = "\033[1;31m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
	colorReset  = "\033[0m"
	colorYellow = "\033[1;33m"
)

func c(str, color string) string {
	if isColorEnabled {
		return color + str + colorReset
	}
	return str
}

func printHelp() {
	fmt.Println(c("Usage:", colorBlue))
	fmt.Println("  ./pbp-tunnel [client|server] [flags]")

	fmt.Println(c("Modes:", colorBlue))
	fmt.Printf("  %s\t%s\n", c("client", colorYellow), "Run the client to establish a reverse SSH tunnel")
	fmt.Printf("  %s\t%s\n", c("server", colorYellow), "Run the server to receive SSH tunnel connections")

	fmt.Println()
	fmt.Println(c("Options:", colorBlue))
	fmt.Printf("  %s\t%s\n", c("-h, --help", colorYellow), "Show this help message")

	fmt.Println()
	fmt.Println(c("To see flags for each mode:", colorBlue))
	fmt.Println("  ./pbp-tunnel client --help")
	fmt.Println("  ./pbp-tunnel server --help")
}

func printClientHelp() {
	fmt.Println(c("Usage:", colorBlue))
	fmt.Println("  ./pbp-tunnel client [flags]")

	fmt.Println(c("Available flags:", colorBlue))
	flag.VisitAll(func(f *flag.Flag) {
		def := f.DefValue
		if def == "" {
			def = "none"
		}
		fmt.Printf("  %s %-20s %s %s\n",
			c("--"+f.Name, colorYellow),
			"",
			f.Usage,
			c(fmt.Sprintf("(default: %s)", def), colorGray),
		)
	})
}

func printServerHelp() {
	fmt.Println(c("Usage:", colorBlue))
	fmt.Println("  ./pbp-tunnel server [flags]\n")

	fmt.Println(c("Available flags:", colorBlue))
	flag.VisitAll(func(f *flag.Flag) {
		def := f.DefValue
		if def == "" {
			def = "none"
		}
		fmt.Printf("  %s %-20s %s %s\n",
			c("--"+f.Name, colorYellow),
			"",
			f.Usage,
			c(fmt.Sprintf("(default: %s)", def), colorGray),
		)
	})
}

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
