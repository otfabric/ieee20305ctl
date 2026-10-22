// SPDX-License-Identifier: MIT

// Package cert implements the cert subcommands: test PKI generation and
// certificate inspection with IEEE 2030.5 device identifiers.
package cert

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/otfabric/go-ieee20305/security"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/spf13/cobra"
)

// NewCertCmd returns the cert subcommand group for certificate management.
func NewCertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Certificate management commands",
		Long:  `Commands for generating, inspecting, and managing IEEE 2030.5 device and server certificates.`,
	}

	cmd.AddCommand(newGenerateCmd())
	cmd.AddCommand(newInspectCmd())
	cmd.AddCommand(newLFDICmd())

	return cmd
}

// Identity is the IEEE 2030.5 identity of a certificate.
type Identity struct {
	LFDI string `json:"lfdi"`
	SFDI uint64 `json:"sfdi"`
}

func identity(c *x509.Certificate) Identity {
	lfdi := security.CertificateLFDI(c)
	return Identity{LFDI: lfdi.String(), SFDI: uint64(lfdi.SFDI())}
}

// GeneratedFile is a certificate written by cert generate.
type GeneratedFile struct {
	Role    string `json:"role"`
	Name    string `json:"name"`
	CertPEM string `json:"cert"`
	KeyPEM  string `json:"key"`
	Identity
}

