// SPDX-License-Identifier: MIT

package conn

import (
	"net"
	"sync"
)

// LimitListener returns a listener that keeps at most n connections of l
// open at once: while n are open, Accept waits until one of them is closed.
// Connections that wait are left in l (the kernel's backlog of a TCP
// listener), where a TLS handshake has not started. n of 0 or less is no
// limit: LimitListener returns l.
func LimitListener(l net.Listener, n int) net.Listener {
	if n <= 0 {
		return l
	}
	return &limitListener{Listener: l, slots: make(chan struct{}, n), closed: make(chan struct{})}
}

type limitListener struct {
	net.Listener
	slots     chan struct{} // a value per open connection
	closeOnce sync.Once
	closed    chan struct{}
}

// Accept waits for a free slot, then for a connection of the listener.
func (l *limitListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.closed:
		return nil, net.ErrClosed
	}
	c, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &limitConn{Conn: c, free: func() { <-l.slots }}, nil
}

// Close closes the listener, and ends the Accept calls that wait for a
// slot.
func (l *limitListener) Close() error {
	err := l.Listener.Close()
	l.closeOnce.Do(func() { close(l.closed) })
	return err
}

// limitConn frees its slot the first time it is closed.
type limitConn struct {
	net.Conn
	once sync.Once
	free func()
}

func (c *limitConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.free)
	return err
}
