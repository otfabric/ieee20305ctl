// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// heldWriter is the stdout of a client subscribe that holds the write of
// its first notification record until release is closed: while it is held,
// the command takes no notification from its queue.
type heldWriter struct {
	holding chan struct{} // closed when the first notification record is held
	release chan struct{}
	saw     chan struct{} // a value per record written

	mu      sync.Mutex
	held    bool
	records []queueRecord
}

type queueRecord struct {
	Event    string `json:"event"`
	Href     string `json:"href"`
	Resource string `json:"resource"`
	Address  string `json:"address"`
	Dropped  int64  `json:"dropped"`
}

// Write keeps the records; a write that is not one is taken and not kept.
func (w *heldWriter) Write(p []byte) (int, error) {
	var r queueRecord
	if json.Unmarshal(bytes.TrimSpace(p), &r) == nil {
		w.keep(r)
	}
	return len(p), nil
}

func (w *heldWriter) keep(r queueRecord) {
	w.mu.Lock()
	hold := r.Event == "notification" && !w.held
	w.held = w.held || hold
	w.mu.Unlock()
	if hold {
		close(w.holding)
		<-w.release
	}
	w.mu.Lock()
	w.records = append(w.records, r)
	w.mu.Unlock()
	w.saw <- struct{}{}
}

// waitFor waits until a record that pred matches has been written.
func (w *heldWriter) waitFor(t *testing.T, what string, pred func(queueRecord) bool) queueRecord {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		w.mu.Lock()
		for _, r := range w.records {
			if pred(r) {
				w.mu.Unlock()
				return r
			}
		}
		w.mu.Unlock()
		select {
		case <-w.saw:
		case <-timeout:
			t.Fatalf("no %s record", what)
		}
	}
}

func (w *heldWriter) count(pred func(queueRecord) bool) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, r := range w.records {
		if pred(r) {
			n++
		}
	}
	return n
}

// heldSubscribe is a client subscribe whose stdout is a heldWriter.
type heldSubscribe struct {
	w        *heldWriter
	url, sub string
	release  func()             // lets the held record be written
	cancel   context.CancelFunc // interrupts the command
	done     chan struct{}      // closed when it has exited with code
	code     int
}

// startHeldSubscribe runs client subscribe in the background, and returns
// once it has subscribed.
func startHeldSubscribe(t *testing.T, s *site) *heldSubscribe {
	t.Helper()
	w := &heldWriter{holding: make(chan struct{}), release: make(chan struct{}), saw: make(chan struct{}, 4096)}
	ctx, cancel := context.WithCancel(context.Background())
	h := &heldSubscribe{w: w, cancel: cancel, done: make(chan struct{})}
	go func() {
		h.code = Run(ctx, []string{"client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--listen-cert", s.pki + "/server.pem",
			"--listen-key", s.pki + "/server.key", "--format", "json"}, strings.NewReader(""), w, &bytes.Buffer{})
		close(h.done)
	}()
	var once sync.Once
	h.release = func() { once.Do(func() { close(w.release) }) }
	t.Cleanup(func() {
		cancel()
		h.release()
		select {
		case <-h.done:
		case <-time.After(30 * time.Second):
			t.Error("client subscribe did not stop")
		}
	})
	h.url = w.waitFor(t, "listening", func(r queueRecord) bool { return r.Event == "listening" }).Address
	h.sub = w.waitFor(t, "subscribed", func(r queueRecord) bool { return r.Event == "subscribed" }).Href
	return h
}

// holdFirst posts a notification of the command's subscription and waits
// until its record is held: the queue is then empty, and stays as the
// handler leaves it until release.
func (h *heldSubscribe) holdFirst(t *testing.T, post func(resource, sub string) int) {
	t.Helper()
	if code := post("/first", h.sub); code < 200 || code > 299 {
		t.Fatalf("the first notification: %d", code)
	}
	select {
	case <-h.w.holding:
	case <-time.After(30 * time.Second):
		t.Fatal("the first notification was not written")
	}
}

// Notifications of another subscription take no slot of the queue: with
// the command held and a queue's worth of them posted, a notification of
// its own subscription is still written.
func TestNotifyQueueFiltersBeforeTheQueue(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	h := startHeldSubscribe(t, s)
	hc := notifier(t, s.pki+"/ca.pem", s.pki+"/server.pem", s.pki+"/server.key")
	send := func(resource, sub string) int { return post(t, hc, h.url, notification(t, resource, sub)) }
	h.holdFirst(t, send)
	for i := range 16 {
		if code := send("/other/"+strconv.Itoa(i), "/edev/9/sub/42"); code < 200 || code > 299 {
			t.Fatalf("a notification of another subscription: %d", code)
		}
	}
	if code := send("/mine", h.sub); code < 200 || code > 299 {
		t.Fatalf("the notification of the subscription: %d", code)
	}
	h.release()
	h.w.waitFor(t, "notification of /mine", func(r queueRecord) bool { return r.Event == "notification" && r.Resource == "/mine" })
	if n := h.w.count(func(r queueRecord) bool { return strings.HasPrefix(r.Resource, "/other/") }); n != 0 {
		t.Errorf("%d notifications of another subscription were written", n)
	}
	if n := h.w.count(func(r queueRecord) bool { return r.Event == "notifications_dropped" }); n != 0 {
		t.Errorf("%d notifications_dropped records, want none", n)
	}
}

// A notification that finds the queue full is not written, and the record
// notifications_dropped says how many were not.
func TestNotifyQueueReportsDrops(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	h := startHeldSubscribe(t, s)
	hc := notifier(t, s.pki+"/ca.pem", s.pki+"/server.pem", s.pki+"/server.key")
	send := func(resource, sub string) int { return post(t, hc, h.url, notification(t, resource, sub)) }
	h.holdFirst(t, send)
	for i := range 16 + 3 {
		if code := send("/queued/"+strconv.Itoa(i), h.sub); code < 200 || code > 299 {
			t.Fatalf("notification %d: %d", i, code)
		}
	}
	h.release()
	r := h.w.waitFor(t, "notifications_dropped", func(r queueRecord) bool { return r.Event == "notifications_dropped" })
	if r.Dropped != 3 || r.Href != h.sub {
		t.Errorf("notifications_dropped %+v, want 3 of %s", r, h.sub)
	}
	h.w.waitFor(t, "notification of /queued/15", func(r queueRecord) bool { return r.Resource == "/queued/15" })
	if n := h.w.count(func(r queueRecord) bool { return strings.HasPrefix(r.Resource, "/queued/") }); n != 16 {
		t.Errorf("%d queued notifications written, want 16", n)
	}
	h.cancel()
	select {
	case <-h.done:
		if h.code != 0 {
			t.Errorf("subscribe interrupted: exit %d", h.code)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("client subscribe did not stop")
	}
}
