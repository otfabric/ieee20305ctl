// SPDX-License-Identifier: MIT

// Package client implements the client subcommands: an IEEE 2030.5 device
// talking to a server over HTTPS with mutual TLS.
package client

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// options are the connection flags shared by the client subcommands.
type options struct {
	host     string
	dcap     string
	certFile string
	keyFile  string
	caFile   string
	timeout  time.Duration
}

// NewClientCmd returns the client subcommand group for IEEE 2030.5 client operations.
func NewClientCmd() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "client",
		Short: "IEEE 2030.5 client commands",
		Long: `Commands for interacting with an IEEE 2030.5 server as a client device.

The device authenticates with its certificate (--cert, --key), and verifies
the server with --ca, or with the system roots when --ca is not given.`,
	}

	f := cmd.PersistentFlags()
	f.StringVar(&opts.host, "host", "", "server as host[:port] (port 443 by default) or https URL")
	f.StringVar(&opts.dcap, "dcap", "/dcap", "path of the DeviceCapability resource")
	f.StringVar(&opts.certFile, "cert", "", "path to the device certificate (PEM)")
	f.StringVar(&opts.keyFile, "key", "", "path to the device private key (PEM)")
	f.StringVar(&opts.caFile, "ca", "", "path to the CA certificates that verify the server (PEM)")
	f.DurationVar(&opts.timeout, "timeout", 30*time.Second, "time limit of the command")

	cmd.AddCommand(newDiscoverCmd(opts))
	cmd.AddCommand(newRegisterCmd())
	cmd.AddCommand(newProgramsCmd())
	cmd.AddCommand(newControlsCmd())
	cmd.AddCommand(newReportCmd())

	return cmd
}

// newRegisterCmd returns the register subcommand.
func newRegisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "register",
		Short: "Register an EndDevice with the server",
		Long:  `Register a new EndDevice resource on the IEEE 2030.5 server using the device certificate identity.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("client register: not implemented")
		},
	}
}

// newProgramsCmd returns the programs subcommand.
func newProgramsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "programs",
		Short: "List DER programs",
		Long:  `List DERProgram resources available from the IEEE 2030.5 server.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("client programs: not implemented")
		},
	}
}

// newControlsCmd returns the controls subcommand.
func newControlsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "controls",
		Short: "List or monitor DER controls",
		Long:  `List or continuously monitor DERControl events from the IEEE 2030.5 server.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("client controls: not implemented")
		},
	}
}

// newReportCmd returns the report subcommand.
func newReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Submit telemetry or metering data",
		Long:  `Submit MirrorUsagePoint or MirrorMeterReading data to the IEEE 2030.5 server.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("client report: not implemented")
		},
	}
}
