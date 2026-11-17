package server

import (
	"sync"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
)

// portPool hands out the ports clients may expose, at most one tunnel per port.
type portPool struct {
	first, last int

	mu    sync.Mutex
	inUse map[int]bool
}

func newPortPool(first, last int) *portPool {
	return &portPool{first: first, last: last, inUse: make(map[int]bool)}
}

// Reserve takes the requested port, or the first free one when requested is 0.
// A refusal is returned as a protocol.Status.
func (p *portPool) Reserve(requested int) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.first > p.last {
		return 0, protocol.StatusPortUnavailable
	}
	if requested != 0 {
		if requested < p.first || requested > p.last {
			return 0, protocol.StatusPortOutOfRange
		}
		if p.inUse[requested] {
			return 0, protocol.StatusPortUnavailable
		}
		p.inUse[requested] = true
		return requested, nil
	}
	for port := p.first; port <= p.last; port++ {
		if !p.inUse[port] {
			p.inUse[port] = true
			return port, nil
		}
	}
	return 0, protocol.StatusPortUnavailable
}

func (p *portPool) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inUse, port)
}
