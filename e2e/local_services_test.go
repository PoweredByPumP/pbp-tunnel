package e2e

import (
	"io"
	"net"
	"testing"
)

func startTCPEchoService(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("start TCP echo service: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func startTCPServiceAnsweringAtEndOfRequest(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("start TCP service: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				request, err := io.ReadAll(conn)
				if err == nil {
					_, _ = conn.Write([]byte("response to " + string(request)))
				}
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func startUDPEchoService(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("start UDP echo service: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	go func() {
		buffer := make([]byte, 64*1024)
		for {
			n, sender, err := conn.ReadFrom(buffer)
			if err != nil {
				return
			}
			_, _ = conn.WriteTo(buffer[:n], sender)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("find free TCP port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", net.JoinHostPort(loopback, "0"))
	if err != nil {
		t.Fatalf("find free UDP port: %v", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}
