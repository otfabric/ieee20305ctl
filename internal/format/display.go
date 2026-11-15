// SPDX-License-Identifier: MIT

package format

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

// Display returns s as it is when every rune of it is printable (letters,
// marks, numbers, punctuation, symbols and the ASCII space), and otherwise
// quoted as a Go string, with the other runes and invalid UTF-8 escaped.
// Text and table output pass the strings of peers and files through it, so
// that control characters in them are shown as escapes, not interpreted by
// the terminal; JSON output escapes them itself.
func Display(s string) string {
	for i, r := range s {
		if !unicode.IsPrint(r) {
			return strconv.Quote(s)
		}
		if r == utf8.RuneError {
			if _, size := utf8.DecodeRuneInString(s[i:]); size == 1 {
				return strconv.Quote(s)
			}
		}
	}
	return s
}

// displayRows is rows with every cell passed through Display.
func displayRows(rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = make([]string, len(row))
		for j, c := range row {
			out[i][j] = Display(c)
		}
	}
	return out
}
