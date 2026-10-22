// SPDX-License-Identifier: MIT

package client

import (
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
