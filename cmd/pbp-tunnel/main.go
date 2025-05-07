package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/poweredbypump/pbp-tunnel/internal/client"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/server"
	"github.com/poweredbypump/pbp-tunnel/internal/util"
)

var Version = "dev"

func main() {
	// Version flag
	versionFlag := flag.Bool("version", false, "Print version information and exit")

	// Default help usage
	flag.Usage = util.PrintHelp

	// Parse general pbp-tunnel command line flags
	flag.Parse()
	if *versionFlag {
		fmt.Printf("pbp-tunnel (version %s)\n", Version)
		fmt.Println("Port-tunnelling utility proudly developed by Powered By PumP.")
		return
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Determine execution mode: subcommand or config file
	if len(os.Args) < 2 {
		cfg := config.LoadConfig()
		switch cfg.Type {
		case "client":
			if cfg.Client == nil {
				log.Fatal("client configuration missing in config file")
			}
			if err := cfg.Client.Validate(); err != nil {
				log.Fatalf("invalid client config: %v", err)
			}
			if err := client.Run(cfg.Client); err != nil {
				log.Fatalf("Client error: %v", err)
			}
			return

		case "server":
			if cfg.Server == nil {
				log.Fatal("server configuration missing in config file")
			}
			if err := cfg.Server.Validate(); err != nil {
				log.Fatalf("invalid server config: %v", err)
			}
			if err := server.Run(cfg.Server); err != nil {
				log.Fatalf("Server error: %v", err)
			}
			return

		default:
			util.PrintHelp()
			os.Exit(1)
		}
	}

	// Extract subcommand and adjust args
	cmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch cmd {
	case "client":
		flag.Usage = util.PrintClientHelp
		cfg := config.LoadClientConfig()
		if err := cfg.Validate(); err != nil {
			log.Fatalf("invalid client config: %v", err)
		}
		if err := client.Run(cfg); err != nil {
			log.Fatalf("Client error: %v", err)
		}

	case "server":
		flag.Usage = util.PrintServerHelp
		cfg := config.LoadServerConfig()
		if err := cfg.Validate(); err != nil {
			log.Fatalf("invalid server config: %v", err)
		}
		if err := server.Run(cfg); err != nil {
			log.Fatalf("Server error: %v", err)
		}

	case "generate":
		if err := config.GenerateConfigTemplate(); err != nil {
			log.Fatalf("Error generating config template: %v", err)
		}

	default:
		log.Fatalf("Unknown command: %s", cmd)
	}
}
