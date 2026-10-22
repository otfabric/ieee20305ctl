// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/security"
	"github.com/otfabric/ieee20305ctl/internal/errs"
)

// testPKI writes a CA, a server certificate for 127.0.0.1 and a device
// certificate to a directory.
type testPKI struct {
	dir    string
	pool   *x509.CertPool
	server tls.Certificate
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeKey(t *testing.T, path string, key *ecdsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	writePEM(t, path, "PRIVATE KEY", der)
}

func newTestPKI(t *testing.T) testPKI {
	t.Helper()
	dir := t.TempDir()
	ca, caKey, err := security.TestCA("Test")
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
	writePEM(t, filepath.Join(dir, "ca.pem"), "CERTIFICATE", ca.Raw)
	writePEM(t, filepath.Join(dir, "device.pem"), "CERTIFICATE", dev.Raw)
	writeKey(t, filepath.Join(dir, "device.key"), devKey)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return testPKI{dir: dir, pool: pool, server: tls.Certificate{Certificate: [][]byte{srv.Raw}, PrivateKey: srvKey, Leaf: srv}}
}

func (p testPKI) flags(host string) []string {
	return []string{"--host", host, "--cert", filepath.Join(p.dir, "device.pem"), "--key", filepath.Join(p.dir, "device.key"), "--ca", filepath.Join(p.dir, "ca.pem")}
}

func startServer(t *testing.T, p testPKI, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = security.ServerTLSConfig(p.server, p.pool)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func dcapHandler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dcap" {
			http.NotFound(w, r)
			return
		}
		var dc ieee20305.DeviceCapability
		all := ieee20305.UInt32(3)
		dc.EndDeviceListLink = &ieee20305.EndDeviceListLink{}
		dc.EndDeviceListLink.Href, dc.EndDeviceListLink.All = "/edev", &all
		dc.TimeLink = &ieee20305.TimeLink{}
		dc.TimeLink.Href = "/tm"
		dc.SelfDeviceLink = &ieee20305.SelfDeviceLink{}
		dc.SelfDeviceLink.Href = "/sdev"
		data, err := ieee20305.Marshal(&dc)
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/sep+xml")
		_, _ = w.Write(data)
	}
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewClientCmd()
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func exitCode(err error) int {
	var c *errs.ExitCodeError
	if errors.As(err, &c) {
		return c.Code
	}
	return -1
}

func TestDiscover(t *testing.T) {
	p := newTestPKI(t)
	srv := startServer(t, p, dcapHandler(t))
	host := strings.TrimPrefix(srv.URL, "https://")

	out, err := run(t, append([]string{"discover"}, p.flags(host)...)...)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DeviceCapability: " + srv.URL + "/dcap", "Poll rate:        900 s", "TimeLink", "/tm", "EndDeviceListLink", "/edev", "3", "SelfDeviceLink"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	out, err = run(t, append([]string{"discover", "-o", "json"}, p.flags(srv.URL)...)...)
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Status string
		Data   Discovery
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	d := env.Data
	if env.Status != "ok" || d.PollRate != 900 || len(d.Links) != 3 {
		t.Fatalf("JSON = %s", out)
	}
	// Links are in schema order, inherited ones first.
	names := []string{d.Links[0].Name, d.Links[1].Name, d.Links[2].Name}
	if strings.Join(names, ",") != "TimeLink,EndDeviceListLink,SelfDeviceLink" {
		t.Errorf("link order = %v", names)
	}
	if d.Links[1].All == nil || *d.Links[1].All != 3 || d.Links[0].All != nil {
		t.Errorf("list counts = %+v", d.Links)
	}
}

func TestDiscoverFailures(t *testing.T) {
	p := newTestPKI(t)
	srv := startServer(t, p, dcapHandler(t))
	host := strings.TrimPrefix(srv.URL, "https://")
	other := newTestPKI(t)

	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadAddr := closed.Addr().String()
	_ = closed.Close()

	otherDevice := []string{"--host", host, "--cert", filepath.Join(other.dir, "device.pem"), "--key", filepath.Join(other.dir, "device.key"), "--ca", filepath.Join(p.dir, "ca.pem")}
	otherCA := []string{"--host", host, "--cert", filepath.Join(p.dir, "device.pem"), "--key", filepath.Join(p.dir, "device.key"), "--ca", filepath.Join(other.dir, "ca.pem")}

	tests := []struct {
		name string
		args []string
		code int
	}{
		{"no host", []string{"--cert", "x", "--key", "y"}, errs.ExitUsage},
		{"http URL", p.flags("http://" + host), errs.ExitUsage},
		{"no certificate", []string{"--host", host}, errs.ExitUsage},
		{"missing key", []string{"--host", host, "--cert", filepath.Join(p.dir, "device.pem"), "--key", filepath.Join(p.dir, "nope.key")}, errs.ExitCertificate},
		{"bad CA file", []string{"--host", host, "--cert", filepath.Join(p.dir, "device.pem"), "--key", filepath.Join(p.dir, "device.key"), "--ca", filepath.Join(p.dir, "device.key")}, errs.ExitCertificate},
		{"server of another CA", otherCA, errs.ExitCertificate},
		{"device of another CA", otherDevice, errs.ExitCertificate},
		{"nothing listening", p.flags(deadAddr), errs.ExitConnect},
		{"no DeviceCapability", append(p.flags(host), "--dcap", "/nope"), errs.ExitError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := run(t, append([]string{"discover"}, tt.args...)...)
			if got := exitCode(err); got != tt.code {
				t.Errorf("exit code %d (%v), want %d", got, err, tt.code)
			}
		})
	}
}

func TestServerURL(t *testing.T) {
	for in, want := range map[string]string{
		"utility.example":             "https://utility.example:443",
		"utility.example:8443":        "https://utility.example:8443",
		"https://utility.example/":    "https://utility.example",
		"https://utility.example/sep": "https://utility.example/sep",
		"[::1]:8443":                  "https://[::1]:8443",
	} {
		if got, err := serverURL(in); err != nil || got != want {
			t.Errorf("serverURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
