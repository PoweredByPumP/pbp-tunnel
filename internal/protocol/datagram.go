package protocol

import "io"

// WriteDatagram sends one UDP datagram over an SSH channel, prefixed by its length
// so the other side can split the stream back into datagrams.
func WriteDatagram(w io.Writer, datagram []byte) error {
	return writeBytes(w, datagram)
}

func ReadDatagram(r io.Reader) ([]byte, error) {
	return readBytes(r, noLengthLimit)
}
