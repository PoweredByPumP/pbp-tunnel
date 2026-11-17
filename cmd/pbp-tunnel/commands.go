package main

import (
	"flag"
	"log"
	"os"

	"github.com/poweredbypump/pbp-tunnel/internal/client"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/server"
)

func runClient(args []string) {
	flags := flag.NewFlagSet("client", flag.ExitOnError)
	flags.Usage = func() { printCommandUsage("client", flags) }
	params, err := config.LoadClientParameters(flags, args)
	if err != nil {
		log.Fatalf("Client configuration error: %v", err)
	}
	if err := client.Run(params); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}

func runServer(args []string) {
	flags := flag.NewFlagSet("server", flag.ExitOnError)
	flags.Usage = func() { printCommandUsage("server", flags) }
	params, err := config.LoadServerParameters(flags, args)
	if err != nil {
		log.Fatalf("Server configuration error: %v", err)
	}
	if err := server.Run(params); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runGenerate() {
	if err := config.GenerateConfigFile(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("Error generating config template: %v", err)
	}
}
