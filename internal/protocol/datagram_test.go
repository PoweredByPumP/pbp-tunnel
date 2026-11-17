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

func TestDatagram_RefusesOversizedDatagrams(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		_, err := ReadDatagram(bytes.NewReader(wireBytes(MaxDatagramSize + 1)))
		if !errors.Is(err, errLimitExceeded) {
			t.Errorf("ReadDatagram error = %v, want %v", err, errLimitExceeded)
		}
	})

	t.Run("write", func(t *testing.T) {
		var stream bytes.Buffer
		err := WriteDatagram(&stream, make([]byte, MaxDatagramSize+1))
		if !errors.Is(err, errLimitExceeded) {
			t.Errorf("WriteDatagram error = %v, want %v", err, errLimitExceeded)
		}
		if stream.Len() != 0 {
			t.Errorf("WriteDatagram wrote %d bytes for a refused datagram", stream.Len())
		}
	})

	t.Run("largest allowed", func(t *testing.T) {
		var stream bytes.Buffer
		datagram := make([]byte, MaxDatagramSize)
		if err := WriteDatagram(&stream, datagram); err != nil {
			t.Fatalf("WriteDatagram: %v", err)
		}
		if got, err := ReadDatagram(&stream); err != nil || len(got) != MaxDatagramSize {
			t.Errorf("ReadDatagram = %d bytes, %v; want %d bytes", len(got), err, MaxDatagramSize)
		}
	})
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
