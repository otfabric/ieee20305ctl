// SPDX-License-Identifier: MIT

package server

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewServerCmd returns the server subcommand for running an IEEE 2030.5 server.
func NewServerCmd() *cobra.Command {
	var (
		addr     string
		certFile string
		keyFile  string
		caFile   string
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start an IEEE 2030.5 server",
		Long: `Start an IEEE 2030.5 (SEP2) HTTPS server with mutual TLS authentication.
The server exposes standard function set resources for client device communication.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("server: not implemented")
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8443", "listen address (host:port)")
	cmd.Flags().StringVar(&certFile, "cert", "", "path to server TLS certificate")
	cmd.Flags().StringVar(&keyFile, "key", "", "path to server TLS private key")
	cmd.Flags().StringVar(&caFile, "ca", "", "path to CA certificate for client verification")

	return cmd
}
