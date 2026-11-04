// SPDX-License-Identifier: MIT

//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// signals skips where a process cannot be sent SIGINT or SIGTERM.
func signals(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("no SIGINT or SIGTERM to send on Windows")
	}
}

// proc is a running ieee20305ctl.
type proc struct {
	cmd   *exec.Cmd
	lines chan map[string]any // the JSON records of stdout
	done  chan struct{}
	code  int
}

// syncBuffer is a bytes.Buffer safe for the copy goroutine and the test.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// start runs the binary with args and env, reading the JSON records it
// writes; it is killed when the test ends.
func start(t *testing.T, env []string, args ...string) (*proc, *syncBuffer) {
	t.Helper()
	bin := os.Getenv("IEEE20305CTL_E2E_BIN")
	if bin == "" {
		t.Fatal("IEEE20305CTL_E2E_BIN is not set; run make e2e")
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	all := &syncBuffer{}
	cmd.Stderr = all
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &proc{cmd: cmd, lines: make(chan map[string]any, 64), done: make(chan struct{})}
	go func() {
		sc := bufio.NewScanner(io.TeeReader(out, all))
		sc.Buffer(nil, 1<<20)
		var rec bytes.Buffer
		for sc.Scan() {
			// A one-shot command writes an indented object, a stream one
			// object per line.
			rec.Write(sc.Bytes())
			var m map[string]any
			if json.Unmarshal(rec.Bytes(), &m) == nil {
				rec.Reset()
				select {
				case p.lines <- m:
				default:
				}
			}
		}
		err := cmd.Wait()
		var exit *exec.ExitError
		switch {
		case err == nil:
		case errors.As(err, &exit):
			p.code = exit.ExitCode()
		default:
			p.code = -1
		}
		close(p.done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-p.done
	})
	return p, all
}

// wait waits for the process to exit and returns its exit code.
func (p *proc) wait(t *testing.T, out *syncBuffer) int {
	t.Helper()
	select {
	case <-p.done:
		return p.code
	case <-time.After(60 * time.Second):
		t.Fatalf("the process did not exit\n%s", out.String())
		return -1
	}
}

// record waits for a record whose event is kind.
func (p *proc) record(t *testing.T, out *syncBuffer, kind string) map[string]any {
	t.Helper()
	timeout := time.After(60 * time.Second)
	for {
		select {
		case m := <-p.lines:
			if m["event"] == kind {
				return m
			}
		case <-p.done:
			t.Fatalf("exited %d before a %s record\n%s", p.code, kind, out.String())
		case <-timeout:
			_ = p.cmd.Process.Signal(syscall.SIGQUIT)
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
			}
			t.Fatalf("no %s record\n%s", kind, out.String())
		}
	}
}

// pki generates a PKI with the binary.
func pki(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "pki")
	if out, code := run(t, "cert", "generate", "--out", dir, "--devices", "1"); code != 0 {
		t.Fatalf("cert generate: exit %d\n%s", code, out)
	}
	return dir
}

func deviceEnv(dir, url string) []string {
	return []string{"IEEE20305CTL_URL=" + url, "IEEE20305CTL_CERT=" + dir + "/device-1.pem", "IEEE20305CTL_KEY=" + dir + "/device-1.key",
		"IEEE20305CTL_CA=" + dir + "/ca.pem", "IEEE20305CTL_FORMAT=json"}
}

// serve starts server start with the PKI of dir and a seed of the device,
// and returns its address.
func serve(t *testing.T, dir string) (*proc, *syncBuffer, string) {
	t.Helper()
	seed := filepath.Join(filepath.Dir(dir), "seed.yaml")
	if err := os.WriteFile(seed, []byte("poll_rate: 5\ndevices:\n  - cert: "+dir+"/device-1.pem\n    pin: 111115\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, out := start(t, nil, "server", "start", "--listen", "127.0.0.1:0", "--cert", dir+"/server.pem", "--key", dir+"/server.key",
		"--ca", dir+"/ca.pem", "--config", seed, "--format", "json")
	addr, _ := p.record(t, out, "listening")["address"].(string)
	if addr == "" {
		t.Fatalf("no address\n%s", out.String())
	}
	return p, out, addr
}

// holder accepts connections and holds them without a byte: a server that
// stalls the TLS handshake. accepted gets one value per connection.
func holder(t *testing.T) (string, chan struct{}) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan struct{}, 16)
	var mu sync.Mutex
	var conns []net.Conn
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
			accepted <- struct{}{}
		}
	}()
	t.Cleanup(func() {
		_ = l.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	return l.Addr().String(), accepted
}

func errorCode(t *testing.T, out string) (string, string) {
	t.Helper()
	start := strings.Index(out, "{")
	var e struct {
		Error struct {
			Kind string `json:"kind"`
			Code string `json:"code"`
		} `json:"error"`
	}
	if start < 0 {
		t.Fatalf("no JSON error in\n%s", out)
	}
	if err := json.NewDecoder(strings.NewReader(out[start:])).Decode(&e); err != nil {
		t.Fatalf("not a JSON error: %v\n%s", err, out)
	}
	return e.Error.Kind, e.Error.Code
}

// SIGINT or SIGTERM to a one-shot command that waits for a server exits 130
// with context_canceled (docs/exitcodes.md), at once rather than at
// --timeout.
func TestInterruptedOneShotExits130(t *testing.T) {
	signals(t)
	dir := pki(t)
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			addr, accepted := holder(t)
			p, out := start(t, deviceEnv(dir, addr), "client", "dcap", "--timeout", "10m")
			select {
			case <-accepted:
			case <-time.After(60 * time.Second):
				t.Fatal("the client did not connect")
			}
			if err := p.cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			if code := p.wait(t, out); code != 130 {
				t.Fatalf("exit %d, want 130\n%s", code, out.String())
			}
			if kind, code := errorCode(t, out.String()); kind != "interrupted" || code != "context_canceled" {
				t.Fatalf("%s/%s\n%s", kind, code, out.String())
			}
		})
	}
}

