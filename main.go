// SPDX-License-Identifier: MIT

package main

import (
	"os"

	"github.com/otfabric/ieee20305ctl/cmd"
)

// Build-time variables injected via ldflags (-X main.version=… and so on); empty when not
// injected, and then filled in from the build information Go embeds (internal/buildinfo).
var (
	version   string
	tag       string
	commit    string
	buildDate string
)

// exit ends the process; a test replaces it.
var exit = os.Exit

func main() {
	cmd.SetBuildMeta(version, tag, commit, buildDate)
	exit(cmd.Execute())
}
