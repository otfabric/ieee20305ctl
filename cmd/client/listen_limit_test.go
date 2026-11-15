// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-ieee20305/security"
	"github.com/otfabric/ieee20305ctl/internal/conn"
)

// With a limit of one connection, a second connection to the listener gets
// no TLS handshake until the first is closed.
func TestListenMaxConnections(t *testing.T) {
	ca, caKey, err := security.TestCA("Utility")
	if err != nil {
		t.Fatal(err)
	}
	srv, srvKey, err := security.TestServerCert(ca, caKey, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	dev, devKey, err := security.TestDeviceCert(ca, caKey, "device")
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	d := &conn.Device{Cert: keyPair(dev, devKey), Roots: roots}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	url, stop, err := listen(context.Background(), "127.0.0.1:0", d.Cert, d, false, 1, ok)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	addr := strings.TrimSuffix(strings.TrimPrefix(url, "https://"), "/notify")
	cfg := &tls.Config{
		InsecureSkipVerify: true, // the test checks the listener's limit, not its certificate
		Certificates:       []tls.Certificate{keyPair(srv, srvKey)},
		MinVersion:         tls.VersionTLS12,
	}

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
