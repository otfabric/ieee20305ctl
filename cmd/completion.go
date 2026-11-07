// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/cli"
)

// completionShells are the shells "completion" writes a script for.
var completionShells = []string{"bash", "zsh", "fish", "powershell"}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate the shell completion script of ieee20305ctl for bash, zsh, fish or
PowerShell; README.md, "Shell completion", says where to put it. Commands, flags and the
values of the flags with a fixed set of them (--format, --profile, --edition, --role,
--usage, --status) complete; the flags that take a file complete files of its type.`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: completionShells,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(out, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			default:
				return cmd.Root().GenPowerShellCompletionWithDesc(out)
			}
		},
	}
}

// enumFlags are the flags with a fixed set of values, by name, on every command that has
// them; enumFlagsOf those of one command (its path without the root name), where a flag
// of that name means something else elsewhere.
var (
	enumFlags = map[string][]string{
		"profile": {"ieee20305", "csipaus"},
		"edition": {"2023", "2018"},
		"role":    {"device", "aggregator", "operator"},
		"usage":   {"client", "server", "any"},
	}
	enumFlagsOf = map[string]map[string][]string{
		"client respond": {"status": {"received", "started", "completed", "opt-out", "opt-in", "cancelled", "superseded"}},
	}
)

// Files of the flags and arguments that take one, by extension.
var (
	pemFiles  = []string{"pem", "crt", "cer"}
	keyFiles  = []string{"key", "pem"}
	certFiles = []string{"pem", "crt", "cer", "der"}
	xmlFiles  = []string{"xml"}
	seedFiles = []string{"yaml", "yml", "json"}
)

// registerCompletions registers the completion of every flag and argument of the tree
// (--format registers its own, with descriptions, when its command is built).
func registerCompletions(root *cobra.Command) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		path := strings.TrimSpace(strings.TrimPrefix(c.CommandPath(), root.Name()))
		for name, values := range enumFlagsOf[path] {
			_ = cli.RegisterEnumFlagCompletion(c, name, values)
		}
		for name, values := range enumFlags {
			if c.LocalNonPersistentFlags().Lookup(name) != nil {
				_ = cli.RegisterEnumFlagCompletion(c, name, values)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	cli.CompleteTree(root, cli.Completion{
		Files: map[string][]string{
			"cert": pemFiles, "key": keyFiles, "ca": pemFiles,
			"listen-cert": pemFiles, "listen-key": keyFiles,
			"config": seedFiles, "output": seedFiles,
			"file": xmlFiles, "capability": xmlFiles, "settings": xmlFiles,
			"status": xmlFiles, "availability": xmlFiles,
			"control-socket": {},
		},
		Dirs: []string{"out"},
		Args: map[string][]string{
			"validate":     xmlFiles,
			"cert inspect": certFiles,
			"cert lfdi":    certFiles,
			"cert verify":  certFiles,
		},
	})
}
