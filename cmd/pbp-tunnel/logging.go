package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

const (
	logToConsole        = "console"
	logToFile           = "file"
	logToConsoleAndFile = "both"
)

const (
	defaultLogFile = "/var/log/pbp-tunnel/pbp-tunnel.log"
	logFileName    = "pbp-tunnel.log"
)

func configureLogging(mode, logFilePath string) error {
	output, logFile, err := openLogOutput(mode, logFilePath)
	if err != nil {
		return err
	}
	log.SetOutput(output)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if logFile != nil {
		log.Printf("pbp-tunnel starting - logging to %s", logFile.Name())
	}
	return nil
}

// openLogOutput returns where the log goes, and the log file when the mode uses
// one. The file and its directory are only created by the modes that write to it.
func openLogOutput(mode, logFilePath string) (output io.Writer, logFile *os.File, err error) {
	switch mode {
	case logToConsole:
		return os.Stdout, nil, nil
	case logToFile, logToConsoleAndFile:
		logFile, err := openLogFile(logFilePath)
		if err != nil {
			return nil, nil, err
		}
		if mode == logToFile {
			return logFile, logFile, nil
		}
		return io.MultiWriter(os.Stdout, logFile), logFile, nil
	default:
		return nil, nil, fmt.Errorf("unknown logging mode %q, expected console, file or both", mode)
	}
}

// openLogFile appends to path. When path is an existing directory, the log goes
// to pbp-tunnel.log inside it, as -logfile used to name a directory.
func openLogFile(path string) (*os.File, error) {
	if path == "" {
		path = defaultLogFile
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, logFileName)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return logFile, nil
}
