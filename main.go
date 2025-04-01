package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
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
			fmt.Println("Cannot guess mode from JSON config. Please specify the `type` attribute as 'client' or 'server'.")
			os.Exit(1)
		}
	}

	switch mode {
	case "client":
		Client(loadClientConfig())
	case "server":
		server := Server(loadServerConfig())

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
	if config := loadConfig(); config != nil && (config.Type == "client" || config.Type == "server") {
		return config.Type
	}

	return ""
}

func loadClientConfig() *ClientParameters {
	config := loadConfig()
	if config != nil && config.Type == "client" && config.Client != nil {
		return config.Client
	}

	return nil
}

func loadServerConfig() *ServerParameters {
	config := loadConfig()
	if config != nil && config.Type == "server" && config.Server != nil {
		return config.Server
	}

	return nil
}

func loadConfig() *AppConfig {
	filePath := GetAppProperty("config", "")

	configFile, err := os.Open(filePath)
	if err != nil {
		return nil
	}

	defer configFile.Close()

	configData, err := ioutil.ReadAll(configFile)
	if err != nil {
		return nil
	}

	var config AppConfig
	err = json.Unmarshal(configData, &config)
	if err != nil {
		return nil
	}

	return &config
}
