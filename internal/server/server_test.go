package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"testing"
)

// --- Tests for assignPort ---
func TestAssignPort_SpecificValid(t *testing.T) {
	forwards := make(map[int]struct{})
	var lock sync.Mutex
	port, mask := assignPort(1500, 1500, 1502, forwards, &lock)
	if port != 1500 || mask != 0 {
		t.Fatalf("expected port=1500 mask=0, got port=%d mask=%d", port, mask)
	}
	if _, ok := forwards[1500]; !ok {
		t.Errorf("port 1500 should be marked used")
	}
}

func TestAssignPort_SpecificUnavailable(t *testing.T) {
	forwards := map[int]struct{}{1500: {}}
	var lock sync.Mutex
	port, mask := assignPort(1500, 1500, 1502, forwards, &lock)
	if port != 0 || mask&(ErrMask|ErrPortUnavailable) == 0 {
		t.Errorf("expected unavailable mask on duplicate assign, got port=%d mask=%08x", port, mask)
	}
}

func TestAssignPort_OutOfRange(t *testing.T) {
	forwards := make(map[int]struct{})
	var lock sync.Mutex
	port, mask := assignPort(1400, 1500, 1502, forwards, &lock)
	if port != 0 || mask&(ErrMask|ErrPortOutOfRange) == 0 {
		t.Errorf("expected out-of-range mask, got port=%d mask=%08x", port, mask)
	}
}

func TestAssignPort_AutoPick(t *testing.T) {
	forwards := map[int]struct{}{1500: {}, 1501: {}}
	var lock sync.Mutex
	port, mask := assignPort(0, 1500, 1502, forwards, &lock)
	if port != 1502 || mask != 0 {
		t.Errorf("expected auto-pick 1502, got port=%d mask=%d", port, mask)
	}
}

func TestAssignPort_NoneAvailable(t *testing.T) {
	forwards := map[int]struct{}{1500: {}, 1501: {}, 1502: {}}
	var lock sync.Mutex
	port, mask := assignPort(0, 1500, 1502, forwards, &lock)
	if port != 0 || mask&(ErrMask|ErrPortUnavailable) == 0 {
		t.Errorf("expected none-available mask, got port=%d mask=%08x", port, mask)
	}
}

func TestAssignPort_InvalidRange(t *testing.T) {
	forwards := make(map[int]struct{})
	var lock sync.Mutex
	port, mask := assignPort(0, 2000, 1000, forwards, &lock)
	if port != 0 || mask&(ErrMask|ErrPortUnavailable) == 0 {
		t.Errorf("expected invalid-range mask, got port=%d mask=%08x", port, mask)
	}
}

// --- Tests for processHandshake ---

type stubRW struct {
	buf        *bytes.Buffer
	written    []uint32
	readCount  int
	errorAfter int // after how many Read calls to error
}

func newStubRW(entries []string, errorAfter int) *stubRW {
	buf := &bytes.Buffer{}
	// preload count and entries
	_ = binary.Write(buf, binary.BigEndian, uint32(len(entries)))
	for _, e := range entries {
		_ = binary.Write(buf, binary.BigEndian, uint32(len(e)))
		buf.WriteString(e)
	}
	return &stubRW{buf: buf, errorAfter: errorAfter}
}

func (s *stubRW) Read(p []byte) (int, error) {
	s.readCount++
	if s.errorAfter >= 0 && s.readCount > s.errorAfter {
		return 0, errors.New("read error")
	}
	return s.buf.Read(p)
}

func (s *stubRW) Write(p []byte) (int, error) {
	if len(p) >= 4 {
		code := binary.BigEndian.Uint32(p[:4])
		s.written = append(s.written, code)
	}
	return len(p), nil
}

func (s *stubRW) Close() error { return nil }

func TestProcessHandshake_SuccessWithEntries(t *testing.T) {
	entries := []string{"127.0.0.1", "10.0.0.0/8"}
	rw := newStubRW(entries, -1)
	got, err := processHandshake(rw, "127.0.0.1", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(entries) {
		t.Errorf("expected %d entries, got %d", len(entries), len(got))
	}
	if len(rw.written) < 2 || rw.written[0] != ErrSuccess || rw.written[1] != ErrSuccess {
		t.Errorf("expected two ErrSuccess writes, got %v", rw.written)
	}
}

func TestProcessHandshake_NoEntries(t *testing.T) {
	rw := newStubRW(nil, -1)
	got, err := processHandshake(rw, "1.2.3.4", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected zero entries, got %d", len(got))
	}
	if len(rw.written) < 2 || rw.written[0] != ErrSuccess || rw.written[1] != ErrSuccess {
		t.Errorf("expected two ErrSuccess writes, got %v", rw.written)
	}
}

func TestProcessHandshake_IPNotAllowed(t *testing.T) {
	rw := newStubRW(nil, -1)
	_, err := processHandshake(rw, "8.8.8.8", []string{"9.9.9.9"})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected IP not allowed error, got %v", err)
	}
	if len(rw.written) == 0 || rw.written[0] != ErrIPNotAllowed {
		t.Errorf("expected ErrIPNotAllowed write, got %v", rw.written)
	}
}

func TestProcessHandshake_CountReadError(t *testing.T) {
	rw := newStubRW(nil, 0) // error on first Read (count)
	_, err := processHandshake(rw, "127.0.0.1", nil)
	if err == nil || !strings.Contains(err.Error(), "read whitelist count") {
		t.Errorf("expected read count error, got %v", err)
	}
}

func TestProcessHandshake_EntryLengthReadError(t *testing.T) {
	entries := []string{"a"}
	rw := newStubRW(entries, 1) // error on second Read (first read = count OK)
	_, err := processHandshake(rw, "127.0.0.1", nil)
	if err == nil || !strings.Contains(err.Error(), "read whitelist entry length") {
		t.Errorf("expected entry length read error, got %v", err)
	}
}
