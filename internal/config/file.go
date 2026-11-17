package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

const defaultConfigFile = "config.json"

// loadConfigFile fills target from the file named by PBP_TUNNEL_CONFIG, or from
// config.json when that variable is not set and the file exists. Fields missing
// from the file keep the value they already had in target.
func loadConfigFile(target *AppConfig) error {
	path := os.Getenv(envVarName("config"))
	pathIsExplicit := path != ""
	if !pathIsExplicit {
		path = defaultConfigFile
	}

	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !pathIsExplicit {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}
	return nil
}
