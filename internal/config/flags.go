package config

import (
	"flag"
	"strings"
)

// Every flag can also be set through an environment variable, see envVarName.

func (p *ClientParameters) registerFlags(flags *flag.FlagSet) {
	flags.StringVar(&p.Endpoint, "endpoint", p.Endpoint, "SSH server endpoint")
	flags.IntVar(&p.EndpointPort, "port", p.EndpointPort, "SSH server port")
	flags.StringVar(&p.Username, "username", p.Username, "SSH username")
	flags.StringVar(&p.Password, "password", p.Password, "SSH password")
	flags.StringVar(&p.PrivateKeyPath, "identity", p.PrivateKeyPath, "Private key path (optional)")
	flags.StringVar(&p.HostKeyPath, "host-key", p.HostKeyPath, "Known host key file (optional)")
	flags.StringVar(&p.LocalHost, "local-host", p.LocalHost, "Local address to forward")
	flags.IntVar(&p.LocalPort, "local-port", p.LocalPort, "Local port to forward")
	flags.StringVar(&p.RemoteHost, "remote-host", p.RemoteHost, "Remote host to expose (unused)")
	flags.IntVar(&p.RemotePort, "remote-port", p.RemotePort, "Remote port to request (0 = random)")
	flags.StringVar(&p.Protocol, "protocol", p.Protocol, "Forwarding protocol (tcp|udp)")
	flags.IntVar(&p.HostKeyLevel, "host-key-level", p.HostKeyLevel, "Host key level (0=no check,1=warn,2=strict)")
	flags.Var(&ipListFlag{list: &p.AllowedIPs}, "allowed-ips", "IPs allowed to use the exposed port (comma-separated)")
}

func (p *ServerParameters) registerFlags(flags *flag.FlagSet) {
	flags.StringVar(&p.BindAddress, "bind", p.BindAddress, "Bind address")
	flags.IntVar(&p.BindPort, "port", p.BindPort, "SSH port")
	flags.IntVar(&p.PortRangeStart, "port-range-start", p.PortRangeStart, "First port clients may expose")
	flags.IntVar(&p.PortRangeEnd, "port-range-end", p.PortRangeEnd, "Last port clients may expose")
	flags.StringVar(&p.Username, "username", p.Username, "SSH username")
	flags.StringVar(&p.Password, "password", p.Password, "SSH password")
	flags.StringVar(&p.PrivateRsaPath, "private-rsa-path", p.PrivateRsaPath, "Path to the RSA host key")
	flags.StringVar(&p.PrivateEcdsaPath, "private-ecdsa-path", p.PrivateEcdsaPath, "Path to the ECDSA host key")
	flags.StringVar(&p.PrivateEd25519Path, "private-ed25519-path", p.PrivateEd25519Path, "Path to the Ed25519 host key")
	flags.StringVar(&p.AuthorizedKeysPath, "authorized-keys-path", p.AuthorizedKeysPath, "Path to authorized_keys")
	flags.Var(&ipListFlag{list: &p.AllowedIPs}, "allowed-ips", "IPs allowed to open a tunnel (comma-separated)")
}

// ipListFlag fills a list of IPs from a flag or an environment variable. The first
// value from a source replaces the list inherited from lower-priority sources; the
// next values from the same source, such as a repeated flag, extend it.
type ipListFlag struct {
	list           *[]string
	extendsCurrent bool
}

func (f *ipListFlag) String() string {
	if f == nil || f.list == nil {
		return ""
	}
	return strings.Join(*f.list, ",")
}

func (f *ipListFlag) Set(value string) error {
	if !f.extendsCurrent {
		*f.list = nil
		f.extendsCurrent = true
	}
	*f.list = append(*f.list, splitCommaSeparated(value)...)
	return nil
}

// startNewSource makes the next value replace the list instead of extending it.
func (f *ipListFlag) startNewSource() {
	f.extendsCurrent = false
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
