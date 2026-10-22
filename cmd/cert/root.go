// SPDX-License-Identifier: MIT

// Package cert is the certificate commands of ieee20305ctl: a test PKI,
// and the IEEE 2030.5 identity and verification of certificates.
package cert

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/go-ieee20305/security"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/runner"
)

// NewCmd returns the cert command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Make, inspect and verify IEEE 2030.5 certificates",
		Long: `IEEE 2030.5 devices and servers authenticate each other with ECDSA P-256 certificates.
A device is known by the LFDI and SFDI of its certificate's fingerprint, and its
certificate may name its hardware module (HardwareModuleName). These commands make a
test PKI, show the identity of a certificate, and verify a chain as an IEEE 2030.5 peer
does. They work offline.`,
	}
	cmd.AddCommand(newGenerateCmd(), newInspectCmd(), newLFDICmd(), newVerifyCmd())
	return cmd
}

// Identity is the IEEE 2030.5 identity of a certificate.
type Identity struct {
	LFDI string `json:"lfdi"`
	SFDI string `json:"sfdi"`
}

func identity(c *x509.Certificate) Identity {
	lfdi := security.CertificateLFDI(c)
	return Identity{LFDI: lfdi.String(), SFDI: lfdi.SFDI().String()}
}

func (i *Identity) MarshalTextOutput() (string, error) {
	return fmt.Sprintf("LFDI: %s\nSFDI: %s", i.LFDI, i.SFDI), nil
}

func (i *Identity) TableHeaders() []string { return []string{"LFDI", "SFDI"} }
func (i *Identity) TableRows() [][]string  { return [][]string{{i.LFDI, i.SFDI}} }

// GeneratedFile is a certificate written by cert generate.
type GeneratedFile struct {
	Role string `json:"role"`
	Name string `json:"name"`
	Cert string `json:"cert"`
	Key  string `json:"key"`
	Identity
}

// Generated is the result of cert generate.
type Generated struct {
	Dir   string          `json:"dir"`
	Files []GeneratedFile `json:"files"`
}

func (g *Generated) MarshalTextOutput() (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Wrote to %s:\n\n", g.Dir)
	b.WriteString(table(g.TableHeaders(), g.TableRows()))
	return b.String(), nil
}

func (g *Generated) TableHeaders() []string {
	return []string{"ROLE", "CERTIFICATE", "KEY", "LFDI", "SFDI"}
}
func (g *Generated) TableRows() [][]string {
	rows := [][]string{}
	for _, f := range g.Files {
		rows = append(rows, []string{f.Role, filepath.Base(f.Cert), filepath.Base(f.Key), f.LFDI, f.SFDI})
	}
	return rows
}

func table(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
		for _, r := range rows {
			widths[i] = max(widths[i], len(r[i]))
		}
	}
	var b strings.Builder
	line := func(cells []string) {
		var l strings.Builder
		for i, c := range cells {
			fmt.Fprintf(&l, "%-*s  ", widths[i], c)
		}
		b.WriteString(strings.TrimRight(l.String(), " ") + "\n")
	}
	line(headers)
	for _, r := range rows {
		line(r)
	}
	return b.String()
}

type generateConfig struct {
	Out     string   `flag:"out" desc:"Directory of the files"`
	Org     string   `flag:"org" desc:"Organization of the certificates' subjects"`
	Names   []string `flag:"server-name" desc:"Host names and IP addresses of the server certificate"`
	Devices int      `flag:"devices" desc:"Number of device certificates"`
	HMN     bool     `flag:"hmn" desc:"Give the device certificates a HardwareModuleName (with their name as serial number), as IEEE 2030.5 device certificates have"`
	Force   bool     `flag:"force" desc:"Overwrite existing files"`
	Format  string   `flag:"format" env:"IEEE20305CTL_FORMAT" desc:"Output format: text, json or table"`
}

