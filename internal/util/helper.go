package util

import (
	"flag"
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

var isColorEnabled = isatty.IsTerminal(os.Stdout.Fd()) || term.IsTerminal(int(os.Stdout.Fd()))

const (
	colorBlue   = "\033[1;34m"
	colorYellow = "\033[1;33m"
	colorGray   = "\033[90m"
	colorReset  = "\033[0m"
)

// c wraps the given string in ANSI color codes if supported
func c(str, color string) string {
	if isColorEnabled {
		return color + str + colorReset
	}
	return str
}

// PrintHelp prints the global help message
func PrintHelp() {
	fmt.Println(c("Usage:", colorBlue))
	fmt.Println("  pbp-tunnel [client|server|generate] [flags]")

	fmt.Println(c("Modes:", colorBlue))
	fmt.Printf("  %s\t%s\n", c("client", colorYellow), "Run the client to establish a reverse SSH tunnel")
	fmt.Printf("  %s\t%s\n", c("server", colorYellow), "Run the server to receive SSH tunnel connections")
	fmt.Printf("  %s\t%s\n", c("generate", colorYellow), "Generate a configuration template file")

	fmt.Println()
	fmt.Println(c("Options:", colorBlue))
	fmt.Printf("  %s\t%s\n", c("-h", colorYellow), "Show this help message")

	fmt.Println()
	fmt.Println(c("To see flags for each mode:", colorBlue))
	fmt.Println("  pbp-tunnel client --help")
	fmt.Println("  pbp-tunnel server --help")
}

// PrintClientHelp prints the help for the client subcommand
func PrintClientHelp() {
	fmt.Println(c("Usage:", colorBlue))
	fmt.Println("  pbp-tunnel client [flags]")

	fmt.Println(c("Available flags:", colorBlue))
	flag.VisitAll(func(f *flag.Flag) {
		def := f.DefValue
		if def == "" {
			def = "none"
		}
		fmt.Printf("  %s\t%s %s\n",
			c("--"+f.Name, colorYellow),
			f.Usage,
			c(fmt.Sprintf("(default: %s)", def), colorGray),
		)
	})
}

// PrintServerHelp prints the help for the server subcommand
func PrintServerHelp() {
	fmt.Println(c("Usage:", colorBlue))
	fmt.Println("  pbp-tunnel server [flags]")

	fmt.Println(c("Available flags:", colorBlue))
	flag.VisitAll(func(f *flag.Flag) {
		def := f.DefValue
		if def == "" {
			def = "none"
		}
		fmt.Printf("  %s\t%s %s\n",
			c("--"+f.Name, colorYellow),
			f.Usage,
			c(fmt.Sprintf("(default: %s)", def), colorGray),
		)
	})
}
