package config

import (
	"slices"
	"strings"
	"testing"
)

func TestStringArray_Set(t *testing.T) {
	tests := []struct {
		name      string
		flagUsage []string
		want      []string
	}{
		{name: "repeated flag", flagUsage: []string{"10.0.0.1", "10.0.0.2"}, want: []string{"10.0.0.1", "10.0.0.2"}},
		{name: "comma-separated", flagUsage: []string{"10.0.0.1,10.0.0.2"}, want: []string{"10.0.0.1", "10.0.0.2"}},
		{name: "spaces around commas", flagUsage: []string{"10.0.0.1, 10.0.0.2 "}, want: []string{"10.0.0.1", "10.0.0.2"}},
		{name: "empty items", flagUsage: []string{"10.0.0.1,,", ""}, want: []string{"10.0.0.1"}},
		{name: "both forms", flagUsage: []string{"10.0.0.1,10.0.0.2", "10.0.0.3"}, want: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var allowedIPs StringArray
			for _, value := range tc.flagUsage {
				if err := allowedIPs.Set(value); err != nil {
					t.Fatalf("Set(%q): %v", value, err)
				}
			}
			if !slices.Equal(allowedIPs, tc.want) {
				t.Errorf("after Set(%q) the list is %q, want %q", tc.flagUsage, allowedIPs, tc.want)
			}
			if got, want := allowedIPs.String(), strings.Join(tc.want, ","); got != want {
				t.Errorf("String() = %q, want %q", got, want)
			}
		})
	}
}
