// SPDX-License-Identifier: MIT

package conn

import (
	"errors"
	"net"
	"sync"
	"testing"
	"testing/synctest"
)

// pipeListener is a listener of the connections a test puts in conns.
type pipeListener struct {
	conns  chan net.Conn
	once   sync.Once
	closed chan struct{}
}

func newPipeListener(t *testing.T, n int) *pipeListener {
	p := &pipeListener{conns: make(chan net.Conn, n), closed: make(chan struct{})}
	for range n {
		a, b := net.Pipe()
		t.Cleanup(func() { _, _ = a.Close(), b.Close() })
		p.conns <- a
	}
	return p
}

func (p *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-p.conns:
		return c, nil
	case <-p.closed:
		return nil, net.ErrClosed
	}
}

func (p *pipeListener) Close() error {
	p.once.Do(func() { close(p.closed) })
	return nil
}

func (p *pipeListener) Addr() net.Addr { return &net.UnixAddr{Name: "pipe", Net: "unix"} }

func accept(t *testing.T, l net.Listener) net.Conn {
	t.Helper()
	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// With n connections open, Accept takes no other until one is closed, and
// a connection closed twice frees one slot.
func TestLimitListener(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := LimitListener(newPipeListener(t, 4), 2)
		c1, c2 := accept(t, l), accept(t, l)
		got := make(chan net.Conn, 4)
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				got <- c
			}
		}()
		synctest.Wait()
		if len(got) != 0 {
			t.Fatal("a third connection was accepted while two were open")
		}
		_ = c1.Close()
		_ = c1.Close()
		synctest.Wait()
		if len(got) != 1 {
			t.Fatalf("%d connections accepted after one of two closed (twice), want 1", len(got))
		}
		_ = c2.Close()
		synctest.Wait()
		if len(got) != 2 {
			t.Fatalf("%d connections accepted after two closed, want 2", len(got))
		}
		_ = l.Close()
	})
}

// Close ends an Accept that waits for a slot, with net.ErrClosed.
func TestLimitListenerClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := LimitListener(newPipeListener(t, 2), 1)
		c := accept(t, l)
		defer func() { _ = c.Close() }()
		done := make(chan error, 1)
		go func() {
			_, err := l.Accept()
			done <- err
		}()
		synctest.Wait()
		if len(done) != 0 {
			t.Fatal("Accept returned while the one slot was taken")
		}
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-done; !errors.Is(err, net.ErrClosed) {
			t.Errorf("Accept after Close: %v, want net.ErrClosed", err)
		}
		if err := l.Close(); err != nil {
			t.Errorf("a second Close: %v", err)
		}
	})
}

// A limit of 0 or less is no limit: the listener itself.
func TestLimitListenerNoLimit(t *testing.T) {
	inner := newPipeListener(t, 0)
	for _, n := range []int{0, -1} {
		if l := LimitListener(inner, n); l != net.Listener(inner) {
			t.Errorf("LimitListener(l, %d) wraps l", n)
		}
	}
}
