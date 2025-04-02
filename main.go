package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	var mode string
	if len(os.Args) >= 2 {
		mode = os.Args[1]

		if mode == "client" || mode == "server" {
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		} else {
			mode = ""
		}
	}

	if mode == "" {
		if mode = guessType(); mode == "" {
			fmt.Println("No mode were specified or cannot guess mode from JSON config. Please specify the `type` attribute as 'client' or 'server' in program arguments or JSON config.")
			os.Exit(1)
		}
	}

	switch mode {
	case "client":
		Client(LoadClientConfig())
	case "server":
		server := Server(LoadServerConfig())

		err := server.Start()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown mode '%s'. Use 'client' or 'server' instead", mode)
		os.Exit(1)
	}
}

func generatePrivateKey(filePath string) ([]byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	err = ioutil.WriteFile(filePath, privateKeyPEM, 0600)
	if err != nil {
		return nil, err
	}

	return privateKeyPEM, nil
}

func guessType() string {
	if config := LoadConfig(); config != nil && (config.Type == "client" || config.Type == "server") {
		return config.Type
	}

	return ""
}
