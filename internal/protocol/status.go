package protocol

import "fmt"

// Status is the code a server answers with at each step of a tunnel request.
// Every status except StatusOK is a refusal and can be returned as an error.
type Status uint32

const (
	StatusOK                   Status = 0
	StatusPortUnavailable      Status = 1
	StatusPeerNotAllowed       Status = 2
	StatusPortOutOfRange       Status = 3
	StatusInternalError        Status = 4
	StatusUnsupportedTransport Status = 5
)

func (s Status) Error() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusPortUnavailable:
		return "no free port available on the server"
	case StatusPeerNotAllowed:
		return "client IP not allowed by the server"
	case StatusPortOutOfRange:
		return "requested port is outside the server port range"
	case StatusInternalError:
		return "internal server error"
	case StatusUnsupportedTransport:
		return "unsupported transport"
	default:
		return fmt.Sprintf("unknown status %d", uint32(s))
	}
}
