package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestAssignPort_SpecificPortRequest(t *testing.T) {
	tests := []struct {
		name     string
		reqPort  int
		start    int
		end      int
		forwards map[int]struct{}
		wantPort int
		wantMask uint32
	}{
		{
			name:     "port available in range",
			reqPort:  8080,
			start:    8000,
			end:      9000,
			forwards: map[int]struct{}{},
			wantPort: 8080,
			wantMask: 0,
		},
		{
			name:     "port already in use",
			reqPort:  8080,
			start:    8000,
			end:      9000,
			forwards: map[int]struct{}{8080: {}},
			wantPort: 0,
			wantMask: ErrMask | ErrPortUnavailable,
		},
		{
			name:     "port out of range",
			reqPort:  7000,
			start:    8000,
			end:      9000,
			forwards: map[int]struct{}{},
			wantPort: 0,
			wantMask: ErrMask | ErrPortOutOfRange,
		},
		{
			name:     "invalid range",
			reqPort:  8080,
			start:    9000, // start > end (invalid)
			end:      8000,
			forwards: map[int]struct{}{},
			wantPort: 0,
			wantMask: ErrMask | ErrPortUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lock := &sync.Mutex{}
			port, mask := assignPort(tc.reqPort, tc.start, tc.end, tc.forwards, lock)
			if port != tc.wantPort || mask != tc.wantMask {
				t.Errorf("assignPort with specific port request (%d, %d, %d) = (%d, %d); want (%d, %d)",
					tc.reqPort, tc.start, tc.end, port, mask, tc.wantPort, tc.wantMask)
			}
		})
	}
}

func TestAssignPort_AutomaticAssignment(t *testing.T) {
	forwards := make(map[int]struct{})
	lock := &sync.Mutex{}

	// Automatic assignment (reqPort = 0)
	port, mask := assignPort(0, 8000, 9000, forwards, lock)
	if port != 8000 || mask != 0 {
		t.Errorf("assignPort(0) = (%d, %d); want (8000, 0)", port, mask)
	}

	// Fill range and test exhaustion
	for i := 8001; i <= 9000; i++ {
		forwards[i] = struct{}{}
	}

	port, mask = assignPort(0, 8000, 9000, forwards, lock)
	if port != 0 || mask != (ErrMask|ErrPortUnavailable) {
		t.Errorf("assignPort with full range = (%d, %d); want (0, %d)", port, mask, ErrMask|ErrPortUnavailable)
	}
}

