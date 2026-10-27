package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// An idle external connection on the forwarded port must not keep the port
// reserved once the tunnel client has disconnected.
func TestServeTCPForward_ReleasesPortWithIdleExternalConn(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)

	port := freeTCPPort(t)
	srv := &ForwardServer{
		sshConfig:      cfg,
		bindAddress:    "127.0.0.1",
		portRangeStart: port,
		portRangeEnd:   port,
		forwards:       make(map[int]struct{}),
	}

	sshLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer sshLn.Close()
	go func() {
		for {
			nc, err := sshLn.Accept()
			if err != nil {
				return
			}
			go srv.handleSSHConnection(nc)
		}
	}()

	client, err := ssh.Dial("tcp", sshLn.Addr().String(), &ssh.ClientConfig{
		User:            "test",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Accept back-channels but never answer, like an idle service.
	go func() {
		for nc := range client.HandleChannelOpen("direct-tcpip") {
			ch, reqs, err := nc.Accept()
			if err != nil {
				continue
			}
			go ssh.DiscardRequests(reqs)
			go io.Copy(io.Discard, ch)
		}
	}()

	ch, reqs, err := client.OpenChannel("direct-tcpip", nil)
	if err != nil {
		t.Fatal(err)
	}
	go ssh.DiscardRequests(reqs)

	var hb [4]byte
	readU32 := func() uint32 {
		if _, err := io.ReadFull(ch, hb[:]); err != nil {
			t.Fatal(err)
		}
		return binary.BigEndian.Uint32(hb[:])
	}
	writeU32 := func(v uint32) {
		binary.BigEndian.PutUint32(hb[:], v)
		if _, err := ch.Write(hb[:]); err != nil {
			t.Fatal(err)
		}
	}
	if readU32() != ErrSuccess {
		t.Fatal("handshake failed")
	}
	writeU32(0) // empty whitelist
	if readU32() != ErrSuccess {
		t.Fatal("whitelist rejected")
	}
	writeU32(3)
	ch.Write([]byte("tcp"))
	writeU32(uint32(port))
	if got := readU32(); got != uint32(port) {
		t.Fatalf("assigned port = %d; want %d", got, port)
	}

	ext, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer ext.Close()
	time.Sleep(200 * time.Millisecond)

	client.Close()

	ext.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := ext.Read(make([]byte, 1)); err == nil {
		t.Fatal("unexpected data on external connection")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("external connection still open after client disconnect")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		srv.lock.Lock()
		_, used := srv.forwards[port]
		srv.lock.Unlock()
		if !used {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("port %d still reserved after client disconnect", port)
}
