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

func main() {
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	debugFlag := flag.Bool("debug", false, "Enable debug monitoring")
	logging := flag.String("logging", "console", "Logging mode: both, file, console")
	logFile := flag.String("logfile", "", "Path to log file (if logging mode is 'file' or 'both')")

	flag.Usage = printUsage

	flag.Parse()

	setupLogging(*logging, *logFile)

	if *versionFlag {
		fmt.Printf("pbp-tunnel (version %s)\n", Version)
		fmt.Println("Port-tunnelling utility proudly developed by Powered By PumP.")
		return
	}

	if *debugFlag {
		go logRuntimeStatsEvery(10 * time.Second)
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
			printUsage()
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
		runGenerate()
	default:
		log.Fatalf("Unknown command: %s", command)
	}
}
