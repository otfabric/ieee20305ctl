// SPDX-License-Identifier: MIT

package cert

import (
	"fmt"

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

// newGenerateCmd returns the generate subcommand.
func newGenerateCmd() *cobra.Command {
	var (
		outDir string
		org    string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate test certificates",
		Long: `Generate a set of test certificates for IEEE 2030.5 communication.
Creates a CA certificate, server certificate, and one or more device certificates
suitable for development and testing.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("cert generate: not implemented")
		},
	}

	cmd.Flags().StringVar(&outDir, "out", ".", "output directory for generated certificates")
	cmd.Flags().StringVar(&org, "org", "IEEE 2030.5 Test", "organization name for certificate subject")

	return cmd
}

// newInspectCmd returns the inspect subcommand.
func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect [certificate]",
		Short: "Inspect a certificate",
		Long:  `Display details of an IEEE 2030.5 certificate including LFDI, SFDI, subject, issuer, and validity period.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("cert inspect: not implemented")
		},
	}
}

// newLFDICmd returns the lfdi subcommand.
func newLFDICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lfdi [certificate]",
		Short: "Derive LFDI from a certificate",
		Long: `Compute the Long-Form Device Identifier (LFDI) from a certificate.
The LFDI is the SHA-256 fingerprint of the certificate, represented as
a 40-character hex string (truncated to 160 bits per IEEE 2030.5).`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("cert lfdi: not implemented")
		},
	}
}
