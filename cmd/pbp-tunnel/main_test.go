package main

import (
	"flag"
	"io"
	"slices"
	"testing"
)

func TestParseGlobalFlags_ReadsFlagsBeforeTheCommand(t *testing.T) {
	flags := flag.NewFlagSet("pbp-tunnel", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options, commandLine, err := parseGlobalFlags(flags, []string{"--debug", "--logging", "file", "client", "--endpoint", "tunnel.example.com"})

	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	if !options.debug || options.logging != "file" {
		t.Errorf("options = %+v, want debug and file logging", options)
	}
	if want := []string{"client", "--endpoint", "tunnel.example.com"}; !slices.Equal(commandLine, want) {
		t.Errorf("command line = %q, want %q", commandLine, want)
	}
}

func TestParseGlobalFlags_Defaults(t *testing.T) {
	flags := flag.NewFlagSet("pbp-tunnel", flag.ContinueOnError)

	options, commandLine, err := parseGlobalFlags(flags, []string{"server"})

	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	if options != (globalOptions{logging: "console"}) {
		t.Errorf("options = %+v, want console logging and nothing else", options)
	}
	if !slices.Equal(commandLine, []string{"server"}) {
		t.Errorf("command line = %q, want [server]", commandLine)
	}
}

func TestCommandFrom_SplitsTheCommandFromItsFlags(t *testing.T) {
	command, commandArgs := commandFrom([]string{"client", "--endpoint", "tunnel.example.com"})

	if command != "client" || !slices.Equal(commandArgs, []string{"--endpoint", "tunnel.example.com"}) {
		t.Errorf("commandFrom = %q, %q", command, commandArgs)
	}
}

func TestCommandFrom_FallsBackToTheConfiguredMode(t *testing.T) {
	t.Setenv("PBP_TUNNEL_CONFIG", "")
	t.Chdir(t.TempDir())
	t.Setenv("PBP_TUNNEL_TYPE", "server")

	command, commandArgs := commandFrom(nil)

	if command != "server" || len(commandArgs) != 0 {
		t.Errorf("commandFrom(nil) = %q, %q; want server and no flags", command, commandArgs)
	}
}
