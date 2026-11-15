// SPDX-License-Identifier: MIT

package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// serve serves h with the HTTP server of server start, its timeouts
// shortened, and returns its address.
func serve(t *testing.T, h http.Handler) string {
	t.Helper()
	r, w, i := readTimeout, writeTimeout, idleTimeout
	readTimeout, writeTimeout, idleTimeout = 200*time.Millisecond, 200*time.Millisecond, 200*time.Millisecond
	hs := newHTTPServer(h)
	readTimeout, writeTimeout, idleTimeout = r, w, i
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = hs.Serve(l) }()
	t.Cleanup(func() { _ = hs.Close() })
	return l.Addr().String()
}

// closedByServer reads c until the server closes it, and fails the test
// when that takes 10 s: the server's timeouts are 200 ms here.
func closedByServer(t *testing.T, c net.Conn, what string) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, err := io.Copy(io.Discard, c)
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatalf("%s: the server held the connection open for 10 s", what)
	}
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// A peer that sends a request's headers and then stalls its body is cut
// off after the read timeout.
func TestHTTPServerStalledBody(t *testing.T) {
	addr := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	c := dial(t, addr)
	_, _ = io.WriteString(c, "POST /edev HTTP/1.1\r\nHost: x\r\nContent-Type: application/sep+xml\r\nContent-Length: 1000\r\n\r\n<EndDev")
	closedByServer(t, c, "a stalled body")
}

// A keep-alive connection with no next request is closed after the idle
// timeout.
func TestHTTPServerIdleConnection(t *testing.T) {
	addr := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	c := dial(t, addr)
	_, _ = io.WriteString(c, "GET /dcap HTTP/1.1\r\nHost: x\r\n\r\n")
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, err = io.Copy(io.Discard, br)
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatal("an idle keep-alive connection was held open for 10 s")
	}
}

// A peer that does not read its answer does not keep the handler writing:
// the write fails after the write timeout.
func TestHTTPServerUnreadAnswer(t *testing.T) {
	done := make(chan error, 1)
	addr := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chunk := make([]byte, 64<<10)
		for range 1 << 14 { // up to 1 GiB, far beyond what the socket buffers hold
			if _, err := w.Write(chunk); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}))
	c := dial(t, addr)
	_, _ = io.WriteString(c, "GET /big HTTP/1.1\r\nHost: x\r\n\r\n")
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the handler wrote 1 GiB to a peer that reads nothing")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the handler was still writing to a peer that reads nothing after 20 s")
	}
}

// Headers beyond 64 KiB are refused with 431; headers below are taken.
func TestHTTPServerMaxHeaderBytes(t *testing.T) {
	addr := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for _, c := range []struct {
		size int
		want int
	}{{32 << 10, http.StatusOK}, {maxHeaderBytes + 8<<10, http.StatusRequestHeaderFieldsTooLarge}} {
		conn := dial(t, addr)
		_, _ = fmt.Fprintf(conn, "GET /dcap HTTP/1.1\r\nHost: x\r\nX-Pad: %s\r\n\r\n", strings.Repeat("a", c.size))
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatalf("headers of %d bytes: %v", c.size, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("headers of %d bytes: %d, want %d", c.size, resp.StatusCode, c.want)
		}
	}
}
