package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

var stdoutIsTerminal = isatty.IsTerminal(os.Stdout.Fd()) || term.IsTerminal(int(os.Stdout.Fd()))

const (
	colorBlue   = "\033[1;34m"
	colorYellow = "\033[1;33m"
	colorGray   = "\033[90m"
	colorReset  = "\033[0m"
)

// colorize leaves text unchanged when stdout is not a terminal, such as a pipe.
func colorize(text, color string) string {
	if stdoutIsTerminal {
		return color + text + colorReset
	}
	return text
}

func printUsage() {
	fmt.Println(colorize("Usage:", colorBlue))
	fmt.Println("  pbp-tunnel [client|server|generate] [flags]")

	fmt.Println(colorize("Modes:", colorBlue))
	fmt.Printf("  %s\t%s\n", colorize("client", colorYellow), "Run the client to establish a reverse SSH tunnel")
	fmt.Printf("  %s\t%s\n", colorize("server", colorYellow), "Run the server to receive SSH tunnel connections")
	fmt.Printf("  %s\t%s\n", colorize("generate", colorYellow), "Generate a configuration template file")

	fmt.Println()
	fmt.Println(colorize("Options:", colorBlue))
	fmt.Printf("  %s\t%s\n", colorize("-h", colorYellow), "Show this help message")

	fmt.Println()
	fmt.Println(colorize("To see flags for each mode:", colorBlue))
	fmt.Println("  pbp-tunnel client --help")
	fmt.Println("  pbp-tunnel server --help")
}

func printCommandUsage(command string, flags *flag.FlagSet) {
	fmt.Println(colorize("Usage:", colorBlue))
	fmt.Printf("  pbp-tunnel %s [flags]\n", command)

	fmt.Println(colorize("Available flags:", colorBlue))
	flags.VisitAll(func(f *flag.Flag) {
		defaultValue := f.DefValue
		if defaultValue == "" {
			defaultValue = "none"
		}
		fmt.Printf("  %s\t%s %s\n",
			colorize("--"+f.Name, colorYellow),
			f.Usage,
			colorize(fmt.Sprintf("(default: %s)", defaultValue), colorGray),
		)
	})
}
