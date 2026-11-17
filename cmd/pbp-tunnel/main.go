package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/client"
	"github.com/poweredbypump/pbp-tunnel/internal/config"
	"github.com/poweredbypump/pbp-tunnel/internal/server"
	"github.com/poweredbypump/pbp-tunnel/internal/util"
)

var Version = "dev"

type LogMode int

const (
	LogBoth LogMode = iota
	LogFileOnly
	LogConsoleOnly
)

// main is the entry point of the pbp-tunnel application.
// It parses command-line flags, sets up logging, and routes to appropriate subcommands.
func main() {
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	debugFlag := flag.Bool("debug", false, "Enable debug monitoring")
	logging := flag.String("logging", "console", "Logging mode: both, file, console")
	logFile := flag.String("logfile", "", "Path to log file (if logging mode is 'file' or 'both')")

	flag.Usage = util.PrintHelp

	flag.Parse()

	setupLogging(*logging, *logFile)

	if *versionFlag {
		fmt.Printf("pbp-tunnel (version %s)\n", Version)
		fmt.Println("Port-tunnelling utility proudly developed by Powered By PumP.")
		return
	}

	if *debugFlag {
		go monitorGoroutines()
	}

	if len(os.Args) < 2 {
		mode, err := config.ReadMode()
		if err != nil {
			log.Fatalf("Configuration error: %v", err)
		}
		switch mode {
		case config.ModeClient:
			runClient(nil)
		case config.ModeServer:
			runServer(nil)
		default:
			util.PrintHelp()
			os.Exit(1)
		}
		return
	}

	switch command := os.Args[1]; command {
	case "client":
		runClient(os.Args[2:])
	case "server":
		runServer(os.Args[2:])
	case "generate":
		if err := config.GenerateConfigFile(os.Stdin, os.Stdout); err != nil {
			log.Fatalf("Error generating config template: %v", err)
		}
	default:
		log.Fatalf("Unknown command: %s", command)
	}
}

func runClient(args []string) {
	flags := flag.NewFlagSet("client", flag.ExitOnError)
	flags.Usage = func() { util.PrintSubcommandHelp("client", flags) }
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
	flags.Usage = func() { util.PrintSubcommandHelp("server", flags) }
	params, err := config.LoadServerParameters(flags, args)
	if err != nil {
		log.Fatalf("Server configuration error: %v", err)
	}
	if err := server.Run(params); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// monitorGoroutines periodically logs the number of active goroutines and memory usage.
// This function runs as a goroutine when debug mode is enabled.
func monitorGoroutines() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		numGoroutines := runtime.NumGoroutine()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		log.Printf("DEBUG: Goroutines count: %d, Memory used: %d KB",
			numGoroutines, m.Alloc/1024)
	}
}

// setupLogging configures the logging output based on the specified mode and log file path.
// Parameters:
//   - quietMode: logging mode ("file", "console", or "both")
//   - logDirOverride: custom log directory path (uses default if empty)
func setupLogging(quietMode string, logDirOverride string) {
	var mode LogMode

	switch quietMode {
	case "file":
		mode = LogFileOnly
	case "console":
		mode = LogConsoleOnly
	default:
		mode = LogBoth
	}

	logDir := logDirOverride
	if logDir == "" {
		logDir = "/var/log/pbp-tunnel"
	}

	cleanLogDir := filepath.Clean(logDir)

	if err := os.MkdirAll(cleanLogDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	logFile := filepath.Join(cleanLogDir, "pbp-tunnel.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	switch mode {
	case LogFileOnly:
		log.SetOutput(file)
	case LogConsoleOnly:
		log.SetOutput(os.Stdout)
	default: // LogBoth
		multiWriter := io.MultiWriter(os.Stdout, file)
		log.SetOutput(multiWriter)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if mode != LogConsoleOnly && mode != LogFileOnly {
		log.Printf("pbp-tunnel starting - logging to %s", logFile)
	}
}