func TestAssignPort_ConcurrentSafety(t *testing.T) {
	forwards := make(map[int]struct{})
	var lock sync.Mutex

	const workers = 10
	const requestsPerWorker = 100

	var wg sync.WaitGroup
	wg.Add(workers)

	results := make([][]int, workers)

	for i := 0; i < workers; i++ {
		results[i] = make([]int, 0, requestsPerWorker)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				port, mask := assignPort(0, 10000, 20000, forwards, &lock)
				if mask == 0 && port != 0 {
					results[workerID] = append(results[workerID], port)
				}
			}
		}(i)
	}

	wg.Wait()

	// Check all returned ports are unique
	allPorts := make(map[int]struct{})
	for _, workerResults := range results {
		for _, port := range workerResults {
			if _, exists := allPorts[port]; exists {
				t.Errorf("port %d was assigned multiple times", port)
			}
			allPorts[port] = struct{}{}
		}
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

func TestProcessHandshake_WithWhitelist(t *testing.T) {
	// Setup whitelist data with multiple entries
	entries := []string{"10.0.0.1", "192.168.1.0/24"}
	rw := newStubRW(entries, -1)

	got, err := processHandshake(rw, "192.168.1.5", []string{})

	if err != nil {
		t.Fatalf("processHandshake returned error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
	if got[0] != "10.0.0.1" || got[1] != "192.168.1.0/24" {
		t.Errorf("whitelist entries incorrect, got %v", got)
	}
}

func TestProcessHandshake_ReadError(t *testing.T) {
	// Test read error during whitelist count
	rw := newStubRW(nil, 0) // Error after 0 reads
	_, err := processHandshake(rw, "192.168.1.1", []string{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "read whitelist count") {
		t.Errorf("error message %q does not contain expected text", err.Error())
	}
}

func TestProcessHandshake_EntryReadError(t *testing.T) {
	// Setup to succeed on count and length reads but fail on the entry content
	rw := newStubRW([]string{"entry-will-fail"}, 2)

	_, err := processHandshake(rw, "127.0.0.1", []string{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "read whitelist entry") {
		t.Errorf("error message %q does not contain expected text", err.Error())
	}
}

func TestProcessHandshake_LongWhitelistEntries(t *testing.T) {
	// Test with unusually long whitelist entries
	longEntry := strings.Repeat("1", 1000) + ".0.0.0/8"
	entries := []string{longEntry, "10.0.0.1"}

	rw := newStubRW(entries, -1)
	got, err := processHandshake(rw, "10.0.0.1", []string{})

	if err != nil {
		t.Fatalf("processHandshake returned error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
	if got[0] != longEntry {
		t.Errorf("long entry was corrupted: got %d bytes, expected %d bytes",
			len(got[0]), len(longEntry))
	}
}

// --- Tests for isAllowed ---
func TestIsAllowed_ManyEntriesPerformance(t *testing.T) {
	// Generate a large number of allowed entries
	const numEntries = 10000
	allowed := make([]string, numEntries)
	for i := 0; i < numEntries; i++ {
		allowed[i] = fmt.Sprintf("192.168.%d.%d", i/255, i%255)
	}

	// Add the needle at the end
	testIP := "192.168.99.99"
	allowed[numEntries-1] = testIP

	// Verify the function still works correctly and completes in reasonable time
	start := time.Now()
	result := isAllowed(testIP, allowed)
	duration := time.Since(start)

	if !result {
		t.Errorf("isAllowed(%q) = false, want true", testIP)
	}

	if duration > 100*time.Millisecond {
		t.Logf("isAllowed with %d entries took %v", numEntries, duration)
	}
}

func TestIsAllowed_CIDRMatches(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		allowed []string
		want    bool
	}{
		{name: "exact IP match", ip: "192.168.1.1", allowed: []string{"192.168.1.1"}, want: true},
		{name: "CIDR match", ip: "192.168.1.100", allowed: []string{"192.168.1.0/24"}, want: true},
		{name: "CIDR and exact mixed", ip: "10.0.0.5", allowed: []string{"192.168.1.0/24", "10.0.0.5", "172.16.0.0/16"}, want: true},
		{name: "no match", ip: "8.8.8.8", allowed: []string{"192.168.1.0/24", "10.0.0.5", "172.16.0.0/16"}, want: false},
		{name: "empty allowed list", ip: "8.8.8.8", allowed: []string{}, want: true}, // empty list should allow all
		{name: "invalid CIDR", ip: "8.8.8.8", allowed: []string{"invalid/cidr"}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAllowed(tc.ip, tc.allowed)
			if got != tc.want {
				t.Errorf("isAllowed(%q, %v) = %v; want %v", tc.ip, tc.allowed, got, tc.want)
			}
		})
	}
}

func TestIsAllowed_ValidIPAddress(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		allowed  []string
		expected bool
	}{
		{name: "class-a-exact-match", ip: "10.1.2.3", allowed: []string{"10.1.2.3"}, expected: true},
		{name: "class-b-exact-match", ip: "172.16.45.67", allowed: []string{"172.16.45.67"}, expected: true},
		{name: "class-c-exact-match", ip: "192.168.1.100", allowed: []string{"192.168.1.100"}, expected: true},
		{name: "loopback-match", ip: "127.0.0.1", allowed: []string{"127.0.0.0/8"}, expected: true},
		{name: "cidr-boundary-included-start", ip: "192.168.1.0", allowed: []string{"192.168.1.0/24"}, expected: true},
		{name: "cidr-boundary-included-end", ip: "192.168.1.255", allowed: []string{"192.168.1.0/24"}, expected: true},
		{name: "cidr-boundary-excluded", ip: "192.168.2.0", allowed: []string{"192.168.1.0/24"}, expected: false},
		{name: "multiple-allowed-match-first", ip: "10.0.0.1", allowed: []string{"10.0.0.0/24", "172.16.0.0/16", "192.168.1.0/24"}, expected: true},
		{name: "multiple-allowed-match-middle", ip: "172.16.1.1", allowed: []string{"10.0.0.0/24", "172.16.0.0/16", "192.168.1.0/24"}, expected: true},
		{name: "multiple-allowed-match-last", ip: "192.168.1.1", allowed: []string{"10.0.0.0/24", "172.16.0.0/16", "192.168.1.0/24"}, expected: true},
		{name: "zero-ip-allowed", ip: "0.0.0.0", allowed: []string{"0.0.0.0/0"}, expected: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isAllowed(tc.ip, tc.allowed)
			if result != tc.expected {
				t.Errorf("isAllowed(%q, %v) = %v; want %v", tc.ip, tc.allowed, result, tc.expected)
			}
		})
	}
}

func TestIsAllowed_InvalidIPAddress(t *testing.T) {
	allowed := []string{"10.0.0.0/8", "192.168.0.0/16"}

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"empty-ip", "", false},
		{"malformed-ip", "300.400.500.600", false},
		{"incomplete-ip", "10.0.0", false},
		{"ipv6-address", "::1", false},
		{"non-ip-string", "localhost", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAllowed(tc.ip, allowed)
			if got != tc.want {
				t.Errorf("isAllowed(%q, %v) = %v; want %v", tc.ip, allowed, got, tc.want)
			}
		})
	}
}
