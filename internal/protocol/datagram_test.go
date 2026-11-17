package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestDatagram_RoundTrip(t *testing.T) {
	for _, datagram := range [][]byte{
		{},
		[]byte("hello"),
		bytes.Repeat([]byte{0xAB}, 1024),
	} {
		t.Run(fmt.Sprintf("%d bytes", len(datagram)), func(t *testing.T) {
			var stream bytes.Buffer
			if err := WriteDatagram(&stream, datagram); err != nil {
				t.Fatalf("WriteDatagram: %v", err)
			}
			got, err := ReadDatagram(&stream)
			if err != nil {
				t.Fatalf("ReadDatagram: %v", err)
			}
			if !bytes.Equal(got, datagram) {
				t.Errorf("ReadDatagram = %x, want %x", got, datagram)
			}
		})
	}
}

func TestDatagram_WireFormat(t *testing.T) {
	var stream bytes.Buffer
	_ = WriteDatagram(&stream, []byte("ping"))

	want := wireBytes(4, "ping")
	if !bytes.Equal(stream.Bytes(), want) {
		t.Errorf("WriteDatagram wrote %x, want %x", stream.Bytes(), want)
	}
}

func TestReadDatagram_FailsOnTruncatedStream(t *testing.T) {
	tests := map[string][]byte{
		"length cut short":  {0, 0},
		"payload cut short": wireBytes(4, "pi"),
	}
	for name, stream := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ReadDatagram(bytes.NewReader(stream))
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("ReadDatagram error = %v, want %v", err, io.ErrUnexpectedEOF)
			}
		})
	}
}
