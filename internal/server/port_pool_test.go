package server

import (
	"errors"
	"sync"
	"testing"

	"github.com/poweredbypump/pbp-tunnel/internal/protocol"
)

func TestPortPool_Reserve(t *testing.T) {
	tests := []struct {
		name        string
		first, last int
		portsInUse  []int
		requested   int
		wantPort    int
		wantErr     error
	}{
		{name: "requested port is free", first: 1500, last: 1502, requested: 1500, wantPort: 1500},
		{name: "requested port is in use", first: 1500, last: 1502, portsInUse: []int{1500}, requested: 1500, wantErr: protocol.StatusPortUnavailable},
		{name: "requested port below range", first: 1500, last: 1502, requested: 1400, wantErr: protocol.StatusPortOutOfRange},
		{name: "requested port above range", first: 1500, last: 1502, requested: 1600, wantErr: protocol.StatusPortOutOfRange},
		{name: "first free port when none requested", first: 1500, last: 1502, portsInUse: []int{1500, 1501}, wantPort: 1502},
		{name: "single-port range", first: 1000, last: 1000, wantPort: 1000},
		{name: "every port in use", first: 1500, last: 1502, portsInUse: []int{1500, 1501, 1502}, wantErr: protocol.StatusPortUnavailable},
		{name: "reversed range", first: 2000, last: 1000, wantErr: protocol.StatusPortUnavailable},
		{name: "reversed range with requested port", first: 9000, last: 8000, requested: 8080, wantErr: protocol.StatusPortUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ports := newPortPool(tc.first, tc.last)
			for _, port := range tc.portsInUse {
				mustReserve(t, ports, port)
			}

			port, err := ports.Reserve(tc.requested)

			if port != tc.wantPort || !errors.Is(err, tc.wantErr) {
				t.Fatalf("Reserve(%d) = %d, %v; want %d, %v", tc.requested, port, err, tc.wantPort, tc.wantErr)
			}
		})
	}
}

func TestPortPool_ReservedPortIsTakenUntilReleased(t *testing.T) {
	ports := newPortPool(1500, 1502)
	port := mustReserve(t, ports, 1500)

	if _, err := ports.Reserve(port); !errors.Is(err, protocol.StatusPortUnavailable) {
		t.Fatalf("second Reserve(%d) error = %v, want %v", port, err, protocol.StatusPortUnavailable)
	}
	ports.Release(port)
	if _, err := ports.Reserve(port); err != nil {
		t.Errorf("Reserve(%d) after Release: %v", port, err)
	}
}

func TestPortPool_ConcurrentCallersNeverGetTheSamePort(t *testing.T) {
	ports := newPortPool(10000, 20000)
	const callers, portsPerCaller = 10, 100

	assigned := make(chan int, callers*portsPerCaller)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			for range portsPerCaller {
				port, err := ports.Reserve(0)
				if err != nil {
					t.Errorf("Reserve: %v", err)
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

func mustReserve(t *testing.T, ports *portPool, port int) int {
	t.Helper()
	reserved, err := ports.Reserve(port)
	if err != nil {
		t.Fatalf("Reserve(%d): %v", port, err)
	}
	return reserved
}
