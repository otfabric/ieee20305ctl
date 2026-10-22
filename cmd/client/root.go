// SPDX-License-Identifier: MIT

package client

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewClientCmd returns the client subcommand group for IEEE 2030.5 client operations.
func NewClientCmd() *cobra.Command {
	var (
		host     string
		certFile string
		keyFile  string
		caFile   string
	)

	cmd := &cobra.Command{
		Use:   "client",
		Short: "IEEE 2030.5 client commands",
		Long:  `Commands for interacting with an IEEE 2030.5 server as a client device.`,
	}

	cmd.PersistentFlags().StringVar(&host, "host", "", "server host:port")
	cmd.PersistentFlags().StringVar(&certFile, "cert", "", "path to client TLS certificate")
	cmd.PersistentFlags().StringVar(&keyFile, "key", "", "path to client TLS private key")
	cmd.PersistentFlags().StringVar(&caFile, "ca", "", "path to CA certificate for server verification")

	cmd.AddCommand(newDiscoverCmd())
	cmd.AddCommand(newRegisterCmd())
	cmd.AddCommand(newProgramsCmd())
	cmd.AddCommand(newControlsCmd())
	cmd.AddCommand(newReportCmd())

	return cmd
}

// newDiscoverCmd returns the discover subcommand.
func newDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Discover server capabilities and resources",
		Long:  `Connect to an IEEE 2030.5 server and discover its DeviceCapability resource and available function sets.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("client discover: not implemented")
		},
	}
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
