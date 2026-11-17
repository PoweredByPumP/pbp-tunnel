package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// Every value on the wire is either a big-endian uint32 or a byte string
// prefixed by its length as a uint32.

const noLengthLimit = math.MaxUint32

var errTooLong = errors.New("length exceeds limit")

func writeUint32(w io.Writer, value uint32) error {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	_, err := w.Write(encoded[:])
	return err
}

func readUint32(r io.Reader) (uint32, error) {
	var encoded [4]byte
	if _, err := io.ReadFull(r, encoded[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(encoded[:]), nil
}

func writeStatus(w io.Writer, status Status) error {
	return writeUint32(w, uint32(status))
}

func readStatus(r io.Reader) (Status, error) {
	value, err := readUint32(r)
	return Status(value), err
}

func writeBytes(w io.Writer, data []byte) error {
	if err := writeUint32(w, uint32(len(data))); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	_, err := w.Write(data)
	return err
}

// readBytes refuses a length above maxLength before allocating anything.
func readBytes(r io.Reader, maxLength uint32) ([]byte, error) {
	length, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	if length > maxLength {
		return nil, fmt.Errorf("%w: %d bytes, maximum is %d", errTooLong, length, maxLength)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}
