// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-ieee20305/security"
	"github.com/otfabric/ieee20305ctl/internal/conn"
)

func keyPair(c *x509.Certificate, k *ecdsa.PrivateKey) tls.Certificate {
	return tls.Certificate{Certificate: [][]byte{c.Raw}, PrivateKey: k, Leaf: c}
}

// post posts to the listener at url with the client certificates certs,
// and returns the status, or the error of the request.
func post(t *testing.T, url string, certs []tls.Certificate) (int, error) {
	t.Helper()
	c := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true, // the test checks the listener's checks of its peer, not its certificate
		Certificates:       certs,
		MinVersion:         tls.VersionTLS12,
	}}}
	resp, err := c.Post(url, "application/sep+xml", strings.NewReader("<x/>"))
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// The listener takes notifications only from peers with a certificate of
// the device's CAs, unless anonymous peers are allowed.
func TestListenPeerCertificates(t *testing.T) {
	ca, caKey, err := security.TestCA("Utility")
	if err != nil {
		t.Fatal(err)
	}
	other, otherKey, err := security.TestCA("Other")
	if err != nil {
		t.Fatal(err)
	}
	srv, srvKey, err := security.TestServerCert(ca, caKey, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	foreign, foreignKey, err := security.TestServerCert(other, otherKey, []string{"127.0.0.1"})
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

	for _, anonymous := range []bool{false, true} {
		url, stop, err := listen(context.Background(), "127.0.0.1:0", d.Cert, d, anonymous, ok)
		if err != nil {
			t.Fatal(err)
		}
		if status, err := post(t, url, []tls.Certificate{keyPair(srv, srvKey)}); err != nil || status != http.StatusCreated {
			t.Errorf("anonymous=%v, the server's certificate: %d, %v", anonymous, status, err)
		}
		if _, err := post(t, url, []tls.Certificate{keyPair(foreign, foreignKey)}); err == nil {
			t.Errorf("anonymous=%v: a certificate of another CA was taken", anonymous)
		}
		status, err := post(t, url, nil)
		if anonymous && (err != nil || status != http.StatusCreated) {
			t.Errorf("anonymous allowed, no certificate: %d, %v", status, err)
		}
		if !anonymous && err == nil {
			t.Errorf("a peer without a certificate was taken: %d", status)
		}
		stop()
	}
}

func TestScale(t *testing.T) {
	for _, c := range []struct {
		in   int64
		v    int64
		mult int8
	}{{7000, 7000, 0}, {-32768, -32768, 0}, {327670, 32767, 1}, {7_000_000, 7000, 3}} {
		v, m, err := scale16(c.in)
		if err != nil || int64(v) != c.v || int8(m) != c.mult {
			t.Errorf("scale16(%d) = %d e%d, %v; want %d e%d", c.in, v, m, err, c.v, c.mult)
		}
	}
	if v, m := scale48(1 << 50); int64(v) > 1<<46 || m < 1 {
		t.Errorf("scale48(2^50) = %d e%d", v, m)
	}
	if v, m := scale48(-(1 << 62)); int64(v) < -(1<<46) || m < 1 {
		t.Errorf("scale48(-2^62) = %d e%d", v, m)
	}
}
