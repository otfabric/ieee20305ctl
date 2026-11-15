// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// controlFree reports whether s has no byte below 0x20 other than newline
// and no rune of U+007F–U+009F, and is valid UTF-8.
func controlFree(s string, newlines bool) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r == '\n' && newlines {
			continue
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return false
		}
	}
	return true
}

// unprintable are strings with characters that are not printable, as a
// peer or a file may hold them.
var unprintable = []string{
	"a\x1b[31mred", "bell\a", "carriage\rreturn", "two\nlines", "tab\there", "csi\u009b2J", "del\x7f",
	"bidi\u202eoverride", "invalid\xffutf8", "nul\x00",
}

// A printable string is shown as it is; anything else is quoted, with the
// characters that are not printable escaped.
func TestDisplay(t *testing.T) {
	for _, s := range []string{"", "/edev/1/der/1", "Zürich", "東京 2030.5", `quote " and \ backslash`, "ieee20305ctl host-1", "�"} {
		if got := Display(s); got != s {
			t.Errorf("Display(%q) = %q, want it unchanged", s, got)
		}
	}
	for _, s := range unprintable {
		got := Display(s)
		if !controlFree(got, false) || !strings.HasPrefix(got, `"`) {
			t.Errorf("Display(%q) = %q", s, got)
		}
	}
}

// A table renders unprintable cells escaped, one line per row.
func TestTablesDisplayCells(t *testing.T) {
	rows := [][]string{}
	for _, s := range unprintable {
		rows = append(rows, []string{s, "x"})
	}
	tm := grid{headers: []string{"A", "B"}, rows: rows}
	out, err := RenderTable(tm)
	if err != nil {
		t.Fatal(err)
	}
	if !controlFree(out, true) || strings.Count(out, "\n") != len(rows)+2 {
		t.Errorf("RenderTable:\n%s", out)
	}
	var b bytes.Buffer
	sw := NewStreamWriter(&b, FormatTable)
	if err := sw.Write(tm); err != nil {
		t.Fatal(err)
	}
	if !controlFree(b.String(), true) || strings.Count(b.String(), "\n") != len(rows)+2 {
		t.Errorf("stream table:\n%s", b.String())
	}
}

type grid struct {
	headers []string
	rows    [][]string
}

func (t grid) TableHeaders() []string { return t.headers }
func (t grid) TableRows() [][]string  { return t.rows }