// A server that stalls the handshake makes a one-shot command exit 5
// within its --timeout.
func TestStalledHandshakeTimesOut(t *testing.T) {
	dir := pki(t)
	addr, _ := holder(t)
	out, code := runEnv(t, deviceEnv(dir, addr), "client", "dcap", "--timeout", "1s")
	if code != 5 {
		t.Fatalf("exit %d, want 5\n%s", code, out)
	}
	if _, c := errorCode(t, out); c != "request_timeout" {
		t.Fatalf("code %s\n%s", c, out)
	}
}

// Streaming commands end normally on SIGINT and SIGTERM: exit 0. server
// start drains, client subscribe deletes its subscription.
func TestInterruptedStreamsExit0(t *testing.T) {
	signals(t)
	dir := pki(t)
	srv, srvOut, addr := serve(t, dir)
	env := deviceEnv(dir, addr)
	if out, code := runEnv(t, env, "client", "register", "--pin", "111115"); code != 0 {
		t.Fatalf("register: exit %d\n%s", code, out)
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run("run/"+sig.String(), func(t *testing.T) {
			p, out := start(t, env, "client", "run", "--pin", "111115")
			p.record(t, out, "started")
			_ = p.cmd.Process.Signal(sig)
			if code := p.wait(t, out); code != 0 {
				t.Fatalf("client run: exit %d\n%s", code, out.String())
			}
		})
	}
	if err := srv.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := srv.wait(t, srvOut); code != 0 {
		t.Fatalf("server start after SIGTERM: exit %d\n%s", code, srvOut.String())
	}
}

// A server that goes away: a one-shot command exits 4; client run keeps
// going through it and ends normally when interrupted.
func TestServerGoesAway(t *testing.T) {
	signals(t)
	dir := pki(t)
	srv, srvOut, addr := serve(t, dir)
	env := deviceEnv(dir, addr)
	if out, code := runEnv(t, env, "client", "register", "--pin", "111115"); code != 0 {
		t.Fatalf("register: exit %d\n%s", code, out)
	}
	agent, agentOut := start(t, env, "client", "run", "--pin", "111115", "--poll-rate", "1s", "--polls")
	agent.record(t, agentOut, "started")
	agent.record(t, agentOut, "poll") // the agent has started: it polls
	_ = srv.cmd.Process.Kill()
	srv.wait(t, srvOut)
	out, code := runEnv(t, env, "client", "dcap")
	if code != 4 {
		t.Fatalf("client dcap of a server gone: exit %d\n%s", code, out)
	}
	if _, c := errorCode(t, out); c != "transport_connect_failed" {
		t.Fatalf("code %s", c)
	}
	agent.record(t, agentOut, "poll_failed")
	_ = agent.cmd.Process.Signal(syscall.SIGINT)
	if code := agent.wait(t, agentOut); code != 0 {
		t.Fatalf("client run after the server went away: exit %d\n%s", code, agentOut.String())
	}
}

// cutter is a TCP proxy to upstream that cuts each connection after it
// has relayed n bytes from upstream: the TLS handshake, or the answer, is
// cut short.
func cutter(t *testing.T, upstream string, n int64) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				u, err := net.Dial("tcp", upstream)
				if err != nil {
					return
				}
				defer func() { _ = u.Close() }()
				go func() { _, _ = io.Copy(u, c) }()
				_, _ = io.CopyN(c, u, n)
			}()
		}
	}()
	return l.Addr().String()
}

// Connections cut at any point exit 4 (no conversation) or 6 (an answer
// that does not decode), never 0, 10 or a crash.
func TestConnectionCuts(t *testing.T) {
	dir := pki(t)
	_, _, addr := serve(t, dir)
	// The whole exchange of client dcap, to cut it short of.
	whole, _ := runEnv(t, deviceEnv(dir, cutter(t, addr, 1<<30)), "client", "dcap")
	if !strings.Contains(whole, "EndDeviceListLink") {
		t.Fatalf("through the proxy: %s", whole)
	}
	for _, n := range []int64{0, 1, 100, 500, 1000} {
		proxy := cutter(t, addr, n)
		out, code := runEnv(t, deviceEnv(dir, proxy), "client", "dcap", "--timeout", "20s")
		whole := code == 0 && strings.Contains(out, "EndDeviceListLink") // n was enough for all of it
		if code != 4 && code != 6 && !whole {
			t.Errorf("a connection cut after %d bytes: exit %d\n%s", n, code, out)
		}
	}
}

// runEnv runs the binary with env added to the environment.
func runEnv(t *testing.T, env []string, args ...string) (string, int) {
	t.Helper()
	bin := os.Getenv("IEEE20305CTL_E2E_BIN")
	var out bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out.String(), 0
	case errors.As(err, &exit):
		return out.String(), exit.ExitCode()
	default:
		t.Fatalf("run %s: %v", bin, err)
		return "", -1
	}
}
