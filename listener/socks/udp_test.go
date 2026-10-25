package socks

import (
	"net"
	"testing"

	"github.com/metacubex/mihomo/transport/socks5"
)

type remoteConn struct {
	net.Conn
	remote net.Addr
}

func (c remoteConn) RemoteAddr() net.Addr { return c.remote }

func TestUDPAssocKeys(t *testing.T) {
	peer := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9}
	conn := remoteConn{remote: peer}

	domain := socks5.ParseAddr("example.com:53")
	if domain == nil || domain[0] != socks5.AtypDomainName {
		t.Fatal("domain address")
	}
	if got := udpAssocKeys(domain, conn); len(got) != 1 || got[0] != "127.0.0.1" {
		t.Fatalf("domain = %v", got)
	}

	unspec := socks5.ParseAddr("0.0.0.0:53")
	if got := udpAssocKeys(unspec, conn); len(got) != 1 || got[0] != "127.0.0.1" {
		t.Fatalf("unspecified = %v", got)
	}
	v6unspec := socks5.ParseAddr("[::]:53")
	if got := udpAssocKeys(v6unspec, conn); len(got) != 1 || got[0] != "127.0.0.1" {
		t.Fatalf("v6 unspecified = %v", got)
	}

	declared := socks5.ParseAddr("192.0.2.10:53")
	got := udpAssocKeys(declared, conn)
	if len(got) != 2 || got[0] != "127.0.0.1" || got[1] != "192.0.2.10" {
		t.Fatalf("declared = %v", got)
	}

	same := socks5.ParseAddr("127.0.0.1:53")
	if got := udpAssocKeys(same, conn); len(got) != 1 || got[0] != "127.0.0.1" {
		t.Fatalf("same = %v", got)
	}

	v6 := remoteConn{remote: &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 9}}
	if got := udpAssocKeys(domain, v6); len(got) != 1 || got[0] != "2001:db8::1" {
		t.Fatalf("v6 peer = %v", got)
	}
}

func TestTrackUDPAssocReleasesBothKeys(t *testing.T) {
	assoc := NewUDPAssoc()
	conn := remoteConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9}}
	release := trackUDPAssoc(socks5.ParseAddr("192.0.2.10:53"), conn, assoc)
	if !assoc.has("127.0.0.1") || !assoc.has("192.0.2.10") {
		t.Fatal("both addresses must be registered")
	}
	release()
	if assoc.has("127.0.0.1") || assoc.has("192.0.2.10") {
		t.Fatal("release left an address registered")
	}
}
