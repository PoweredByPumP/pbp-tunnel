package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenLogOutput_ConsoleModeCreatesNoFile(t *testing.T) {
	logFilePath := filepath.Join(t.TempDir(), "logs", "pbp-tunnel.log")

	output, logFile, err := openLogOutput(logToConsole, logFilePath)

	if err != nil || output != os.Stdout || logFile != nil {
		t.Fatalf("openLogOutput = %v, %v, %v; want stdout and no file", output, logFile, err)
	}
	if _, err := os.Stat(filepath.Dir(logFilePath)); !os.IsNotExist(err) {
		t.Errorf("console mode created the log directory (stat error: %v)", err)
	}
}

func TestOpenLogOutput_FileModesWriteToTheLogFile(t *testing.T) {
	for _, mode := range []string{logToFile, logToConsoleAndFile} {
		t.Run(mode, func(t *testing.T) {
			logFilePath := filepath.Join(t.TempDir(), "logs", "pbp-tunnel.log")

			output, logFile, err := openLogOutput(mode, logFilePath)
			if err != nil {
				t.Fatalf("openLogOutput: %v", err)
			}
			defer logFile.Close()
			if _, err := output.Write([]byte("tunnel opened\n")); err != nil {
				t.Fatal(err)
			}

			if content, _ := os.ReadFile(logFilePath); !strings.Contains(string(content), "tunnel opened") {
				t.Errorf("log file holds %q, want the written line", content)
			}
		})
	}
}

func TestOpenLogOutput_ExistingDirectoryGetsALogFileInside(t *testing.T) {
	logDirectory := t.TempDir()

	_, logFile, err := openLogOutput(logToFile, logDirectory)
	if err != nil {
		t.Fatalf("openLogOutput: %v", err)
	}
	defer logFile.Close()

	if want := filepath.Join(logDirectory, logFileName); logFile.Name() != want {
		t.Errorf("log file = %q, want %q", logFile.Name(), want)
	}
}

func TestOpenLogOutput_RejectsUnknownMode(t *testing.T) {
	_, _, err := openLogOutput("syslog", "")
	if err == nil || !strings.Contains(err.Error(), "unknown logging mode") {
		t.Errorf("openLogOutput error = %v, want an unknown logging mode error", err)
	}
}
