// SPDX-License-Identifier: MIT

package cmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cert inspect shows the fields of a certificate of unknown origin escaped
// when they hold control characters: its subject, issuer and DNS names.
func TestCertInspectEscapesControlCharacters(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const cn = "device\x1b]52;c;Y2xpcA==\a\u009b"
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: cn, Organization: []string{"org\r\nSerial: extra"}},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"host\x07name", "line\nbreak"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "peer.pem")
	if err := os.WriteFile(file, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"text", "table"} {
		r := must(t, 0, "cert", "inspect", file, "--format", f)
		for _, c := range r.stdout {
			if c != '\n' && (c < 0x20 || (c >= 0x7f && c <= 0x9f)) {
				t.Fatalf("--format %s: a control character %U in\n%q", f, c, r.stdout)
			}
		}
		if f == "text" && strings.Count(r.stdout, "\nSerial:") != 1 {
			t.Errorf("--format text: an extra line in\n%s", r.stdout)
		}
	}
}
