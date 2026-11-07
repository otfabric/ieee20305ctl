// SPDX-License-Identifier: MIT

// Package cmd is the command tree of ieee20305ctl.
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/cmd/cert"
	"github.com/otfabric/ieee20305ctl/cmd/client"
	"github.com/otfabric/ieee20305ctl/cmd/server"
	"github.com/otfabric/ieee20305ctl/internal/buildinfo"
	"github.com/otfabric/ieee20305ctl/internal/config"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/otfabric/ieee20305ctl/internal/runner"
)

// SetBuildMeta is called from main before Execute; values come from ldflags -X main.*.
func SetBuildMeta(version, tag, commit, buildDate string) {
	buildinfo.Set(version, tag, commit, buildDate)
}

// NewRootCmd builds the command tree. Every call returns a fresh tree, with the flags of
// every command at the defaults the environment gives them.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ieee20305ctl",
		Short: "IEEE 2030.5 command-line device and server",
		Long: `ieee20305ctl speaks IEEE 2030.5 (Smart Energy Profile 2.0), the protocol between
utilities, aggregators and distributed energy resources, in editions 2018 and 2023,
with the CSIP and CSIP-AUS profiles, in both roles.

As a device ("ieee20305ctl client") it reads a server's resources, registers, lists
DER programs and controls, answers events, reports metering and DER status, posts log
events, asks for flow reservations, subscribes to notifications and runs a device agent.
As a server ("ieee20305ctl server") it serves a seeded resource tree to devices, and
an operator issues controls to it while it runs. "cert" makes and inspects IEEE 2030.5
certificates, and "validate" checks documents against the schema.

Results go to stdout as text, a table or JSON (--format); progress and diagnostics go to
stderr. Exit codes are stable: see docs/exitcodes.md.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("debug", false, "Show the causes of an error, and the requests and answers of the library, on stderr")
	root.AddCommand(
		client.NewCmd(),
		server.NewCmd(),
		cert.NewCmd(),
		newValidateCmd(),
		newCompletionCmd(),
		newVersionCmd(),
	)
	registerCompletions(root)
	return root
}

func newVersionCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of ieee20305ctl",
		Long: `Print the version of ieee20305ctl on one line: version, tag, commit, build date, Go
version and platform. --json (or IEEE20305CTL_FORMAT=json) prints it as one JSON object.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			info := buildinfo.Get("ieee20305ctl")
			line := info.String()
			if asJSON || strings.EqualFold(strings.TrimSpace(config.Getenv("IEEE20305CTL_FORMAT")), "json") {
				line = info.JSON()
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the version as one JSON object")
	return cmd
}

// Execute runs the root command and returns the process exit code (main should call os.Exit).
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

// Run executes ieee20305ctl with args, reading stdin and writing to stdout and stderr, and
// returns the exit code. It is Execute without the process: what the tests drive.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root, err := buildRoot()
	if err != nil {
		// No command parsed its flags: render the error in the format they ask for.
		inv := &runner.Invocation{}
		if f, ferr := format.Parse(requestedFormat(args)); ferr == nil {
			inv.SetOutputFormat(f)
		}
		return runner.RenderFatal(inv, err, stdout, stderr, slices.Contains(args, "--debug"))
	}
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	inv := &runner.Invocation{}
	ctx = runner.WithInvocation(ctx, inv)
	err = root.ExecuteContext(ctx)
	debug, _ := root.PersistentFlags().GetBool("debug")
	if err != nil {
		return runner.RenderFatal(inv, usageError(err), stdout, stderr, debug)
	}
	if inv.SuccessExitSet {
		return inv.SuccessExitCode
	}
	return errs.ExitOK
}

// buildRoot is NewRootCmd with a malformed environment variable reported as invalid input
// instead of a panic: the configuration of every command is read when the tree is built.
func buildRoot() (root *cobra.Command, err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := strings.TrimPrefix(fmt.Sprint(r), "ieee20305ctl: ")
			err = errs.InvalidInput(errs.CodeInvalidInput, msg, nil)
		}
	}()
	return NewRootCmd(), nil
}

// usageError classifies what Cobra itself rejects (an unknown command or flag, a missing
// required flag, a value a flag cannot hold) as a usage error: exit code 2.
func usageError(err error) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.New(errs.KindUsage, errs.CodeInvalidInput, err.Error(), err)
}

// requestedFormat is the --format of args, or else IEEE20305CTL_FORMAT, for
// an error that ends the run before any command parses its flags.
func requestedFormat(args []string) string {
	for i, a := range args {
		if a == "--" {
			break
		}
		if v, ok := strings.CutPrefix(a, "--format="); ok {
			return v
		}
		if a == "--format" && i+1 < len(args) {
			return args[i+1]
		}
	}
	if v := config.Getenv("IEEE20305CTL_FORMAT"); v != "" {
		return v
	}
	return "text"
}
