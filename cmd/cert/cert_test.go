// SPDX-License-Identifier: MIT

package cert

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-ieee20305/security"
	"github.com/otfabric/ieee20305ctl/internal/errs"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewCertCmd()
	// As the root command does.
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// decode reads the data of a JSON result envelope into v.
func decode(t *testing.T, out string, v any) {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || env.Status != "ok" {
		t.Fatalf("not a successful JSON result (%v): %s", err, out)
	}
	if err := json.Unmarshal(env.Data, v); err != nil {
		t.Fatal(err)
	}
}

func exitCode(err error) int {
	var coded *errs.ExitCodeError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return -1
}

func generated(t *testing.T, extra ...string) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := run(t, append([]string{"generate", "--out", dir}, extra...)...); err != nil {
		t.Fatalf("generate: %v", err)
	}
	return dir
}

func TestGenerateFiles(t *testing.T) {
	dir := generated(t, "--devices", "2")
	for _, name := range []string{"ca", "server", "device-1", "device-2"} {
		if _, err := LoadCertificate(filepath.Join(dir, name+".pem")); err != nil {
			t.Errorf("%s.pem: %v", name, err)
		}
		fi, err := os.Stat(filepath.Join(dir, name+".key"))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s.key has mode %v, want 0600", name, fi.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "device-3.pem")); err == nil {
		t.Error("an extra device was generated")
	}
}

