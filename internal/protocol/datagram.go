package protocol

import (
	"fmt"
	"io"
)

// MaxDatagramSize is large enough for any UDP payload.
const MaxDatagramSize = 64 * 1024

// WriteDatagram sends one UDP datagram over an SSH channel, prefixed by its length
// so the other side can split the stream back into datagrams.
func WriteDatagram(w io.Writer, datagram []byte) error {
	if len(datagram) > MaxDatagramSize {
		return fmt.Errorf("%w: datagram of %d bytes, maximum is %d", errLimitExceeded, len(datagram), MaxDatagramSize)
	}
	return writeBytes(w, datagram)
}

func ReadDatagram(r io.Reader) ([]byte, error) {
	return readBytes(r, MaxDatagramSize)
}
