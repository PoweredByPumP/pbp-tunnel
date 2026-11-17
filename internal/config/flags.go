package config

import (
	"strings"
)

// StringArray is a flag.Stringer implementation for multiple values
// used for JSON unmarshalling and environment parsing
// Represents a list of IPs allowed for forwarding
// (e.g., ["10.0.0.0/8", "192.168.1.1"])
type StringArray []string

func (s *StringArray) String() string {
	return strings.Join(*s, ",")
}

// Set accepts both "--allowed-ips a,b" and a repeated "--allowed-ips a --allowed-ips b".
func (s *StringArray) Set(value string) error {
	*s = append(*s, splitCommaSeparated(value)...)
	return nil
}

func splitCommaSeparated(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
