package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var mode string
	if len(os.Args) >= 2 {
		mode = os.Args[1]

		if mode == "client" || mode == "server" {
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		} else if mode == "-h" || mode == "--help" {
			printHelp()
			os.Exit(0)
		} else {
			mode = ""
		}
	}

	if mode == "" {
		if mode = guessType(); mode == "" {
			fmt.Println("No mode were specified or cannot guess mode from JSON config. Please specify the `type` attribute as 'client' or 'server' in program arguments or JSON config.")
			printHelp()
			os.Exit(1)
		}
	}

	switch mode {
	case "client":
		flag.Usage = printClientHelp
		Client(LoadClientConfig())
	case "server":
		flag.Usage = printServerHelp
		server := Server(LoadServerConfig())

		err := server.Start()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown mode '%s'. Use 'client' or 'server' instead\n", mode)
		printHelp()
		os.Exit(1)
	}
}

func guessType() string {
	if config := LoadConfig(); config != nil && (config.Type == "client" || config.Type == "server") {
		return config.Type
	}

	return ""
}
