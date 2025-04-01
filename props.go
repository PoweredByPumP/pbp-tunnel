package main

import (
	"os"
	"strings"
)

func GetAppProperty(key, defaultValue string) string {
	key = "PBP_" + strings.ReplaceAll(strings.ToUpper(key), "-", "_")

	envValue, found := os.LookupEnv(key)
	if found {
		return envValue
	}

	return defaultValue
}
