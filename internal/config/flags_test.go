package config

import (
	"flag"
	"io"
	"slices"
	"testing"
)

func TestIPListFlag_RepeatedAndCommaSeparatedValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "repeated flag", args: []string{"--allowed-ips", "10.0.0.1", "--allowed-ips", "10.0.0.2"}, want: []string{"10.0.0.1", "10.0.0.2"}},
		{name: "comma-separated", args: []string{"--allowed-ips", "10.0.0.1,10.0.0.2"}, want: []string{"10.0.0.1", "10.0.0.2"}},
		{name: "spaces around commas", args: []string{"--allowed-ips", "10.0.0.1, 10.0.0.2 "}, want: []string{"10.0.0.1", "10.0.0.2"}},
		{name: "empty items", args: []string{"--allowed-ips", "10.0.0.1,,", "--allowed-ips", ""}, want: []string{"10.0.0.1"}},
		{name: "both forms", args: []string{"--allowed-ips", "10.0.0.1,10.0.0.2", "--allowed-ips", "10.0.0.3"}, want: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := ServerParameters{AllowedIPs: []string{"192.0.2.1"}}
			flags := newSilentFlagSet()
			params.registerFlags(flags)

			if err := flags.Parse(tc.args); err != nil {
				t.Fatalf("Parse(%q): %v", tc.args, err)
			}
			if !slices.Equal(params.AllowedIPs, tc.want) {
				t.Errorf("AllowedIPs = %q, want %q", params.AllowedIPs, tc.want)
			}
		})
	}
}

func newSilentFlagSet() *flag.FlagSet {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}
