package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

const envVarPrefix = "PBP_TUNNEL_"

// envVarName turns a flag name such as "local-port" into PBP_TUNNEL_LOCAL_PORT.
func envVarName(flagName string) string {
	return envVarPrefix + strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))
}

// applyEnvironment sets every flag whose environment variable is set and not empty.
func applyEnvironment(flags *flag.FlagSet) error {
	var errs []error
	flags.VisitAll(func(f *flag.Flag) {
		name := envVarName(f.Name)
		value := os.Getenv(name)
		if value == "" {
			return
		}
		if err := flags.Set(f.Name, value); err != nil {
			errs = append(errs, fmt.Errorf("%s=%q: %w", name, value, err))
		}
		if list, ok := f.Value.(*ipListFlag); ok {
			list.startNewSource()
		}
	})
	return errors.Join(errs...)
}
