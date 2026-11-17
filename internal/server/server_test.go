package server

import (
	"errors"
	"sync"
	"testing"

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
