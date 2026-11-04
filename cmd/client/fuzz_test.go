// SPDX-License-Identifier: MIT

package client

import (
	"strconv"
	"strings"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
)

// FuzzParseStatus feeds --status of client respond: a name of a Response
// status, or its number. A number is taken only when it is the decimal of
// a UInt8; nothing wraps.
func FuzzParseStatus(f *testing.F) {
	for _, s := range []string{"received", "COMPLETED", "opt-out", "partial-opt-in", "0", "255", "256", "-1", "+1", "0x10", "1_0", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n, err := parseStatus(s)
		if err != nil {
			return
		}
		if _, named := responseStatuses[strings.ToLower(s)]; named {
			return
		}
		v, perr := strconv.ParseUint(s, 10, 8)
		if perr != nil || uint8(v) != n {
			t.Fatalf("parseStatus(%q) = %d", s, n)
		}
	})
}

// FuzzUnix feeds the times a server gives: whatever the number, the time
// is in the years 1 to 9999 that the JSON output can write.
func FuzzUnix(f *testing.F) {
	for _, v := range []int64{0, -1, 1700000000, 1<<62 - 1, -1 << 62, maxUnix, maxUnix + 1, minUnix, minUnix - 1} {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v int64) {
		tm := unix(ieee20305.TimeType(v))
		if _, err := tm.MarshalJSON(); err != nil {
			t.Fatalf("unix(%d) = %v: %v", v, tm, err)
		}
		if v >= minUnix && v <= maxUnix && tm.Unix() != v {
			t.Fatalf("unix(%d) = %v, which is not the time", v, tm)
		}
	})
}
