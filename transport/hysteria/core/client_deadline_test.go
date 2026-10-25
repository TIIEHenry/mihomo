package core

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestQuicPktConnReadDeadline(t *testing.T) {
	ch := make(chan *udpMessage, 4)
	var mu sync.Mutex
	closed := false
	c := &quicPktConn{
		CloseFunc: func() {
			mu.Lock()
			closed = true
			mu.Unlock()
		},
		MsgCh: ch,
	}
	closedNow := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return closed
	}

	if err := c.SetReadDeadline(time.Now().Add(40 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	_, _, err := c.ReadFrom()
	if err == nil || err == ErrClosed {
		t.Fatalf("err = %v", err)
	}
	ne, ok := err.(net.Error)
	if !ok || !ne.Timeout() {
		t.Fatalf("err = %v", err)
	}
	if closedNow() {
		t.Fatal("CloseFunc called")
	}

	ch <- &udpMessage{Host: "h", Port: 9, Data: []byte("p")}
	ch <- &udpMessage{Host: "h", Port: 9, Data: []byte("q")}
	data, addr, err := c.ReadFrom()
	if err != nil || string(data) != "p" || addr != "h:9" {
		t.Fatalf("data=%q addr=%s err=%v", data, addr, err)
	}
	data, _, err = c.ReadFrom()
	if err != nil || string(data) != "q" {
		t.Fatalf("data=%q err=%v", data, err)
	}

	if err = c.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() {
		time.Sleep(40 * time.Millisecond)
		errCh <- c.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
	}()
	done := make(chan error, 1)
	go func() {
		_, _, err := c.ReadFrom()
		done <- err
	}()
	select {
	case err = <-done:
	case <-time.After(time.Second):
		ch <- &udpMessage{Host: "h", Port: 9, Data: []byte("unblock")}
		t.Fatal("ReadFrom did not return")
	}
	if setErr := <-errCh; setErr != nil {
		t.Fatal(setErr)
	}
	if err == nil || err == ErrClosed {
		t.Fatalf("err = %v", err)
	}
	ne, ok = err.(net.Error)
	if !ok || !ne.Timeout() {
		t.Fatalf("err = %v", err)
	}
	if closedNow() {
		t.Fatal("CloseFunc called")
	}

	if err = c.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	blocked := make(chan struct{})
	go func() {
		_, _, err := c.ReadFrom()
		if err != nil {
			t.Errorf("cleared deadline: %v", err)
		}
		close(blocked)
	}()
	select {
	case <-blocked:
		t.Fatal("ReadFrom returned without a datagram")
	case <-time.After(40 * time.Millisecond):
	}
	ch <- &udpMessage{Host: "h", Port: 9, Data: []byte("r")}
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("ReadFrom still blocked")
	}
	if closedNow() {
		t.Fatal("CloseFunc called")
	}
}