func newGenerateCmd() *cobra.Command {
	var (
		outDir  string
		org     string
		names   []string
		devices int
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate test certificates",
		Long: `Generate a test PKI for IEEE 2030.5 mutual TLS: a CA, a server certificate,
and one or more device certificates, all ECDSA P-256, with PEM files written
to the output directory:

  ca.pem, ca.key            the CA
  server.pem, server.key    the server, with --server-name as its names
  device-N.pem, device-N.key  the devices (client authentication)

Private keys are PKCS #8 and readable by the owner only. Existing files are
not overwritten unless --force is given. The LFDI and SFDI of every
certificate are printed. These certificates are for development and testing.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := format.Parse(cmd.Flag("output").Value.String())
			if err != nil {
				return errs.WithCode(errs.ExitUsage, err)
			}
			if devices < 1 {
				return errs.WithCode(errs.ExitUsage, errors.New("--devices must be at least 1"))
			}
			if len(names) == 0 {
				return errs.WithCode(errs.ExitUsage, errors.New("--server-name must name the server at least once"))
			}
			files, err := generate(outDir, org, names, devices, force)
			return result(cmd.OutOrStdout(), f, Generated{Dir: outDir, Files: files}, err, func(w io.Writer) error {
				return printGenerated(w, outDir, files)
			})
		},
	}

	cmd.Flags().StringVar(&outDir, "out", ".", "output directory for generated certificates")
	cmd.Flags().StringVar(&org, "org", "IEEE 2030.5 Test", "organization name for certificate subjects")
	cmd.Flags().StringSliceVar(&names, "server-name", []string{"localhost", "127.0.0.1"}, "host names and IP addresses of the server certificate")
	cmd.Flags().IntVar(&devices, "devices", 1, "number of device certificates")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	format.AddFlag(cmd)

	return cmd
}

func generate(dir, org string, names []string, devices int, force bool) ([]GeneratedFile, error) {
	files := []GeneratedFile{{Role: "ca", Name: "ca"}, {Role: "server", Name: "server"}}
	for i := 1; i <= devices; i++ {
		files = append(files, GeneratedFile{Role: "device", Name: fmt.Sprintf("device-%d", i)})
	}
	for i := range files {
		files[i].CertPEM = filepath.Join(dir, files[i].Name+".pem")
		files[i].KeyPEM = filepath.Join(dir, files[i].Name+".key")
		if force {
			continue
		}
		for _, p := range []string{files[i].CertPEM, files[i].KeyPEM} {
			if _, err := os.Stat(p); err == nil {
				return nil, errs.WithCode(errs.ExitUsage, fmt.Errorf("%s exists; use --force to overwrite", p))
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	ca, caKey, err := security.TestCA(org)
	if err != nil {
		return nil, errs.WithCode(errs.ExitCertificate, fmt.Errorf("generating the CA: %w", err))
	}
	if err := write(&files[0], ca, caKey); err != nil {
		return nil, err
	}
	srv, srvKey, err := security.TestServerCert(ca, caKey, names)
	if err != nil {
		return nil, errs.WithCode(errs.ExitCertificate, fmt.Errorf("generating the server certificate: %w", err))
	}
	if err := write(&files[1], srv, srvKey); err != nil {
		return nil, err
	}
	for i := 2; i < len(files); i++ {
		dev, devKey, err := security.TestDeviceCert(ca, caKey, files[i].Name)
		if err != nil {
			return nil, errs.WithCode(errs.ExitCertificate, fmt.Errorf("generating %s: %w", files[i].Name, err))
		}
		if err := write(&files[i], dev, devKey); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func write(f *GeneratedFile, c *x509.Certificate, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(f.CertPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw}), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(f.KeyPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return err
	}
	// WriteFile keeps the mode of an existing file; a key must not stay readable.
	if err := os.Chmod(f.KeyPEM, 0o600); err != nil {
		return err
	}
	f.Identity = identity(c)
	return nil
}

// Generated is the result of cert generate.
type Generated struct {
	Dir   string          `json:"dir"`
	Files []GeneratedFile `json:"files"`
}

// result writes data in format f: as a JSON envelope, or as text written
// by text. In JSON a failure is written as an error envelope as well.
func result(w io.Writer, f format.Format, data any, err error, text func(io.Writer) error) error {
	if f == format.JSON {
		if err != nil {
			_ = format.WriteJSONError(w, err)
			return err
		}
		return format.WriteJSON(w, data)
	}
	if err != nil {
		return err
	}
	return text(w)
}

func printGenerated(w io.Writer, dir string, files []GeneratedFile) error {
	if _, err := fmt.Fprintf(w, "Wrote to %s:\n\n", dir); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ROLE\tCERTIFICATE\tKEY\tLFDI\tSFDI")
	for _, g := range files {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n", g.Role, filepath.Base(g.CertPEM), filepath.Base(g.KeyPEM), g.LFDI, g.SFDI)
	}
	return tw.Flush()
}

// Info is what cert inspect reports about a certificate.
type Info struct {
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	Serial       string    `json:"serial"`
	NotBefore    time.Time `json:"notBefore"`
	NotAfter     time.Time `json:"notAfter"`
	Expired      bool      `json:"expired"`
	PublicKey    string    `json:"publicKey"`
	IsCA         bool      `json:"isCA"`
	DNSNames     []string  `json:"dnsNames,omitempty"`
	IPAddresses  []string  `json:"ipAddresses,omitempty"`
	ExtKeyUsages []string  `json:"extKeyUsages,omitempty"`
	Identity
}

func newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect [certificate]",
		Short: "Inspect a certificate",
		Long: `Display details of a certificate, PEM or DER: subject, issuer, validity,
public key, names, key usages, and its IEEE 2030.5 LFDI and SFDI.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := format.Parse(cmd.Flag("output").Value.String())
			if err != nil {
				return errs.WithCode(errs.ExitUsage, err)
			}
			c, err := LoadCertificate(args[0])
			var info Info
			if err == nil {
				info = inspect(c, time.Now())
			}
			return result(cmd.OutOrStdout(), f, info, err, func(w io.Writer) error {
				return printInfo(w, info)
			})
		},
	}
	format.AddFlag(cmd)
	return cmd
}

