package config

import (
	"golang.org/x/crypto/ssh"
	"net"
	"strings"
	"testing"
)

// dummyConn implements ssh.ConnMetadata for testing PasswordCallback
type dummyConn struct{ user string }

func (d *dummyConn) User() string                  { return d.user }
func (d *dummyConn) SessionID() []byte             { return nil }
func (d *dummyConn) ClientVersion() []byte         { return nil }
func (d *dummyConn) ServerVersion() []byte         { return nil }
func (d *dummyConn) RemoteAddr() net.Addr          { return nil }
func (d *dummyConn) LocalAddr() net.Addr           { return nil }
func (d *dummyConn) Permissions() *ssh.Permissions { return nil }

func TestGetClientConfig_PasswordAuth(t *testing.T) {
	params := &ClientParameters{
		Username:     "testuser",
		Password:     "secret",
		Endpoint:     "example.com",
		EndpointPort: 2222,
	}
	sshCfg, addr, err := GetClientConfig(params)
	if err != nil {
		t.Fatalf("GetClientConfig returned error: %v", err)
	}
	if addr != "example.com:2222" {
		t.Errorf("addr = %q; want %q", addr, "example.com:2222")
	}
	if sshCfg.User != "testuser" {
		t.Errorf("sshCfg.User = %q; want %q", sshCfg.User, "testuser")
	}
	if len(sshCfg.Auth) != 1 {
		t.Errorf("len(sshCfg.Auth) = %d; want %d", len(sshCfg.Auth), 1)
	}
	// HostKeyCallback should allow any key (insecure)
	if err := sshCfg.HostKeyCallback("host", nil, nil); err != nil {
		t.Errorf("HostKeyCallback returned error: %v", err)
	}
}

func TestGetClientConfig_PrivateKeyPathError(t *testing.T) {
	params := &ClientParameters{
		Username:       "testuser",
		PrivateKeyPath: "/nonexistent/key",
		Endpoint:       "example.com",
		EndpointPort:   22,
	}
	_, _, err := GetClientConfig(params)
	if err == nil {
		t.Fatal("expected error for missing private key, got nil")
	}
	if !strings.Contains(err.Error(), "read private key") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "read private key")
	}
}

func TestGetServerConfig_PasswordCallback(t *testing.T) {
	params := &ServerParameters{
		BindAddress:    "0.0.0.0",
		BindPort:       2022,
		Username:       "admin",
		Password:       "passwd",
		PrivateRsaPath: "", // no host key file
	}
	sshCfg, addr, err := GetServerConfig(params)
	if err != nil {
		t.Fatalf("GetServerConfig returned error: %v", err)
	}
	if addr != "0.0.0.0:2022" {
		t.Errorf("addr = %q; want %q", addr, "0.0.0.0:2022")
	}
	// PasswordCallback should accept correct and reject incorrect password
	cb := sshCfg.PasswordCallback
	if cb == nil {
		t.Fatal("expected PasswordCallback to be set, got nil")
	}
	// Correct
	if perms, err := cb(&dummyConn{user: "admin"}, []byte("passwd")); err != nil || perms != nil {
		t.Errorf("PasswordCallback ok = (%v, %v); want (nil, nil)", perms, err)
	}
	// Incorrect
	if _, err := cb(&dummyConn{user: "admin"}, []byte("wrong")); err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}

func TestGetServerConfig_NoAuth(t *testing.T) {
	params := &ServerParameters{
		BindAddress:    "127.0.0.1",
		BindPort:       8022,
		Username:       "",
		Password:       "",
		PrivateRsaPath: "", // no host key
	}
	sshCfg, _, err := GetServerConfig(params)
	if err != nil {
		t.Fatalf("GetServerConfig returned error: %v", err)
	}
	// When no password, PasswordCallback should be nil (no auth)
	if sshCfg.PasswordCallback != nil {
		t.Errorf("expected PasswordCallback to be nil, got non-nil")
	}
}
