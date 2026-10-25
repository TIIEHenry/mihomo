package v5

import (
	"bytes"
	"net"
	"testing"

	N "github.com/metacubex/mihomo/common/net"
)

func domainAddress(host string, port uint16) Address {
	addr := make([]byte, 1+len(host))
	addr[0] = byte(len(host))
	copy(addr[1:], host)
	return Address{TYPE: AtypDomainName, ADDR: addr, PORT: port}
}

func mustPack(t *testing.T, pkt Packet) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := pkt.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func packetConnFrom(t *testing.T, packets [][]byte) *quicStreamPacketConn {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	go func() {
		for _, p := range packets {
			if _, err := server.Write(p); err != nil {
				return
			}
		}
	}()
	return &quicStreamPacketConn{inputConn: N.NewBufferedConn(client)}
}

func pack(id uint16, addr Address, payload []byte) Packet {
	return NewPacket(1, id, 1, 0, uint16(len(payload)), addr, payload)
}

func TestReadFromDomainAndIP(t *testing.T) {
	domain := domainAddress("abc", 53)
	ip4 := Address{TYPE: AtypIPv4, ADDR: []byte{1, 2, 3, 4}, PORT: 80}
	ip6 := Address{TYPE: AtypIPv6, ADDR: net.ParseIP("2001:db8::1").To16(), PORT: 443}
	payload := []byte("ping")
	pc := packetConnFrom(t, [][]byte{
		mustPack(t, pack(1, domain, payload)),
		mustPack(t, pack(2, domain, payload)),
		mustPack(t, pack(3, ip4, payload)),
		mustPack(t, pack(4, ip6, payload)),
	})

	buf := make([]byte, 16)
	n, addr, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(payload) || string(buf[:n]) != string(payload) {
		t.Fatalf("payload %q", buf[:n])
	}
	assertDomain(t, addr, domain)
	data, _, addr, err := pc.WaitReadFrom()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatalf("payload %q", data)
	}
	assertDomain(t, addr, domain)

	_, addr, err = pc.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	assertIP(t, addr, net.IP{1, 2, 3, 4}, 80)
	_, _, addr, err = pc.WaitReadFrom()
	if err != nil {
		t.Fatal(err)
	}
	assertIP(t, addr, net.ParseIP("2001:db8::1"), 443)
}

func assertDomain(t *testing.T, addr net.Addr, want Address) {
	t.Helper()
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		t.Fatal(err)
	}
	if host != string(want.ADDR[1:]) || port != "53" {
		t.Fatalf("addr %s", addr)
	}
	if udp, ok := addr.(*net.UDPAddr); ok && udp.IP.Equal(net.IP(want.ADDR)) {
		t.Fatal("domain treated as ipv4")
	}
	back, err := NewAddressNetAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	if back.TYPE != AtypDomainName || string(back.ADDR[1:]) != host || back.PORT != want.PORT {
		t.Fatalf("round trip %+v", back)
	}
}

func assertIP(t *testing.T, addr net.Addr, ip net.IP, port int) {
	t.Helper()
	udp, ok := addr.(*net.UDPAddr)
	if !ok || !udp.IP.Equal(ip) || udp.Port != port {
		t.Fatalf("addr %v", addr)
	}
}
