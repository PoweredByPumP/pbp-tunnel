package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

type LogMode int

const (
	LogBoth LogMode = iota
	LogFileOnly
	LogConsoleOnly
)

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
