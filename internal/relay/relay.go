package relay

import (
	"io"
	"sync"
)

// HalfCloser is a connection that can stop sending while it keeps receiving,
// such as *net.TCPConn or ssh.Channel.
type HalfCloser interface {
	io.ReadWriter
	CloseWrite() error
}

// CopyBothWays copies a to b and b to a until both directions are done. When one
// side stops sending, the other side is told through CloseWrite, so a peer that
// waits for the end of a request still gets it.
func CopyBothWays(a, b HalfCloser) (copiedAToB, copiedBToA int64) {
	var bothDirections sync.WaitGroup
	bothDirections.Go(func() {
		copiedAToB, _ = io.Copy(b, a)
		_ = b.CloseWrite()
	})
	bothDirections.Go(func() {
		copiedBToA, _ = io.Copy(a, b)
		_ = a.CloseWrite()
	})
	bothDirections.Wait()
	return copiedAToB, copiedBToA
}
