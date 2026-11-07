// SPDX-License-Identifier: MIT

package cmd

import (
	"runtime"
	"strings"
	"testing"

	"github.com/otfabric/ieee20305ctl/internal/buildinfo"
)

const testSHA = "1802d0f9afe8642d1e278120b76fcdfac967cb9e"

func TestVersionOutput(t *testing.T) {
	t.Cleanup(func() { buildinfo.Set("", "", "", "") })
	SetBuildMeta("0.5.0", "v0.5.0", testSHA, "2026-11-05T15:41:49Z")
	tail := runtime.Version() + ", " + runtime.GOOS + "/" + runtime.GOARCH
	text := "ieee20305ctl 0.5.0 (tag v0.5.0, commit " + testSHA + ", built 2026-11-05T15:41:49Z, " + tail + ")\n"
	json := `{"name":"ieee20305ctl","version":"0.5.0","tag":"v0.5.0","commit":"` + testSHA +
		`","built":"2026-11-05T15:41:49Z","go":"` + runtime.Version() + `","platform":"` + runtime.GOOS + "/" + runtime.GOARCH + "\"}\n"
	if r := must(t, 0, "version"); r.stdout != text {
		t.Errorf("version:\n got %q\nwant %q", r.stdout, text)
	}
	if r := must(t, 0, "version", "--json"); r.stdout != json {
		t.Errorf("version --json:\n got %q\nwant %q", r.stdout, json)
	}
	t.Setenv("IEEE20305CTL_FORMAT", "json")
	if r := must(t, 0, "version"); r.stdout != json {
		t.Errorf("IEEE20305CTL_FORMAT=json version:\n got %q\nwant %q", r.stdout, json)
	}
	t.Setenv("IEEE20305CTL_FORMAT", "table")
	if r := must(t, 0, "version"); r.stdout != text {
		t.Errorf("IEEE20305CTL_FORMAT=table version:\n got %q\nwant %q", r.stdout, text)
	}
}

func TestVersionOfADevelopmentBuild(t *testing.T) {
	SetBuildMeta("", "", "", "")
	r := must(t, 0, "version")
	if !strings.HasPrefix(r.stdout, "ieee20305ctl ") || strings.Contains(r.stdout, "tag ") ||
		!strings.Contains(r.stdout, "built unknown, ") || strings.Count(r.stdout, "\n") != 1 {
		t.Errorf("version: %q", r.stdout)
	}
}
