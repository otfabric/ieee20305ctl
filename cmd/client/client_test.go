// SPDX-License-Identifier: MIT

package client

import (
	"strings"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
)

type ieeeUInt8 = ieee20305.UInt8

func TestNotificationStatus(t *testing.T) {
	for s, want := range map[uint8]string{0: "changed", 1: "cancelled-no-info", 2: "cancelled-moved", 3: "cancelled-deleted", 4: "cancelled-other", 9: "status"} {
		if got := notificationStatus(statusOf(s)); got != want {
			t.Errorf("notificationStatus(%d) = %q", s, got)
		}
	}
	if statusName(2) != "started" || statusName(99) != "status 99" {
		t.Error("statusName")
	}
	if _, _, err := scale16(123456); err == nil {
		t.Error("scale16 of a value that does not fit")
	}
}

func statusOf(s uint8) ieeeUInt8 { return ieeeUInt8(s) }

// A DNS-SD answer comes from any host on the link: its instance, host and
// addresses are shown escaped when they hold control characters.
func TestServiceTextEscapesControlCharacters(t *testing.T) {
	const bad = "utility\x1b[8m\r\nieee20305ctl other\u009b"
	s := &Service{Instance: bad, Host: bad, Port: 8443, Addrs: []string{bad}, URL: bad}
	out, err := s.MarshalTextOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "\n") != 2 {
		t.Errorf("a service of %d lines: %q", strings.Count(out, "\n")+1, out)
	}
	for _, r := range out {
		if r != '\n' && (r < 0x20 || (r >= 0x7f && r <= 0x9f)) {
			t.Fatalf("a control character %U in %q", r, out)
		}
	}
}
