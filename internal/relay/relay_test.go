package relay

import (
	"io"
	"net"
	"testing"
	"time"
)

// A request/response exchange where each side only answers once the other has
// finished sending, like an HTTP/1.0 server reading a request until EOF.
func TestCopyBothWays_RelaysAndPropagatesEndOfStream(t *testing.T) {
	client, relayClientSide := tcpConnPair(t)
	relayServiceSide, service := tcpConnPair(t)

	relayDone := make(chan [2]int64, 1)
	go func() {
		aToB, bToA := CopyBothWays(relayClientSide, relayServiceSide)
		relayDone <- [2]int64{aToB, bToA}
	}()

	serviceDone := make(chan error, 1)
	go func() {
		request, err := io.ReadAll(service)
		if err == nil && string(request) != "request" {
			t.Errorf("service received %q, want %q", request, "request")
		}
		if err == nil {
			_, err = service.Write([]byte("response!"))
		}
		_ = service.CloseWrite()
		serviceDone <- err
	}()

	if _, err := client.Write([]byte("request")); err != nil {
		t.Fatal(err)
	}
	_ = client.CloseWrite()
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	response, err := io.ReadAll(client)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(response) != "response!" {
		t.Errorf("client received %q, want %q", response, "response!")
	}
	if err := <-serviceDone; err != nil {
		t.Fatalf("service: %v", err)
	}

	select {
	case copied := <-relayDone:
		if copied != [2]int64{7, 9} {
			t.Errorf("CopyBothWays copied %v bytes, want [7 9]", copied)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CopyBothWays did not return after both sides finished")
	}
}

func tcpConnPair(t *testing.T) (dialed, accepted *net.TCPConn) {
	t.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	acceptedConn := make(chan *net.TCPConn, 1)
	go func() {
		conn, _ := listener.AcceptTCP()
		acceptedConn <- conn
	}()
	dialed, err = net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatal(err)
	}
	accepted = <-acceptedConn
	if accepted == nil {
		t.Fatal("accept failed")
	}
	t.Cleanup(func() { dialed.Close(); accepted.Close() })
	return dialed, accepted
}
