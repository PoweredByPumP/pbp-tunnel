package allowlist

import (
	"net"
	"slices"
	"testing"
)

func TestParse_AcceptsIPAddressesAndCIDRRanges(t *testing.T) {
	entries := []string{"10.0.0.1", "192.168.1.0/24", "::1", "fe80::/64", "0.0.0.0/0"}

	list, err := Parse(entries)
	if err != nil {
		t.Fatalf("Parse(%q): %v", entries, err)
	}
	if !slices.Equal(list.Entries(), entries) {
		t.Errorf("Entries() = %q, want %q", list.Entries(), entries)
	}
}

func TestParse_RejectsInvalidEntries(t *testing.T) {
	for _, entry := range []string{
		"",
		"localhost",
		"300.1.1.1",
		"10.0.0",
		"10.0.0.1,10.0.0.2",
		"192.168.1.1/33",
		"192.168.1.1/-1",
		"192.168.1.1/",
		"/24",
	} {
		t.Run(entry, func(t *testing.T) {
			if _, err := Parse([]string{"10.0.0.1", entry}); err == nil {
				t.Errorf("Parse accepted invalid entry %q", entry)
			}
		})
	}
}

func TestList_Allows(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		remote  net.Addr
		want    bool
	}{
		{name: "empty list allows everyone", entries: nil, remote: tcpAddr("203.0.113.10"), want: true},
		{name: "exact IP", entries: []string{"203.0.113.10"}, remote: tcpAddr("203.0.113.10"), want: true},
		{name: "other IP", entries: []string{"203.0.113.10"}, remote: tcpAddr("203.0.113.11"), want: false},
		{name: "inside CIDR range", entries: []string{"192.168.1.0/24"}, remote: tcpAddr("192.168.1.42"), want: true},
		{name: "first address of range", entries: []string{"192.168.1.0/24"}, remote: tcpAddr("192.168.1.0"), want: true},
		{name: "last address of range", entries: []string{"192.168.1.0/24"}, remote: tcpAddr("192.168.1.255"), want: true},
		{name: "just outside range", entries: []string{"192.168.1.0/24"}, remote: tcpAddr("192.168.2.0"), want: false},
		{name: "range written with host bits", entries: []string{"192.168.1.77/24"}, remote: tcpAddr("192.168.1.3"), want: true},
		{name: "matches any entry", entries: []string{"10.0.0.0/24", "172.16.0.0/16", "192.168.1.1"}, remote: tcpAddr("172.16.9.9"), want: true},
		{name: "matches no entry", entries: []string{"10.0.0.0/24", "172.16.0.0/16"}, remote: tcpAddr("8.8.8.8"), want: false},
		{name: "IPv6 range", entries: []string{"fe80::/64"}, remote: tcpAddr("fe80::1"), want: true},
		{name: "IPv6 address against IPv4 list", entries: []string{"10.0.0.0/8"}, remote: tcpAddr("::1"), want: false},
		{name: "IPv4 seen through an IPv6 socket", entries: []string{"10.0.0.1"}, remote: tcpAddr("::ffff:10.0.0.1"), want: true},
		{name: "UDP address", entries: []string{"10.0.0.0/8"}, remote: &net.UDPAddr{IP: net.ParseIP("10.1.2.3"), Port: 53}, want: true},
		{name: "address without IP", entries: []string{"10.0.0.0/8"}, remote: unparsableAddr{}, want: false},
		{name: "nil address", entries: []string{"10.0.0.0/8"}, remote: nil, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			list, err := Parse(tc.entries)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.entries, err)
			}
			if got := list.Allows(tc.remote); got != tc.want {
				t.Errorf("Allows(%v) with %q = %v, want %v", tc.remote, tc.entries, got, tc.want)
			}
		})
	}
}

func TestList_ZeroValueAllowsEveryone(t *testing.T) {
	var list List
	if !list.Allows(tcpAddr("203.0.113.10")) {
		t.Error("zero List refused an address")
	}
}

func tcpAddr(ip string) *net.TCPAddr {
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: 40000}
}

type unparsableAddr struct{}

func (unparsableAddr) Network() string { return "tcp" }
func (unparsableAddr) String() string  { return "not-an-ip" }
