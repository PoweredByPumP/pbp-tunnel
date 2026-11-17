package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/config"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

// globalOptions come before the command: pbp-tunnel [global flags] <command> [flags].
type globalOptions struct {
	showVersion bool
	debug       bool
	logging     string
	logFile     string
}

func main() {
	globalFlags := flag.NewFlagSet("pbp-tunnel", flag.ExitOnError)
	globalFlags.Usage = func() { printUsage(globalFlags) }
	options, commandLine, _ := parseGlobalFlags(globalFlags, os.Args[1:])

	if options.showVersion {
		fmt.Printf("pbp-tunnel (version %s)\n", Version)
		fmt.Println("Port-tunnelling utility proudly developed by Powered By PumP.")
		return
	}
	setupLogging(options.logging, options.logFile)
	if options.debug {
		go logRuntimeStatsEvery(10 * time.Second)
	}

	command, commandArgs := commandFrom(commandLine)
	switch command {
	case "client":
		runClient(commandArgs)
	case "server":
		runServer(commandArgs)
	case "generate":
		runGenerate()
	case "":
		printUsage(globalFlags)
		os.Exit(1)
	default:
		log.Fatalf("Unknown command: %s", command)
	}
}

// parseGlobalFlags returns the global options and what follows them: the command
// and its own flags.
func parseGlobalFlags(flags *flag.FlagSet, args []string) (globalOptions, []string, error) {
	options := globalOptions{}
	flags.BoolVar(&options.showVersion, "version", false, "Print version information and exit")
	flags.BoolVar(&options.debug, "debug", false, "Log goroutine and memory usage every 10 seconds")
	flags.StringVar(&options.logging, "logging", "console", "Logging mode: console, file or both")
	flags.StringVar(&options.logFile, "logfile", "", "Path to the log file, for the file and both logging modes")
	err := flags.Parse(args)
	return options, flags.Args(), err
}

// commandFrom falls back to the mode of the config file or of PBP_TUNNEL_TYPE
// when the command line names no command.
func commandFrom(commandLine []string) (command string, commandArgs []string) {
	if len(commandLine) > 0 {
		return commandLine[0], commandLine[1:]
	}
	mode, err := config.ReadMode()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}
	switch mode {
	case config.ModeClient, config.ModeServer:
		return string(mode), nil
	default:
		return "", nil
	}
}
