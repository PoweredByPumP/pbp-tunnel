package config

import (
	"flag"
	"os"
)

// Parameters come from four sources, each one overriding the previous:
// the defaults, the config file, PBP_TUNNEL_* environment variables, and flags.

// LoadClientParameters registers the client flags on flags, so that its usage
// lists them, then reads every source with args as the command line.
func LoadClientParameters(flags *flag.FlagSet, args []string) (*ClientParameters, error) {
	params := DefaultClientParameters()
	params.registerFlags(flags)
	if err := loadSources(&AppConfig{Client: params}, flags, args); err != nil {
		return nil, err
	}
	return params, nil
}

// LoadServerParameters works like LoadClientParameters for the server.
func LoadServerParameters(flags *flag.FlagSet, args []string) (*ServerParameters, error) {
	params := DefaultServerParameters()
	params.registerFlags(flags)
	if err := loadSources(&AppConfig{Server: params}, flags, args); err != nil {
		return nil, err
	}
	return params, nil
}

// ReadMode returns the mode set by PBP_TUNNEL_TYPE, or else by the config file.
func ReadMode() (Mode, error) {
	if mode := os.Getenv(envVarName("type")); mode != "" {
		return Mode(mode), nil
	}
	var file AppConfig
	if err := loadConfigFile(&file); err != nil {
		return "", err
	}
	return file.Type, nil
}

func loadSources(file *AppConfig, flags *flag.FlagSet, args []string) error {
	if err := loadConfigFile(file); err != nil {
		return err
	}
	if err := applyEnvironment(flags); err != nil {
		return err
	}
	return flags.Parse(args)
}