func TestGenerateRefusesToOverwrite(t *testing.T) {
	dir := generated(t)
	before, _ := os.ReadFile(filepath.Join(dir, "ca.pem"))
	_, err := run(t, "generate", "--out", dir)
	if exitCode(err) != errs.ExitUsage || !strings.Contains(err.Error(), "--force") {
		t.Errorf("err = %v, want a usage error mentioning --force", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if !bytes.Equal(before, after) {
		t.Error("ca.pem was overwritten")
	}
	if _, err := run(t, "generate", "--out", dir, "--force"); err != nil {
		t.Errorf("--force: %v", err)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "ca.pem")); bytes.Equal(before, after) {
		t.Error("--force did not regenerate ca.pem")
	}
}

func TestGenerateRejectsBadFlags(t *testing.T) {
	for _, args := range [][]string{
		{"generate", "--out", t.TempDir(), "--devices", "0"},
		{"generate", "--out", t.TempDir(), "--server-name", ""},
		{"generate", "--out", t.TempDir(), "-o", "xml"},
	} {
		if _, err := run(t, args...); exitCode(err) != errs.ExitUsage {
			t.Errorf("%v: err = %v, want a usage error", args, err)
		}
	}
}

// TestGeneratedPKIWorksForMutualTLS completes a TLS 1.2 handshake with the
// generated server and device certificates, as IEEE 2030.5 requires.
func TestGeneratedPKIWorksForMutualTLS(t *testing.T) {
	dir := generated(t)
	ca, err := LoadCertificate(filepath.Join(dir, "ca.pem"))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	srvCert, err := tls.LoadX509KeyPair(filepath.Join(dir, "server.pem"), filepath.Join(dir, "server.key"))
	if err != nil {
		t.Fatal(err)
	}
	devCert, err := tls.LoadX509KeyPair(filepath.Join(dir, "device-1.pem"), filepath.Join(dir, "device-1.key"))
	if err != nil {
		t.Fatal(err)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MaxVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	peer := make(chan *x509.Certificate, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			peer <- nil
			return
		}
		defer func() { _ = c.Close() }()
		tc := c.(*tls.Conn)
		if err := tc.Handshake(); err != nil {
			peer <- nil
			return
		}
		peer <- tc.ConnectionState().PeerCertificates[0]
	}()

	// The client verifies the server as 127.0.0.1, an IP SAN.
	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		Certificates: []tls.Certificate{devCert},
		RootCAs:      pool,
		MaxVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	_ = conn.Close()
	select {
	case got := <-peer:
		if got == nil {
			t.Fatal("server handshake failed")
		}
		if security.CertificateLFDI(got) != security.CertificateLFDI(devCert.Leaf) {
			t.Error("the server saw a different device certificate")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
	if err := srvCert.Leaf.VerifyHostname("localhost"); err != nil {
		t.Error(err)
	}
}

func TestLFDI(t *testing.T) {
	dir := generated(t)
	path := filepath.Join(dir, "device-1.pem")
	c, err := LoadCertificate(path)
	if err != nil {
		t.Fatal(err)
	}
	lfdi := security.CertificateLFDI(c)

	out, err := run(t, "lfdi", path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "LFDI: " + lfdi.String() + "\nSFDI: " + lfdi.SFDI().String() + "\n"; out != want {
		t.Errorf("text output = %q, want %q", out, want)
	}

	out, err = run(t, "lfdi", path, "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var id Identity
	decode(t, out, &id)
	if id.LFDI != lfdi.String() || id.SFDI != uint64(lfdi.SFDI()) {
		t.Errorf("json output = %s", out)
	}
	if !security.SFDI(id.SFDI).Valid() {
		t.Error("SFDI check digit invalid")
	}
}

func TestJSONError(t *testing.T) {
	dir := generated(t)
	out, err := run(t, "lfdi", filepath.Join(dir, "ca.key"), "-o", "json")
	if exitCode(err) != errs.ExitCertificate {
		t.Errorf("err = %v, want a certificate error", err)
	}
	var env struct{ Status, Error string }
	if json.Unmarshal([]byte(out), &env) != nil || env.Status != "error" || !strings.Contains(env.Error, "no CERTIFICATE block") {
		t.Errorf("JSON error output = %s", out)
	}
}

func TestGenerateJSON(t *testing.T) {
	dir := t.TempDir()
	out, err := run(t, "generate", "--out", dir, "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var g Generated
	decode(t, out, &g)
	if g.Dir != dir || len(g.Files) != 3 || g.Files[2].Role != "device" || len(g.Files[2].LFDI) != 40 {
		t.Errorf("generate JSON = %s", out)
	}
}

func TestInspect(t *testing.T) {
	dir := generated(t, "--server-name", "sep.example,10.0.0.1")
	out, err := run(t, "inspect", filepath.Join(dir, "server.pem"), "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var info Info
	decode(t, out, &info)
	if info.IsCA || info.Expired || info.PublicKey != "ECDSA P-256" ||
		strings.Join(info.DNSNames, ",") != "sep.example" || strings.Join(info.IPAddresses, ",") != "10.0.0.1" ||
		strings.Join(info.ExtKeyUsages, ",") != "server auth" || len(info.LFDI) != 40 {
		t.Errorf("info = %+v", info)
	}

	out, err = run(t, "inspect", filepath.Join(dir, "ca.pem"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Subject:", "CN=IEEE 2030.5 Test CA", "CA:", "true", "LFDI:", "SFDI:"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
}

func TestInspectReportsInvalidPeriod(t *testing.T) {
	ca, _, err := security.TestCA("Old")
	if err != nil {
		t.Fatal(err)
	}
	if info := inspect(ca, ca.NotAfter.Add(time.Hour)); !info.Expired {
		t.Error("a certificate past NotAfter is reported valid")
	}
	if info := inspect(ca, ca.NotBefore.Add(-time.Hour)); !info.Expired {
		t.Error("a certificate before NotBefore is reported valid")
	}
}

func TestLoadCertificate(t *testing.T) {
	dir := generated(t)
	pemPath := filepath.Join(dir, "ca.pem")
	c, err := LoadCertificate(pemPath)
	if err != nil {
		t.Fatal(err)
	}
	derPath := filepath.Join(dir, "ca.der")
	if err := os.WriteFile(derPath, c.Raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if d, err := LoadCertificate(derPath); err != nil || !d.Equal(c) {
		t.Errorf("DER: %v", err)
	}
	// A key before the certificate in one PEM file.
	key, _ := os.ReadFile(filepath.Join(dir, "ca.key"))
	cert, _ := os.ReadFile(pemPath)
	bundle := filepath.Join(dir, "bundle.pem")
	if err := os.WriteFile(bundle, append(key, cert...), 0o600); err != nil {
		t.Fatal(err)
	}
	if d, err := LoadCertificate(bundle); err != nil || !d.Equal(c) {
		t.Errorf("bundle: %v", err)
	}
	for _, bad := range []string{filepath.Join(dir, "ca.key"), filepath.Join(dir, "missing.pem")} {
		if _, err := LoadCertificate(bad); exitCode(err) != errs.ExitCertificate {
			t.Errorf("%s: err = %v, want a certificate error", bad, err)
		}
	}
	junk := filepath.Join(dir, "junk")
	_ = os.WriteFile(junk, []byte("not a certificate"), 0o644)
	if _, err := LoadCertificate(junk); exitCode(err) != errs.ExitCertificate {
		t.Errorf("junk: err = %v", err)
	}
}
