package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/metacubex/mihomo/transport/hysteria/transport"
	"github.com/metacubex/quic-go"
	"github.com/metacubex/tls"
)

type testDialer struct {
	ctx context.Context
}

func (d testDialer) ListenPacket(rAddr net.Addr) (net.PacketConn, error) {
	return net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
}

func (d testDialer) Context() context.Context { return d.ctx }

func (d testDialer) RemoteAddr(host string) (net.Addr, error) {
	return net.ResolveUDPAddr("udp", host)
}

func testCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func recvConn(t *testing.T, conns <-chan *quic.Conn) *quic.Conn {
	t.Helper()
	select {
	case conn := <-conns:
		return conn
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for quic conn")
		return nil
	}
}

func startHysteriaServer(t *testing.T) (string, <-chan *quic.Conn, func()) {
	t.Helper()
	ln, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{testCert(t)},
		NextProtos:   []string{"hysteria"},
	}, &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	conns := make(chan *quic.Conn, 4)
	go func() {
		for {
			conn, err := ln.Accept(context.Background())
			if err != nil {
				return
			}
			go func(conn *quic.Conn) {
				stream, err := conn.AcceptStream(context.Background())
				if err != nil {
					return
				}
				if _, err = ReadClientHello(stream); err != nil {
					return
				}
				if err = WriteServerHello(stream, ServerHello{OK: true, SendBPS: 1, RecvBPS: 1}); err != nil {
					return
				}
				conns <- conn
			}(conn)
		}
	}()
	return ln.Addr().String(), conns, func() { _ = ln.Close() }
}

func TestReconnectDoesNotDeliverOldUDP(t *testing.T) {
	addr, conns, stop := startHysteriaServer(t)
	defer stop()

	client, err := NewClient(addr, "", "udp", nil, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"hysteria"},
		ServerName:         "127.0.0.1",
	}, &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  time.Minute,
	}, &transport.ClientTransport{}, 1, 1, nil, nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	dialer := testDialer{ctx: context.Background()}
	if err = client.connectToServer(dialer); err != nil {
		t.Fatal(err)
	}
	oldSession := client.quicSession
	oldMap := client.udpSessionMap
	oldCh := make(chan *udpMessage, 4)
	client.udpSessionMutex.Lock()
	oldMap[1] = oldCh
	oldMap[99] = make(chan *udpMessage, 1)
	client.udpSessionMutex.Unlock()

	oldServer := recvConn(t, conns)
	if err = oldServer.SendDatagram((udpMessage{
		SessionID: 1,
		Host:      "h",
		Port:      1,
		MsgID:     9,
		FragID:    0,
		FragCount: 2,
		Data:      []byte("OLD"),
	}).Pack()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	select {
	case msg := <-oldCh:
		t.Fatalf("partial delivered: %s", msg.Data)
	default:
	}

	if err = oldServer.CloseWithError(0, "bye"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	var openErr error
	for {
		_, openErr = oldSession.OpenStream()
		if openErr != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("OpenStream still succeeds")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ne, ok := openErr.(net.Error); ok && ne.Temporary() {
		t.Fatalf("temporary: %v", openErr)
	}

	session, _, err := client.openStreamWithReconnect(dialer)
	if err != nil {
		t.Fatal(err)
	}
	if session == oldSession || client.quicSession == oldSession {
		t.Fatal("session not replaced")
	}
	client.udpSessionMutex.Lock()
	_, reused := client.udpSessionMap[99]
	newCh := make(chan *udpMessage, 8)
	client.udpSessionMap[1] = newCh
	client.udpSessionMutex.Unlock()
	if reused {
		t.Fatal("udpSessionMap reused")
	}

	_ = oldServer.SendDatagram((udpMessage{
		SessionID: 1,
		Host:      "h",
		Port:      1,
		FragCount: 1,
		Data:      []byte("stale"),
	}).Pack())

	newServer := recvConn(t, conns)
	stopHammer := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			client.udpSessionMutex.Lock()
			id := uint32(1000 + i)
			client.udpSessionMap[id] = make(chan *udpMessage, 1)
			delete(client.udpSessionMap, id)
			client.udpSessionMutex.Unlock()
		}
		close(stopHammer)
	}()
	if err = newServer.SendDatagram((udpMessage{
		SessionID: 1,
		Host:      "h",
		Port:      1,
		MsgID:     9,
		FragID:    1,
		FragCount: 2,
		Data:      []byte("NEW"),
	}).Pack()); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-newCh:
		t.Fatalf("new map got %q", msg.Data)
	case <-time.After(200 * time.Millisecond):
	}
	if err = newServer.SendDatagram((udpMessage{
		SessionID: 1,
		Host:      "h",
		Port:      1,
		MsgID:     10,
		FragID:    0,
		FragCount: 2,
		Data:      []byte("AAA"),
	}).Pack()); err != nil {
		t.Fatal(err)
	}
	if err = newServer.SendDatagram((udpMessage{
		SessionID: 1,
		Host:      "h",
		Port:      1,
		MsgID:     10,
		FragID:    1,
		FragCount: 2,
		Data:      []byte("BBB"),
	}).Pack()); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-newCh:
		if string(msg.Data) != "AAABBB" {
			t.Fatalf("reassembly %q", msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("missing reassembly")
	}
	select {
	case msg := <-newCh:
		t.Fatalf("extra datagram %q", msg.Data)
	default:
	}
	<-stopHammer
}
