// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// result is what a run of ieee20305ctl wrote and returned.
type result struct {
	stdout, stderr string
	code           int
}

// run runs ieee20305ctl with args, as the process does.
func run(t *testing.T, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	code := Run(ctx, args, strings.NewReader(""), &out, &errb)
	return result{out.String(), errb.String(), code}
}

// must runs args and fails the test unless it exits with code.
func must(t *testing.T, code int, args ...string) result {
	t.Helper()
	r := run(t, args...)
	if r.code != code {
		t.Fatalf("ieee20305ctl %s: exit %d, want %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), r.code, code, r.stdout, r.stderr)
	}
	return r
}

// decode decodes the JSON of a result.
func decode[T any](t *testing.T, r result) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, r.stdout)
	}
	return v
}

// event is a record of a streaming command.
type event struct {
	Event    string `json:"event"`
	Href     string `json:"href"`
	Resource string `json:"resource"`
	Status   string `json:"status"`
	MRID     string `json:"mrid"`
	Address  string `json:"address"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Settings []struct {
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"settings"`
}

// events decodes JSON Lines.
func events(t *testing.T, s string) []event {
	t.Helper()
	var out []event
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line == "" {
			continue
		}
		var e event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("not a JSON line: %v\n%s", err, line)
		}
		out = append(out, e)
	}
	return out
}

func has(evs []event, pred func(event) bool) bool {
	for _, e := range evs {
		if pred(e) {
			return true
		}
	}
	return false
}

// site is a server start of the tests, with its PKI and control socket.
type site struct {
	dir, pki, sock, url string

	mu     sync.Mutex
	events []event
	done   chan int
	cancel context.CancelFunc
}

const seedYAML = `poll_rate: 1
devices:
  - cert: %PKI%/device-1.pem
    pin: 111115
programs:
  - description: Utility program
    primacy: 1
    default:
      opModConnect: true
      opModMaxLimW: 100
    controls:
      - description: Limit to 50 %
        set:
          opModMaxLimW: 50
        start: 0s
        duration: 1h
      - description: Disconnect
        set:
          opModConnect: false
        start: 1h
        duration: 30m
tariffs:
  - description: Time of use
    currency: 36
    price_multiplier: -4
    primacy: 1
    rate_code: TOU1
    rate_components:
      - description: Import
        role_flags: "0001"
        flow_direction: 1
        intervals:
          - description: Off-peak
            start: 0s
            duration: 1h
            price: 1500
`

// startSite generates a PKI, and serves a seeded server with a control
// socket until the test ends.
func startSite(t *testing.T, seed string, extra ...string) *site {
	t.Helper()
	dir := t.TempDir()
	s := &site{dir: dir, pki: filepath.Join(dir, "pki"), done: make(chan int, 1)}
	must(t, 0, "cert", "generate", "--out", s.pki, "--devices", "2")
	if seed == "" {
		seed = seedYAML
	}
	seedFile := filepath.Join(dir, "seed.yaml")
	if err := os.WriteFile(seedFile, []byte(strings.ReplaceAll(seed, "%PKI%", s.pki)), 0o644); err != nil {
		t.Fatal(err)
	}
	// A Unix socket path must be short: not under the test's temp dir.
	sockDir, err := os.MkdirTemp("", "i2c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	s.sock = filepath.Join(sockDir, "c.sock")
	args := append([]string{"server", "start", "--listen", "127.0.0.1:0", "--cert", s.pki + "/server.pem", "--key", s.pki + "/server.key",
		"--ca", s.pki + "/ca.pem", "--config", seedFile, "--control-socket", s.sock, "--notify-loopback", "--format", "json"}, extra...)
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	var errb bytes.Buffer
	go func() {
		code := Run(ctx, args, strings.NewReader(""), pw, &errb)
		_ = pw.Close()
		s.done <- code
	}()
	listening := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			var e event
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue
			}
			s.mu.Lock()
			s.events = append(s.events, e)
			s.mu.Unlock()
			if e.Event == "listening" {
				listening <- e.Address
			}
		}
	}()
	select {
	case addr := <-listening:
		s.url = "https://" + addr + "/dcap"
	case code := <-s.done:
		t.Fatalf("server start exited %d: %s", code, errb.String())
	case <-time.After(30 * time.Second):
		t.Fatal("server start did not listen")
	}
	// The control socket is up once the seed is applied.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(s.sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no control socket")
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() {
		cancel()
		select {
		case code := <-s.done:
			if code != 0 {
				t.Errorf("server start exited %d: %s", code, errb.String())
			}
		case <-time.After(10 * time.Second):
			t.Error("server start did not stop")
		}
	})
	return s
}

// device sets the environment of the client commands to device n.
func (s *site) device(t *testing.T, n string) {
	t.Setenv("IEEE20305CTL_URL", s.url)
	t.Setenv("IEEE20305CTL_CERT", s.pki+"/device-"+n+".pem")
	t.Setenv("IEEE20305CTL_KEY", s.pki+"/device-"+n+".key")
	t.Setenv("IEEE20305CTL_CA", s.pki+"/ca.pem")
	t.Setenv("IEEE20305CTL_CONTROL_SOCKET", s.sock)
}

// seen reports whether the server wrote an event pred matches, waiting a
// little for it.
func (s *site) seen(pred func(event) bool) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		ok := has(s.events, pred)
		s.mu.Unlock()
		if ok {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
