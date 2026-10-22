// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/otfabric/ieee20305ctl/cmd/cert"
	"github.com/otfabric/ieee20305ctl/cmd/client"
	"github.com/otfabric/ieee20305ctl/cmd/server"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/spf13/cobra"
)

// Build metadata, set via SetBuildMeta.
var (
	buildVersion   = "dev"
	buildTag       = "none"
	buildCommit    = "unknown"
	buildBuildDate = "unknown"
)

// SetBuildMeta stores link-time build metadata for use by the version command.
func SetBuildMeta(version, tag, commit, buildDate string) {
	buildVersion = version
	buildTag = tag
	buildCommit = commit
	buildBuildDate = buildDate
}

// Execute runs the root command and returns an exit code.
func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "ieee20305ctl:", err)
		var coded *errs.ExitCodeError
		if errors.As(err, &coded) {
			return coded.Code
		}
		return errs.ExitError
	}
	return errs.ExitOK
}

// NewRootCmd builds and returns the top-level cobra command tree.
func NewRootCmd() *cobra.Command {
	var debug bool

	root := &cobra.Command{
		Use:   "ieee20305ctl",
		Short: "IEEE 2030.5 command-line client and server tool",
		Long: `ieee20305ctl is a CLI companion for the go-ieee20305 library.
It provides client and server commands for IEEE 2030.5 (SEP2) smart-grid
communication, along with certificate management and XML validation utilities.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")

	// Wire up a signal-aware context for all subcommands.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cmd.SetContext(ctx)
		if debug {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "debug mode enabled")
		}
		return nil
	}

	// Ensure the cancel func is called when the root finishes.
	root.PersistentPostRun = func(_ *cobra.Command, _ []string) {
		cancel()
	}

	// Subcommands.
	root.AddCommand(client.NewClientCmd())
	root.AddCommand(server.NewServerCmd())
	root.AddCommand(cert.NewCertCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newCompletionCmd())

	return root
}

// newVersionCmd returns the version subcommand.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  `Print version, tag, commit, build date, and Go runtime information.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ieee20305ctl %s\n", buildVersion)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  tag:       %s\n", buildTag)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  commit:    %s\n", buildCommit)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  built:     %s\n", buildBuildDate)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  go:        %s\n", runtime.Version())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  os/arch:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}

// newCompletionCmd returns the completion subcommand for shell autocompletion.
func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for ieee20305ctl.

To load completions:

  bash:       source <(ieee20305ctl completion bash)
  zsh:        ieee20305ctl completion zsh > "${fpath[1]}/_ieee20305ctl"
  fish:       ieee20305ctl completion fish | source
  powershell: ieee20305ctl completion powershell | Out-String | Invoke-Expression`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
}
