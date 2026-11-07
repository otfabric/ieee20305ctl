// SPDX-License-Identifier: MIT

package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// complete runs the hidden __complete command and returns the candidates (descriptions
// cut off) and the directive line.
func complete(t *testing.T, args ...string) (values []string, directive string) {
	t.Helper()
	r := must(t, 0, append([]string{"__complete"}, args...)...)
	for line := range strings.SplitSeq(strings.TrimSpace(r.stdout), "\n") {
		if strings.HasPrefix(line, ":") {
			directive = line
			continue
		}
		values = append(values, strings.SplitN(line, "\t", 2)[0])
	}
	return values, directive
}

func TestCompletionShells(t *testing.T) {
	got, dir := complete(t, "completion", "")
	if strings.Join(got, " ") != "bash zsh fish powershell" || dir != ":4" {
		t.Fatalf("completion shells: %q %s", got, dir)
	}
	for _, sh := range completionShells {
		if r := must(t, 0, "completion", sh); len(r.stdout) == 0 {
			t.Errorf("completion %s wrote nothing", sh)
		}
	}
}

func TestEnumFlagCompletion(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"client", "dcap", "--format", ""}, "json table text"},
		{[]string{"client", "dcap", "--profile", ""}, "ieee20305 csipaus"},
		{[]string{"client", "dcap", "--edition", ""}, "2023 2018"},
		{[]string{"client", "respond", "/x", "--status", "opt"}, "opt-out opt-in"},
		{[]string{"server", "start", "--profile", ""}, "ieee20305 csipaus"},
		{[]string{"server", "provision", "--role", ""}, "device aggregator operator"},
		{[]string{"server", "devices", "--format", ""}, "json table text"},
		{[]string{"cert", "verify", "x.pem", "--usage", ""}, "client server any"},
		{[]string{"validate", "--profile", ""}, "ieee20305 csipaus"},
	} {
		got, dir := complete(t, tc.args...)
		if strings.Join(got, " ") != tc.want || dir != ":4" {
			t.Errorf("%v: %q %s, want %q :4", tc.args, got, dir, tc.want)
		}
	}
}

func TestFileAndFreeValueCompletion(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"server", "start", "--cert", ""}, "pem crt cer :8"},
		{[]string{"server", "start", "--config", ""}, "yaml yml json :8"},
		{[]string{"client", "der", "put", "--status", ""}, "xml :8"},
		{[]string{"cert", "inspect", ""}, "pem crt cer der :8"},
		{[]string{"validate", ""}, "xml :8"},
		{[]string{"cert", "generate", "--out", ""}, ":16"},
		{[]string{"client", "dcap", "--url", ""}, ":4"},
		{[]string{"client", "get", ""}, ":4"},
	} {
		got, dir := complete(t, tc.args...)
		if s := strings.TrimSpace(strings.Join(got, " ") + " " + dir); s != tc.want {
			t.Errorf("%v: %q, want %q", tc.args, s, tc.want)
		}
	}
}

// valueSet matches a usage that lists the values of a flag: "text, json or table",
// "Profile: ieee20305 or csipaus", "one of …", "a|b".
var valueSet = regexp.MustCompile(`(^|: )[a-z0-9-]+( \([^)]*\))?(, [a-z0-9-]+( \([^)]*\))?)*,? or [a-z0-9-]+|\bone of\b|\b[a-z0-9-]+\|[a-z0-9-]+\b`)

// Every flag that takes a value completes, and one whose usage lists its values
// completes to them, not to file names.
func TestEveryFlagCompletes(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.NoOptDefVal != "" || f.Name == "help" {
				return
			}
			fn, ok := c.GetFlagCompletionFunc(f.Name)
			if !ok {
				t.Errorf("%s --%s has no completion", c.CommandPath(), f.Name)
				return
			}
			if valueSet.MatchString(f.Usage) {
				values, dir := fn(c, nil, "")
				if len(values) < 2 || dir != cobra.ShellCompDirectiveNoFileComp {
					t.Errorf("%s --%s (%q) completes %q, directive %d", c.CommandPath(), f.Name, f.Usage, values, dir)
				}
			}
		})
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(NewRootCmd())
}
