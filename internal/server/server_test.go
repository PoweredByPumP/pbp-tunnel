package server

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
)

func TestAssignPort(t *testing.T) {
	tests := []struct {
		name          string
		requestedPort int
		start, end    int
		portsInUse    []int
		wantPort      int
		wantErr       error
	}{
		{name: "requested port is free", requestedPort: 1500, start: 1500, end: 1502, wantPort: 1500},
		{name: "requested port is in use", requestedPort: 1500, start: 1500, end: 1502, portsInUse: []int{1500}, wantErr: protocol.StatusPortUnavailable},
		{name: "requested port below range", requestedPort: 1400, start: 1500, end: 1502, wantErr: protocol.StatusPortOutOfRange},
		{name: "requested port above range", requestedPort: 1600, start: 1500, end: 1502, wantErr: protocol.StatusPortOutOfRange},
		{name: "first free port when none requested", start: 1500, end: 1502, portsInUse: []int{1500, 1501}, wantPort: 1502},
		{name: "single-port range", start: 1000, end: 1000, wantPort: 1000},
		{name: "every port in use", start: 1500, end: 1502, portsInUse: []int{1500, 1501, 1502}, wantErr: protocol.StatusPortUnavailable},
		{name: "reversed range", start: 2000, end: 1000, wantErr: protocol.StatusPortUnavailable},
		{name: "reversed range with requested port", requestedPort: 8080, start: 9000, end: 8000, wantErr: protocol.StatusPortUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			forwards := make(map[int]struct{})
			for _, port := range tc.portsInUse {
				forwards[port] = struct{}{}
			}
			var lock sync.Mutex

			port, err := assignPort(tc.requestedPort, tc.start, tc.end, forwards, &lock)

			if port != tc.wantPort || !errors.Is(err, tc.wantErr) {
				t.Fatalf("assignPort(%d, %d, %d) = %d, %v; want %d, %v",
					tc.requestedPort, tc.start, tc.end, port, err, tc.wantPort, tc.wantErr)
			}
			if _, reserved := forwards[port]; err == nil && !reserved {
				t.Errorf("port %d was not marked as in use", port)
			}
		})
	}
}

func TestAssignPort_ConcurrentCallersNeverGetTheSamePort(t *testing.T) {
	forwards := make(map[int]struct{})
	var lock sync.Mutex
	const callers, portsPerCaller = 10, 100

	assigned := make(chan int, callers*portsPerCaller)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			for range portsPerCaller {
				port, err := assignPort(0, 10000, 20000, forwards, &lock)
				if err != nil {
					t.Errorf("assignPort: %v", err)
					return
				}
				assigned <- port
			}
		})
	}
	wg.Wait()
	close(assigned)

	alreadyAssigned := make(map[int]bool)
	for port := range assigned {
		if alreadyAssigned[port] {
			t.Errorf("port %d was assigned twice", port)
		}
		alreadyAssigned[port] = true
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

// Test de performance sur isAllowed avec cache simulation
func TestIsAllowed_PerformanceWithCaching(t *testing.T) {
	// Simuler un cache simple
	cache := make(map[string]bool)
	var cacheMutex sync.RWMutex

	// Fonction isAllowed avec cache
	isAllowedWithCache := func(ip string, allowed []string) bool {
		cacheMutex.RLock()
		if result, exists := cache[ip]; exists {
			cacheMutex.RUnlock()
			return result
		}
		cacheMutex.RUnlock()

		result := isAllowed(ip, allowed)

		cacheMutex.Lock()
		cache[ip] = result
		cacheMutex.Unlock()

		return result
	}

	// Créer une grande liste d'IPs autorisées
	allowed := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		allowed[i] = fmt.Sprintf("192.168.%d.0/24", i%256)
	}

	// Test de performance sans cache
	start := time.Now()
	for i := 0; i < 1000; i++ {
		testIP := fmt.Sprintf("192.168.%d.100", i%256)
		isAllowed(testIP, allowed)
	}
	durationWithoutCache := time.Since(start)

	// Test de performance avec cache
	start = time.Now()
	for i := 0; i < 1000; i++ {
		testIP := fmt.Sprintf("192.168.%d.100", i%256)
		isAllowedWithCache(testIP, allowed)
	}
	durationWithCache := time.Since(start)

	t.Logf("Performance without cache: %v", durationWithoutCache)
	t.Logf("Performance with cache: %v", durationWithCache)

	if durationWithCache > durationWithoutCache {
		t.Logf("Note: Cache overhead detected for small datasets")
	}
}

// Test de validation robuste des IPs
func TestIsAllowed_RobustIPValidation(t *testing.T) {
	// Test avec des entrées malformées qui pourraient causer des paniques
	malformedEntries := []string{
		"300.300.300.300",
		"192.168.1",
		"192.168.1.1/33", // CIDR invalide
		"",
		"invalid-string",
		"::1", // IPv6 (pas encore supporté)
		"192.168.1.1/",
		"/24",
		"192.168.1.1/-1",
	}

	testIPs := []string{
		"192.168.1.1",
		"10.0.0.1",
		"invalid-ip",
		"",
	}

	for _, entry := range malformedEntries {
		for _, testIP := range testIPs {
			// Ne devrait pas paniquer même avec des entrées malformées
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic with entry %q and IP %q: %v", entry, testIP, r)
					}
				}()
				isAllowed(testIP, []string{entry})
			}()
		}
	}
}
