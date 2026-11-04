// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"strings"
	"testing"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/format"
)

// Only the agent's reports of a failed start become records.
func TestAgentLog(t *testing.T) {
	var buf bytes.Buffer
	w := &lockedWriter{out: format.NewStreamWriter(&buf, format.FormatJSON)}
	l := agentLog{w: w}
	l.Debugf("debug %d", 1)
	l.Infof("info %d", 2)
	l.Warnf("poll failed: %v", "x")
	l.Errorf("other: %v", "y")
	if buf.Len() != 0 {
		t.Fatalf("records of other log lines: %s", buf.String())
	}
	l.Warnf("start failed; again in %v: %v", "1s", "503")
	l.Errorf("cannot start: %v", "certificate")
	out := buf.String()
	if strings.Count(out, `"start_failed"`) != 2 || !strings.Contains(out, "503") || !strings.Contains(out, "certificate") {
		t.Errorf("records: %s", out)
	}
}

// The bound of a command's lists counts every document read.
func TestCount(t *testing.T) {
	d := &conn.Device{}
	if err := count(d, "/x", maxListed); err != nil {
		t.Fatal(err)
	}
	if err := count(d, "/x", 1); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Errorf("count beyond maxListed: %v", err)
	}
}
