package protocol

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// A tunnel request runs on the first channel the client opens:
//
//	server → client  status          StatusOK, or StatusPeerNotAllowed
//	client → server  allowed peers   count, then each peer as a byte string
//	server → client  status          StatusOK
//	client → server  transport       "tcp" or "udp" as a byte string
//	client → server  requested port  0 lets the server choose
//	server → client  port response   the exposed port, or refusedFlag | Status

type Transport string

const (
	TCP Transport = "tcp"
	UDP Transport = "udp"
)

const maxTransportNameLength = 16

// refusedFlag marks a port response as a refusal; the remaining bits hold the Status.
const refusedFlag uint32 = 0x80000000

type TunnelRequest struct {
	// AllowedPeers lists the IPs or CIDRs allowed to connect to the exposed port.
	// An empty list allows everyone.
	AllowedPeers []string
	Transport    Transport
	// Port is the port the client wants exposed; 0 lets the server choose.
	Port int
}

func ParseTransport(name string) (Transport, error) {
	switch transport := Transport(strings.ToLower(strings.TrimSpace(name))); transport {
	case TCP, UDP:
		return transport, nil
	default:
		return "", fmt.Errorf("%w: %q", StatusUnsupportedTransport, name)
	}
}

// RequestTunnel runs the client side of a tunnel request and returns the port the
// server exposed. A refusal from the server is returned as a Status error.
func RequestTunnel(conn io.ReadWriter, request TunnelRequest) (exposedPort int, err error) {
	if err := expectStatusOK(conn, "greeting"); err != nil {
		return 0, err
	}
	if err := writeAllowedPeers(conn, request.AllowedPeers); err != nil {
		return 0, fmt.Errorf("send allowed peers: %w", err)
	}
	if err := expectStatusOK(conn, "allowed peers acknowledgement"); err != nil {
		return 0, err
	}
	if err := writeBytes(conn, []byte(request.Transport)); err != nil {
		return 0, fmt.Errorf("send transport: %w", err)
	}
	if err := writeUint32(conn, uint32(request.Port)); err != nil {
		return 0, fmt.Errorf("send requested port: %w", err)
	}
	return readPortResponse(conn)
}

// RejectPeer answers a client whose IP is not allowed to request a tunnel.
func RejectPeer(w io.Writer) error {
	return writeStatus(w, StatusPeerNotAllowed)
}

// ReceiveTunnelRequest greets an allowed client and reads its tunnel request.
// An unsupported transport is refused before the error is returned. Otherwise the
// caller must answer with AcceptTunnel or RefuseTunnel.
func ReceiveTunnelRequest(conn io.ReadWriter) (TunnelRequest, error) {
	if err := writeStatus(conn, StatusOK); err != nil {
		return TunnelRequest{}, fmt.Errorf("send greeting: %w", err)
	}
	allowedPeers, err := readAllowedPeers(conn)
	if err != nil {
		return TunnelRequest{}, fmt.Errorf("read allowed peers: %w", err)
	}
	if err := writeStatus(conn, StatusOK); err != nil {
		return TunnelRequest{}, fmt.Errorf("acknowledge allowed peers: %w", err)
	}
	transport, err := readTransport(conn)
	if errors.Is(err, StatusUnsupportedTransport) {
		_ = RefuseTunnel(conn, StatusUnsupportedTransport)
	}
	if err != nil {
		return TunnelRequest{}, fmt.Errorf("read transport: %w", err)
	}
	port, err := readUint32(conn)
	if err != nil {
		return TunnelRequest{}, fmt.Errorf("read requested port: %w", err)
	}
	return TunnelRequest{AllowedPeers: allowedPeers, Transport: transport, Port: int(port)}, nil
}

func AcceptTunnel(w io.Writer, exposedPort int) error {
	return writeUint32(w, uint32(exposedPort))
}

// RefuseTunnel sends reason to the client when it is a Status, and
// StatusInternalError for any other error.
func RefuseTunnel(w io.Writer, reason error) error {
	status := StatusInternalError
	errors.As(reason, &status)
	return writeUint32(w, refusedFlag|uint32(status))
}

func expectStatusOK(r io.Reader, step string) error {
	status, err := readStatus(r)
	if err != nil {
		return fmt.Errorf("read %s: %w", step, err)
	}
	if status != StatusOK {
		return fmt.Errorf("%s: %w", step, status)
	}
	return nil
}

func writeAllowedPeers(w io.Writer, allowedPeers []string) error {
	if err := writeUint32(w, uint32(len(allowedPeers))); err != nil {
		return err
	}
	for _, peer := range allowedPeers {
		if err := writeBytes(w, []byte(peer)); err != nil {
			return err
		}
	}
	return nil
}

func readAllowedPeers(r io.Reader) ([]string, error) {
	count, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	allowedPeers := make([]string, 0, count)
	for range count {
		peer, err := readBytes(r, noLengthLimit)
		if err != nil {
			return nil, err
		}
		allowedPeers = append(allowedPeers, string(peer))
	}
	return allowedPeers, nil
}

func readTransport(r io.Reader) (Transport, error) {
	name, err := readBytes(r, maxTransportNameLength)
	if errors.Is(err, errTooLong) {
		return "", fmt.Errorf("%w: %w", StatusUnsupportedTransport, err)
	}
	if err != nil {
		return "", err
	}
	return ParseTransport(string(name))
}

func readPortResponse(r io.Reader) (int, error) {
	response, err := readUint32(r)
	if err != nil {
		return 0, fmt.Errorf("read port response: %w", err)
	}
	if response&refusedFlag != 0 {
		return 0, Status(response &^ refusedFlag)
	}
	return int(response), nil
}