func inspect(c *x509.Certificate, now time.Time) Info {
	info := Info{
		Subject:   c.Subject.String(),
		Issuer:    c.Issuer.String(),
		Serial:    fmt.Sprintf("%X", c.SerialNumber),
		NotBefore: c.NotBefore.UTC(),
		NotAfter:  c.NotAfter.UTC(),
		Expired:   now.After(c.NotAfter) || now.Before(c.NotBefore),
		PublicKey: publicKey(c),
		IsCA:      c.IsCA,
		DNSNames:  c.DNSNames,
		Identity:  identity(c),
	}
	for _, ip := range c.IPAddresses {
		info.IPAddresses = append(info.IPAddresses, ip.String())
	}
	for _, u := range c.ExtKeyUsage {
		info.ExtKeyUsages = append(info.ExtKeyUsages, extKeyUsage(u))
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
	x509.ExtKeyUsageAny:             "any",
	x509.ExtKeyUsageServerAuth:      "server auth",
	x509.ExtKeyUsageClientAuth:      "client auth",
	x509.ExtKeyUsageCodeSigning:     "code signing",
	x509.ExtKeyUsageEmailProtection: "email protection",
	x509.ExtKeyUsageTimeStamping:    "time stamping",
	x509.ExtKeyUsageOCSPSigning:     "OCSP signing",
}

func extKeyUsage(u x509.ExtKeyUsage) string {
	if name, ok := extKeyUsages[u]; ok {
		return name
	}
	return fmt.Sprintf("usage %d", u)
}

func printInfo(w io.Writer, info Info) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	row := func(k, v string) { _, _ = fmt.Fprintf(tw, "%s:\t%s\n", k, v) }
	row("Subject", info.Subject)
	row("Issuer", info.Issuer)
	row("Serial", info.Serial)
	validity := info.NotBefore.Format(time.RFC3339) + " to " + info.NotAfter.Format(time.RFC3339)
	if info.Expired {
		validity += " (not valid now)"
	}
	row("Valid", validity)
	row("Public key", info.PublicKey)
	row("CA", fmt.Sprint(info.IsCA))
	if len(info.DNSNames) > 0 {
		row("DNS names", strings.Join(info.DNSNames, ", "))
	}
	if len(info.IPAddresses) > 0 {
		row("IP addresses", strings.Join(info.IPAddresses, ", "))
	}
	if len(info.ExtKeyUsages) > 0 {
		row("Key usages", strings.Join(info.ExtKeyUsages, ", "))
	}
	row("LFDI", info.LFDI)
	row("SFDI", fmt.Sprint(info.SFDI))
	return tw.Flush()
}

func newLFDICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lfdi [certificate]",
		Short: "Derive the LFDI and SFDI of a certificate",
		Long: `Compute the IEEE 2030.5 device identifiers of a certificate, PEM or DER.
The LFDI (Long-Form Device Identifier) is the leftmost 160 bits of the
SHA-256 fingerprint of the certificate, as 40 hexadecimal characters. The
SFDI (Short-Form Device Identifier) is the leftmost 36 bits of the same
fingerprint as a decimal number, followed by a check digit.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := format.Parse(cmd.Flag("output").Value.String())
			if err != nil {
				return errs.WithCode(errs.ExitUsage, err)
			}
			c, err := LoadCertificate(args[0])
			var id Identity
			if err == nil {
				id = identity(c)
			}
			return result(cmd.OutOrStdout(), f, id, err, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "LFDI: %s\nSFDI: %d\n", id.LFDI, id.SFDI)
				return err
			})
		},
	}
	format.AddFlag(cmd)
	return cmd
}

// LoadCertificate reads the first certificate of a PEM file, or a DER file.
func LoadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errs.WithCode(errs.ExitCertificate, err)
	}
	der := data
	for rest := data; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			der = block.Bytes
			break
		}
		der = nil
	}
	if der == nil {
		return nil, errs.WithCode(errs.ExitCertificate, fmt.Errorf("%s: no CERTIFICATE block", path))
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, errs.WithCode(errs.ExitCertificate, fmt.Errorf("%s: %w", path, err))
	}
	return c, nil
}
