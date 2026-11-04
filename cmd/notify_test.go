// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/ieee20305ctl/internal/conn"
)

// listener is a client subscribe (or client run) of the tests, running in
// the background, and its notification listener.
type listener struct {
	addr string // host:port of the listener
	url  string // the listener's notification URL
	sub  string // the subscription client subscribe made

	mu     sync.Mutex
	events []event
	cancel context.CancelFunc
	done   chan struct{} // closed when the command has exited with code
	code   int
}

// saw reports whether a notification of resource was written, waiting
// for it until the command ends or 20 seconds pass.
func (l *listener) saw(resource string) bool {
	deadline := time.Now().Add(20 * time.Second)
	for {
		l.mu.Lock()
		ok := has(l.events, func(e event) bool { return e.Event == "notification" && e.Resource == resource })
		l.mu.Unlock()
		if ok {
			return true
		}
		select {
		case <-l.done:
			return false
		default:
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond) // polling a record, not synchronizing on time
	}
}

// legit posts a notification of the server (with its certificate) and
// waits for its record: the records are in the order the listener took
// the notifications, so what was posted before has been written by then.
func (l *listener) legit(t *testing.T, s *site) {
	t.Helper()
	hc := notifier(t, s.pki+"/ca.pem", s.pki+"/server.pem", s.pki+"/server.key")
	l.mu.Lock()
	sub := l.sub
	l.mu.Unlock()
	if code := post(t, hc, l.url, notification(t, "/legit", sub)); code < 200 || code > 299 {
		t.Fatalf("the server's notification: %d", code)
	}
	if !l.saw("/legit") {
		t.Fatal("no record of the server's notification")
	}
}

// notifications counts the notification records written so far.
func (l *listener) notifications() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, e := range l.events {
		if e.Event == "notification" {
			n++
		}
	}
	return n
}

// stop interrupts the command and returns its exit code.
func (l *listener) stop(t *testing.T) int {
	t.Helper()
	l.cancel()
	select {
	case <-l.done:
		return l.code
	case <-time.After(30 * time.Second):
		t.Fatal("the command did not stop when interrupted")
		return -1
	}
}

// startListener runs args (a streaming command with a listener) in the
// background, and waits for its listening record. It is stopped when the
// test ends.
func startListener(t *testing.T, args ...string) *listener {
	t.Helper()
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	l := &listener{cancel: cancel, done: make(chan struct{})}
	var errb bytes.Buffer
	go func() {
		l.code = Run(ctx, append(args, "--format", "json"), strings.NewReader(""), pw, &errb)
		_ = pw.Close()
		close(l.done)
	}()
	listening := make(chan string, 1)
	subscribed := make(chan struct{}, 1)
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			var e event
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue
			}
			l.mu.Lock()
			l.events = append(l.events, e)
			l.mu.Unlock()
			switch e.Event {
			case "listening":
				listening <- e.Address
			case "subscribed":
				subscribed <- struct{}{}
				l.mu.Lock()
				l.sub = e.Href
				l.mu.Unlock()
			}
		}
	}()
	select {
	case a := <-listening:
		l.url = a
		if !strings.Contains(a, "://") {
			l.url = "https://" + a + "/notify"
		}
		l.addr = strings.TrimSuffix(strings.TrimPrefix(l.url, "https://"), "/notify")
	case <-l.done:
		t.Fatalf("%s exited %d: %s", strings.Join(args, " "), l.code, errb.String())
	case <-time.After(30 * time.Second):
		t.Fatal("no listening record")
	}
	if args[1] == "subscribe" {
		select {
		case <-subscribed:
		case <-l.done:
			t.Fatalf("%s exited %d: %s", strings.Join(args, " "), l.code, errb.String())
		case <-time.After(30 * time.Second):
			t.Fatal("no subscribed record")
		}
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-l.done:
		case <-time.After(30 * time.Second):
			t.Error("the command did not stop")
		}
	})
	return l
}

// notifier is an HTTPS client that posts to a listener with the certificate
// of certFile/keyFile (none when empty), verifying the listener against ca.
func notifier(t *testing.T, ca, certFile, keyFile string) *http.Client {
	t.Helper()
	roots, err := conn.LoadPool(ca)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	if certFile != "" {
		c, err := conn.LoadKeyPair(certFile, keyFile)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Certificates = []tls.Certificate{c}
	}
	tr := &http.Transport{TLSClientConfig: cfg}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 20 * time.Second}
}