func newGenerateCmd() *cobra.Command {
	c := generateConfig{Out: ".", Org: "IEEE 2030.5 Test", Names: []string{"localhost", "127.0.0.1"}, Devices: 1, Format: "text"}
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a test PKI: a CA, a server certificate and device certificates",
		Long: `Generate a test PKI for IEEE 2030.5 mutual TLS: a CA, a server certificate, and device
certificates, all ECDSA P-256, as PEM files in --out:

  ca.pem, ca.key                the CA
  server.pem, server.key        the server, for --server-name
  device-N.pem, device-N.key    the devices (client authentication)

Keys are PKCS #8 and readable by their owner only. Existing files are not overwritten
unless --force. For development and testing only.`,
		Example: `  ieee20305ctl cert generate --out pki --devices 2 --server-name localhost,127.0.0.1`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.RunFormattedChecked(cmd, c.Format, func() error {
				if c.Devices < 1 {
					return errs.InvalidInput(errs.CodeInvalidInput, "--devices must be at least 1", nil)
				}
				if len(c.Names) == 0 {
					return errs.InvalidInput(errs.CodeInvalidInput, "--server-name must name the server at least once", nil)
				}
				return nil
			}, func(context.Context) (any, error) {
				files, err := generate(c)
				if err != nil {
					return nil, err
				}
				return &Generated{Dir: c.Out, Files: files}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func generate(c generateConfig) ([]GeneratedFile, error) {
	files := []GeneratedFile{{Role: "ca", Name: "ca"}, {Role: "server", Name: "server"}}
	for i := 1; i <= c.Devices; i++ {
		files = append(files, GeneratedFile{Role: "device", Name: fmt.Sprintf("device-%d", i)})
	}
	for i := range files {
		files[i].Cert = filepath.Join(c.Out, files[i].Name+".pem")
		files[i].Key = filepath.Join(c.Out, files[i].Name+".key")
		if c.Force {
			continue
		}
		for _, p := range []string{files[i].Cert, files[i].Key} {
			if _, err := os.Stat(p); err == nil {
				return nil, errs.InvalidInput(errs.CodeInvalidInput, p+" exists; use --force to overwrite", nil)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, errs.Output(errs.CodeOutputFileCreateFailed, err)
			}
		}
	}
	if err := os.MkdirAll(c.Out, 0o755); err != nil {
		return nil, errs.Output(errs.CodeOutputFileCreateFailed, err)
	}
	ca, caKey, err := security.TestCA(c.Org)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if err := write(&files[0], ca, caKey); err != nil {
		return nil, err
	}
	srv, srvKey, err := security.TestServerCert(ca, caKey, c.Names)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if err := write(&files[1], srv, srvKey); err != nil {
		return nil, err
	}
	for i := 2; i < len(files); i++ {
		var dev *x509.Certificate
		var devKey *ecdsa.PrivateKey
		if c.HMN {
			dev, devKey, err = security.TestDeviceCertHMN(ca, caKey, security.HMN{Type: testHMNType, SerialNumber: []byte(files[i].Name)})
		} else {
			dev, devKey, err = security.TestDeviceCert(ca, caKey, files[i].Name)
		}
		if err != nil {
			return nil, errs.Internal(err)
		}
		if err := write(&files[i], dev, devKey); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// testHMNType is the hardware module type of the test certificates: an
// object identifier under the documentation arc.
var testHMNType = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 99999, 2030, 5}

func write(f *GeneratedFile, c *x509.Certificate, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return errs.Internal(err)
	}
	if err := os.WriteFile(f.Cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw}), 0o644); err != nil {
		return errs.Output(errs.CodeOutputFileWriteFailed, err)
	}
	if err := os.WriteFile(f.Key, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return errs.Output(errs.CodeOutputFileWriteFailed, err)
	}
	// WriteFile keeps the mode of an existing file; a key must not stay readable.
	if err := os.Chmod(f.Key, 0o600); err != nil {
		return errs.Output(errs.CodeOutputFileWriteFailed, err)
	}
	f.Identity = identity(c)
	return nil
}

// Info is what cert inspect reports about a certificate.
type Info struct {
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	Serial       string    `json:"serial"`
	NotBefore    time.Time `json:"not_before"`
	NotAfter     time.Time `json:"not_after"`
	Expired      bool      `json:"expired"`
	PublicKey    string    `json:"public_key"`
	IsCA         bool      `json:"is_ca"`
	DNSNames     []string  `json:"dns_names,omitempty"`
	IPAddresses  []string  `json:"ip_addresses,omitempty"`
	ExtKeyUsages []string  `json:"ext_key_usages,omitempty"`
	HMN          *HMNInfo  `json:"hardware_module_name,omitempty"`
	Identity
}

// HMNInfo is the HardwareModuleName of a certificate.
type HMNInfo struct {
	Type   string `json:"type"`
	Serial string `json:"serial_number"`
}

func (info *Info) MarshalTextOutput() (string, error) {
	var b strings.Builder
	row := func(k, v string) { fmt.Fprintf(&b, "%-20s %s\n", k+":", v) }
	row("Subject", info.Subject)
	row("Issuer", info.Issuer)
	row("Serial", info.Serial)
	validity := info.NotBefore.Format(time.RFC3339) + " to " + info.NotAfter.Format(time.RFC3339)
	if info.Expired {
		validity += " (not valid now)"
	}
	row("Valid", validity)
	row("Public key", info.PublicKey)
	row("CA", strconv.FormatBool(info.IsCA))
	if len(info.DNSNames) > 0 {
		row("DNS names", strings.Join(info.DNSNames, ", "))
	}
	if len(info.IPAddresses) > 0 {
		row("IP addresses", strings.Join(info.IPAddresses, ", "))
	}
	if len(info.ExtKeyUsages) > 0 {
		row("Key usages", strings.Join(info.ExtKeyUsages, ", "))
	}
	if info.HMN != nil {
		row("Hardware module", info.HMN.Type+" serial "+info.HMN.Serial)
	}
	row("LFDI", info.LFDI)
	row("SFDI", info.SFDI)
	return b.String(), nil
}

func (info *Info) TableHeaders() []string {
	return []string{"SUBJECT", "ISSUER", "NOT_AFTER", "CA", "LFDI", "SFDI"}
}
func (info *Info) TableRows() [][]string {
	return [][]string{{info.Subject, info.Issuer, info.NotAfter.Format(time.RFC3339), strconv.FormatBool(info.IsCA), info.LFDI, info.SFDI}}
}

type fileConfig struct {
	Format string `flag:"format" env:"IEEE20305CTL_FORMAT" desc:"Output format: text, json or table"`
}

func newInspectCmd() *cobra.Command {
	c := fileConfig{Format: "text"}
	cmd := &cobra.Command{
		Use:   "inspect <certificate>",
		Short: "Show a certificate with its IEEE 2030.5 identity",
		Long: `Show a certificate, PEM or DER: subject, issuer, validity, public key, names, key usages,
its HardwareModuleName if it has one, and its LFDI and SFDI.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.RunFormattedChecked(cmd, c.Format, func() error { return nil }, func(context.Context) (any, error) {
				cert, err := load(args[0])
				if err != nil {
					return nil, err
				}
				return inspect(cert, time.Now()), nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func inspect(c *x509.Certificate, now time.Time) *Info {
	info := &Info{
		Subject: c.Subject.String(), Issuer: c.Issuer.String(), Serial: fmt.Sprintf("%X", c.SerialNumber),
		NotBefore: c.NotBefore.UTC(), NotAfter: c.NotAfter.UTC(), Expired: now.After(c.NotAfter) || now.Before(c.NotBefore),
		PublicKey: publicKey(c), IsCA: c.IsCA, DNSNames: c.DNSNames, Identity: identity(c),
	}
	for _, ip := range c.IPAddresses {
		info.IPAddresses = append(info.IPAddresses, ip.String())
	}
	for _, u := range c.ExtKeyUsage {
		info.ExtKeyUsages = append(info.ExtKeyUsages, extKeyUsage(u))
	}
	if h, ok := security.HardwareModuleName(c); ok {
		info.HMN = &HMNInfo{Type: h.Type.String(), Serial: string(h.SerialNumber)}
	}
	return info
}

func publicKey(c *x509.Certificate) string {
	if k, ok := c.PublicKey.(*ecdsa.PublicKey); ok {
		return "ECDSA " + k.Curve.Params().Name
	}
	return c.PublicKeyAlgorithm.String()
}

var extKeyUsages = map[x509.ExtKeyUsage]string{
	x509.ExtKeyUsageAny: "any", x509.ExtKeyUsageServerAuth: "server auth", x509.ExtKeyUsageClientAuth: "client auth",
	x509.ExtKeyUsageCodeSigning: "code signing", x509.ExtKeyUsageEmailProtection: "email protection",
	x509.ExtKeyUsageTimeStamping: "time stamping", x509.ExtKeyUsageOCSPSigning: "OCSP signing",
}

func extKeyUsage(u x509.ExtKeyUsage) string {
	if name, ok := extKeyUsages[u]; ok {
		return name
	}
	return fmt.Sprintf("usage %d", u)
}

func newLFDICmd() *cobra.Command {
	c := fileConfig{Format: "text"}
	cmd := &cobra.Command{
		Use:   "lfdi <certificate>",
		Short: "Derive the LFDI and SFDI of a certificate",
		Long: `Compute the IEEE 2030.5 device identifiers of a certificate, PEM or DER. The LFDI
(Long-Form Device Identifier) is the leftmost 160 bits of the SHA-256 fingerprint of the
certificate, as 40 hexadecimal characters. The SFDI (Short-Form Device Identifier) is the
leftmost 36 bits of the same fingerprint as a decimal number, followed by a check digit.`,
		Example: `  ieee20305ctl cert lfdi device-1.pem`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.RunFormattedChecked(cmd, c.Format, func() error { return nil }, func(context.Context) (any, error) {
				cert, err := load(args[0])
				if err != nil {
					return nil, err
				}
				id := identity(cert)
				return &id, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type verifyConfig struct {
	CA     string `flag:"ca" desc:"CA certificates (PEM) to verify against"`
	Usage  string `flag:"usage" desc:"Extended key usage to require: client (a device), server, or any"`
	Name   string `flag:"name" desc:"Host name or IP address the certificate must be for (a server's)"`
	Format string `flag:"format" env:"IEEE20305CTL_FORMAT" desc:"Output format: text, json or table"`
}

// Verified is the result of cert verify.
type Verified struct {
	Certificate string `json:"certificate"`
	Valid       bool   `json:"valid"`
	Identity
}

func (v *Verified) MarshalTextOutput() (string, error) {
	return fmt.Sprintf("%s: verified\n  LFDI %s\n  SFDI %s", v.Certificate, v.LFDI, v.SFDI), nil
}

func newVerifyCmd() *cobra.Command {
	c := verifyConfig{Usage: "client", Format: "text"}
	cmd := &cobra.Command{
		Use:   "verify <certificate>",
		Short: "Verify a certificate as an IEEE 2030.5 peer does",
		Long: `Verify a certificate chain (the certificate, then any intermediates, in one PEM file)
against --ca as go-ieee20305 does: the critical HardwareModuleName subjectAltName of
IEEE 2030.5 device certificates is accepted, and the chain, validity, key usage and,
with --name, the host name are checked. A certificate that does not verify exits with 3.`,
		Example: `  ieee20305ctl cert verify device-1.pem --ca ca.pem
  ieee20305ctl cert verify server.pem --ca ca.pem --usage server --name localhost`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var usage x509.ExtKeyUsage
			return runner.RunFormattedChecked(cmd, c.Format, func() error {
				if c.CA == "" {
					return errs.InvalidInput(errs.CodeInvalidInput, "--ca is required", nil)
				}
				switch c.Usage {
				case "client":
					usage = x509.ExtKeyUsageClientAuth
				case "server":
					usage = x509.ExtKeyUsageServerAuth
				case "any":
					usage = x509.ExtKeyUsageAny
				default:
					return errs.InvalidInput(errs.CodeInvalidInput, "--usage: client, server or any", nil)
				}
				return nil
			}, func(context.Context) (any, error) {
				chain, err := loadChain(args[0])
				if err != nil {
					return nil, err
				}
				roots, err := conn.LoadPool(c.CA)
				if err != nil {
					return nil, err
				}
				if err := security.Verify(chain, security.VerifyOptions{Roots: roots, Usage: usage, ServerName: c.Name}); err != nil {
					return nil, errs.InvalidInput(errs.CodeInvalidCertificate, fmt.Sprintf("%s does not verify: %v", args[0], err), err)
				}
				return &Verified{Certificate: args[0], Valid: true, Identity: identity(chain[0])}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

// load reads the first certificate of a PEM file, or a DER file.
func load(path string) (*x509.Certificate, error) {
	chain, err := loadChain(path)
	if err != nil {
		return nil, err
	}
	return chain[0], nil
}

func loadChain(path string) ([]*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errs.InvalidInput(errs.CodeInputFileOpenFailed, err.Error(), err)
	}
	var chain []*x509.Certificate
	for rest := data; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, errs.InvalidInput(errs.CodeInvalidCertificate, fmt.Sprintf("%s: %v", path, err), err)
		}
		chain = append(chain, c)
	}
	if len(chain) == 0 {
		c, err := x509.ParseCertificate(data)
		if err != nil {
			return nil, errs.InvalidInput(errs.CodeInvalidCertificate, fmt.Sprintf("%s: no PEM CERTIFICATE block and not DER: %v", path, err), err)
		}
		chain = append(chain, c)
	}
	return chain, nil
}
