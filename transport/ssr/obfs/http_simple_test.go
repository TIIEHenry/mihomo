package obfs

import (
	"errors"
	"io"
	"net"
	"testing"
)

type scriptedConn struct {
	net.Conn
	reads []struct {
		b   []byte
		err error
	}
	n int
}

func (c *scriptedConn) Read(p []byte) (int, error) {
	if c.n >= len(c.reads) {
		return 0, io.EOF
	}
	item := c.reads[c.n]
	c.n++
	return copy(p, item.b), item.err
}

func TestHTTPSimpleEmptyReadStops(t *testing.T) {
	conn := &scriptedConn{reads: []struct {
		b   []byte
		err error
	}{{b: nil, err: nil}, {b: nil, err: errors.New("spun")}}}
	c := &httpConn{Conn: conn}
	buf := make([]byte, 16)
	_, err := c.Read(buf)
	if err == nil || err.Error() != "http_simple empty read" {
		t.Fatalf("err = %v, reads = %d", err, conn.n)
	}
	if conn.n != 1 {
		t.Fatalf("reads = %d", conn.n)
	}
}

func TestHTTPSimpleHeaderAcrossReads(t *testing.T) {
	conn := &scriptedConn{reads: []struct {
		b   []byte
		err error
	}{
		{b: []byte("HTTP/1.1 200 OK\r\n")},
		{b: []byte("\r\nhello")},
	}}
	c := &httpConn{Conn: conn}
	buf := make([]byte, 16)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("body = %q", buf[:n])
	}
}