// notification is a Notification of a change of resource.
func notification(t *testing.T, resource, subscription string) []byte {
	t.Helper()
	n := &ieee20305.Notification{SubscriptionURI: subscription}
	n.SubscribedResource = resource
	data, err := ieee20305.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// post posts body to the listener; it returns the status, or 0 when the
// request did not get an answer (the handshake was refused, say).
func post(t *testing.T, hc *http.Client, url string, body []byte) int {
	t.Helper()
	resp, err := hc.Post(url, "application/sep+xml", bytes.NewReader(body))
	if err != nil {
		return 0
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// A peer that presents no certificate gets no notification in, unless
// --listen-allow-anonymous says so.
func TestNotifyListenerRefusesAnonymousPeers(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	l := startListener(t, "client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--listen-cert", s.pki+"/server.pem", "--listen-key", s.pki+"/server.key")
	hc := notifier(t, s.pki+"/ca.pem", "", "")
	if code := post(t, hc, l.url, notification(t, "/anonymous", "/edev/1/sub/1")); code >= 200 && code < 300 {
		t.Fatalf("an anonymous peer posted a notification: %d", code)
	}
	l.legit(t, s)
	if n := l.notifications(); n != 1 {
		t.Fatalf("%d notifications, want only the server's", n)
	}
	if code := l.stop(t); code != 0 {
		t.Fatalf("subscribe interrupted: exit %d", code)
	}
}

// Another device of the same CA is not the server: its certificate does not
// make it a notifier of this device.
func TestNotifyListenerRefusesAnotherDevice(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	l := startListener(t, "client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--listen-cert", s.pki+"/server.pem", "--listen-key", s.pki+"/server.key")
	hc := notifier(t, s.pki+"/ca.pem", s.pki+"/device-2.pem", s.pki+"/device-2.key")
	code := post(t, hc, l.url, notification(t, "/forged", "/edev/1/sub/1"))
	if code >= 200 && code < 300 {
		t.Errorf("device-2 posted a notification to device-1: %d", code)
	}
	l.legit(t, s)
	if n := l.notifications(); n != 1 {
		t.Errorf("%d notifications, want only the server's", n)
	}
}

// A notification for a subscription the command did not make is not one of
// its notifications.
func TestNotifyListenerOnlyItsSubscription(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	l := startListener(t, "client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--listen-cert", s.pki+"/server.pem", "--listen-key", s.pki+"/server.key")
	hc := notifier(t, s.pki+"/ca.pem", s.pki+"/server.pem", s.pki+"/server.key")
	_ = post(t, hc, l.url, notification(t, "/somewhere/else", "/edev/9/sub/42"))
	l.legit(t, s)
	if n := l.notifications(); n != 1 {
		t.Errorf("%d notifications, want only the one of the command's subscription", n)
	}
}

// An oversized notification is refused, and a flood of notifications does
// not block the listener or the command: it stops when interrupted, exit 0.
func TestNotifyListenerOversizedAndFlood(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	l := startListener(t, "client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--listen-cert", s.pki+"/server.pem", "--listen-key", s.pki+"/server.key")
	hc := notifier(t, s.pki+"/ca.pem", s.pki+"/server.pem", s.pki+"/server.key")

	big := append(notification(t, "/derp/1/derc", "/edev/1/sub/1"), bytes.Repeat([]byte(" "), 4<<20)...)
	if code := post(t, hc, l.url, big); code != http.StatusRequestEntityTooLarge && code != http.StatusBadRequest && code != 0 {
		t.Errorf("a 4 MiB notification: %d", code)
	}
	if code := post(t, hc, l.url, []byte("<nope/>")); code != http.StatusBadRequest {
		t.Errorf("a document that is not a Notification: %d", code)
	}

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 50 {
				res := "/derp/1/derc/" + string(rune('a'+i)) + strings.Repeat("x", j)
				_ = post(t, hc, l.url, notification(t, res, "/edev/1/sub/1"))
			}
		})
	}
	wg.Wait()
	if code := l.stop(t); code != 0 {
		t.Fatalf("subscribe after a flood: exit %d", code)
	}
}

// --count ends the stream after that many notifications, exit 0, even
// when many arrive at once.
func TestNotifyListenerCount(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	l := startListener(t, "client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--listen-cert", s.pki+"/server.pem", "--listen-key", s.pki+"/server.key", "--count", "3")
	hc := notifier(t, s.pki+"/ca.pem", s.pki+"/server.pem", s.pki+"/server.key")
	for i := range 10 {
		_ = post(t, hc, l.url, notification(t, "/derp/1/derc/"+strings.Repeat("y", i+1), "/edev/1/sub/1"))
	}
	select {
	case <-l.done:
		if l.code != 0 {
			t.Fatalf("subscribe --count 3: exit %d", l.code)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("subscribe --count 3 did not end after 10 notifications")
	}
}

// A peer that opens a connection to the listener and sends nothing, or
// sends a request's headers and then nothing of its body, is cut off.
func TestNotifyListenerSlowPeer(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	l := startListener(t, "client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--listen-cert", s.pki+"/server.pem", "--listen-key", s.pki+"/server.key")
	c, err := conn.LoadKeyPair(s.pki+"/server.pem", s.pki+"/server.key")
	if err != nil {
		t.Fatal(err)
	}
	roots, err := conn.LoadPool(s.pki + "/ca.pem")
	if err != nil {
		t.Fatal(err)
	}
	tc, err := tls.Dial("tcp", l.addr, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{c}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tc.Close() }()
	_, _ = io.WriteString(tc, "POST /notify HTTP/1.1\r\nHost: x\r\nContent-Type: application/sep+xml\r\nContent-Length: 1000\r\n\r\n<Notif")
	// The listener gives up on the body after its ReadTimeout (10 s).
	_ = tc.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, err = tc.Read(make([]byte, 1))
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatal("the listener held a stalled request open for 30 s")
	}
}
