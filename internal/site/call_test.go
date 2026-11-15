// SPDX-License-Identifier: MIT

package site

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/otfabric/ieee20305ctl/internal/errs"
)

// heldSocket serves a Unix socket that reads one request per connection
// and then, unless closeAfterRead, keeps the connection open without a
// reply, as a server busy elsewhere does. Each request read is reported on
// the channel.
func heldSocket(t *testing.T, closeAfterRead bool) (string, <-chan struct{}) {
	t.Helper()
	dir, err := os.MkdirTemp("", "i2c") // a socket path must be short
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "c.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	read := make(chan struct{}, 16)
	var mu sync.Mutex
	var open []net.Conn
	t.Cleanup(func() {
		_ = l.Close()
		mu.Lock()
		for _, c := range open {
			_ = c.Close()
		}
		mu.Unlock()
		_ = os.RemoveAll(dir)
	})
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			open = append(open, c)
			mu.Unlock()
			go func() {
				if _, err := bufio.NewReader(c).ReadString('\n'); err == nil {
					read <- struct{}{}
				}
				if closeAfterRead {
					_ = c.Close()
				}
			}()
		}
	}()
	return path, read
}

func kindOf(t *testing.T, err error) *errs.Error {
	t.Helper()
	e, ok := errs.As(err)
	if !ok {
		t.Fatalf("%v is not an errs.Error", err)
	}
	return e
}

// A request the server took and does not answer before the deadline is a
// timeout, exit 5: outcome_unknown for an operation that may have changed
// the server, request_timeout for one that changes nothing.
func TestCallTimeout(t *testing.T) {
	path, _ := heldSocket(t, false)
	for _, c := range []struct {
		op   string
		code string
	}{
		{"issue-control", errs.CodeOutcomeUnknown},
		{"cancel", errs.CodeOutcomeUnknown},
		{"set-default", errs.CodeOutcomeUnknown},
		{"provision", errs.CodeOutcomeUnknown},
		{"register", errs.CodeOutcomeUnknown},
		{"devices", errs.CodeRequestTimeout},
		{"get", errs.CodeRequestTimeout},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		err := Call(ctx, path, Request{Op: c.op}, nil)
		cancel()
		e := kindOf(t, err)
		if e.Kind != errs.KindTimeout || e.Code != c.code || errs.ExitCodeForKind(e.Kind) != 5 {
			t.Errorf("%s without a reply: %s/%s, exit %d; want timeout/%s, exit 5", c.op, e.Kind, e.Code, errs.ExitCodeForKind(e.Kind), c.code)
		}
	}
}

// An interrupt while the command waits for the reply ends the wait at once:
// interrupted, exit 130.
func TestCallInterrupted(t *testing.T) {
	path, read := heldSocket(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Call(ctx, path, Request{Op: "issue-control"}, nil) }()
	<-read // the server has the request
	cancel()
	select {
	case err := <-done:
		e := kindOf(t, err)
		if e.Kind != errs.KindInterrupted || errs.ExitCodeForKind(e.Kind) != 130 {
			t.Errorf("an interrupted call: %s/%s, exit %d; want interrupted, exit 130", e.Kind, e.Code, errs.ExitCodeForKind(e.Kind))
		}
	case <-time.After(5 * time.Second): // far below the 20 s deadline
		t.Fatal("the call did not return within 5 s of the interrupt")
	}
}

// A socket that cannot be opened, or that closes without a reply, is
// control_socket_failed, exit 4, as before.
func TestCallSocketFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	closing, _ := heldSocket(t, true)
	for name, path := range map[string]string{
		"a missing socket":             filepath.Join(t.TempDir(), "none.sock"),
		"a socket that closes at once": closing,
	} {
		e := kindOf(t, Call(ctx, path, Request{Op: "issue-control"}, nil))
		if e.Kind != errs.KindTransport || e.Code != errs.CodeControlSocketFailed || errs.ExitCodeForKind(e.Kind) != 4 {
			t.Errorf("%s: %s/%s, exit %d; want transport/control_socket_failed, exit 4", name, e.Kind, e.Code, errs.ExitCodeForKind(e.Kind))
		}
	}
}
