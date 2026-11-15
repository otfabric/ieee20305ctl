// SPDX-License-Identifier: MIT

package cmd

import (
	"crypto/tls"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/ieee20305ctl/internal/conn"
)

// server start --max-connections 1: a second connection gets no TLS
// handshake while the first is open, and gets it once the first closes.
func TestServerMaxConnections(t *testing.T) {
	s := startSite(t, "", "--max-connections", "1")
	addr := strings.TrimSuffix(strings.TrimPrefix(s.url, "https://"), "/dcap")
	cert, err := conn.LoadKeyPair(s.pki+"/device-1.pem", s.pki+"/device-1.key")
	if err != nil {
		t.Fatal(err)
	}
	roots, err := conn.LoadPool(s.pki + "/ca.pem")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}, ServerName: "127.0.0.1"}
	first, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	second := tls.Client(raw, cfg)
	defer func() { _ = second.Close() }()
	handshake := make(chan error, 1)
	go func() { handshake <- second.Handshake() }()
	select {
	case err := <-handshake:
		t.Fatalf("a second connection got its handshake while the first was open: %v", err)
	case <-time.After(300 * time.Millisecond): // a bounded look for what must not happen
	}
	_ = first.Close()
	select {
	case err := <-handshake:
		if err != nil {
			t.Fatalf("the second connection after the first closed: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("no handshake for the second connection after the first closed")
	}
}

// The connection limits are on by default: 256 for server start, 64 for
// the notification listeners.
func TestMaxConnectionsDefaults(t *testing.T) {
	root := NewRootCmd()
	for _, c := range []struct {
		path []string
		flag string
		want string
	}{
		{[]string{"server", "start"}, "max-connections", "256"},
		{[]string{"client", "subscribe"}, "listen-max-connections", "64"},
		{[]string{"client", "run"}, "notify-max-connections", "64"},
	} {
		cmd, _, err := root.Find(c.path)
		if err != nil {
			t.Fatal(err)
		}
		f := cmd.Flags().Lookup(c.flag)
		if f == nil {
			t.Fatalf("%s has no --%s", strings.Join(c.path, " "), c.flag)
		}
		if f.DefValue != c.want {
			t.Errorf("%s --%s defaults to %s, want %s", strings.Join(c.path, " "), c.flag, f.DefValue, c.want)
		}
	}
}

// A negative connection limit is invalid input: exit 3, nothing listens.
func TestMaxConnectionsNegative(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	for _, args := range [][]string{
		{"server", "start", "--listen", "127.0.0.1:0", "--cert", s.pki + "/server.pem", "--key", s.pki + "/server.key", "--ca", s.pki + "/ca.pem",
			"--max-connections", "-1"},
		{"client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--listen-max-connections", "-1"},
		{"client", "run", "--notify-listen", "127.0.0.1:0", "--notify-max-connections", "-1"},
	} {
		if r := run(t, args...); r.code != 3 || !strings.Contains(r.stderr, "max-connections") {
			t.Errorf("%s with a negative limit: %+v", strings.Join(args[:2], " "), r)
		}
	}
}
