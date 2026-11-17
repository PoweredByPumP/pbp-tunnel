package allowlist

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
)

// List is a set of IP addresses and CIDR ranges. An empty List allows every address.
type List struct {
	entries  []string
	networks []netip.Prefix
}

// Parse accepts IP addresses ("10.0.0.1", "::1") and CIDR ranges ("10.0.0.0/8").
func Parse(entries []string) (List, error) {
	list := List{entries: slices.Clone(entries)}
	for _, entry := range entries {
		network, err := parseNetwork(entry)
		if err != nil {
			return List{}, err
		}
		list.networks = append(list.networks, network)
	}
	return list, nil
}

// Entries returns the entries exactly as they were given to Parse.
func (l List) Entries() []string {
	return slices.Clone(l.entries)
}

func (l List) String() string {
	return fmt.Sprint(l.entries)
}

// Allows reports whether the IP of remote belongs to the list.
func (l List) Allows(remote net.Addr) bool {
	if len(l.networks) == 0 {
		return true
	}
	ip, ok := ipOf(remote)
	if !ok {
		return false
	}
	for _, network := range l.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func parseNetwork(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		network, err := netip.ParsePrefix(entry)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("invalid CIDR range %q: %w", entry, err)
		}
		return network.Masked(), nil
	}
	ip, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid IP address %q: %w", entry, err)
	}
	ip = ip.Unmap()
	return netip.PrefixFrom(ip, ip.BitLen()), nil
}

// ipOf extracts the IP of a remote address. IPv4 addresses seen through an
// IPv6 socket (::ffff:10.0.0.1) are turned back into plain IPv4.
func ipOf(remote net.Addr) (netip.Addr, bool) {
	if remote == nil {
		return netip.Addr{}, false
	}
	var ip netip.Addr
	if withAddrPort, ok := remote.(interface{ AddrPort() netip.AddrPort }); ok {
		ip = withAddrPort.AddrPort().Addr()
	} else if addrPort, err := netip.ParseAddrPort(remote.String()); err == nil {
		ip = addrPort.Addr()
	}
	return ip.Unmap().WithZone(""), ip.IsValid()
}
