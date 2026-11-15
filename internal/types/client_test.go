// SPDX-License-Identifier: MIT

package types

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/ieee20305ctl/internal/ieee"
)

func TestAheadBehind(t *testing.T) {
	for ms, want := range map[int64]string{1: "ahead", -1: "behind", 0: "in step"} {
		if got := aheadBehind(ms); got != want {
			t.Errorf("%d: %q, want %q", ms, got, want)
		}
	}
}

func TestPriceText(t *testing.T) {
	neg, pos := int8(-2), int8(2)
	for _, c := range []struct {
		p    int64
		mult *int8
		want string
	}{{1234, nil, "1234"}, {1234, &neg, "12.34/kWh"}, {12, &pos, "1200/kWh"}} {
		if got := priceText(c.p, c.mult); got != c.want {
			t.Errorf("%d: %q, want %q", c.p, got, c.want)
		}
	}
}

func TestPostedAndWrittenTables(t *testing.T) {
	p := &Posted{Action: "post", Target: "/t", Location: "/l", Detail: "d"}
	if len(p.TableHeaders()) != len(p.TableRows()[0]) {
		t.Error("Posted: headers and row differ in length")
	}
	if got := p.TableRows(); !reflect.DeepEqual(got, [][]string{{"post", "/t", "/l", "d"}}) {
		t.Errorf("Posted: %v", got)
	}
	w := &Written{Action: "put", Hrefs: []string{"/a", "/b"}}
	if s, _ := w.MarshalTextOutput(); s != "put: /a, /b" {
		t.Errorf("Written: %q", s)
	}
	if got := w.TableRows(); !reflect.DeepEqual(got, [][]string{{"put", "/a"}, {"put", "/b"}}) || len(w.TableHeaders()) != 2 {
		t.Errorf("Written: %v", got)
	}
	if got := (&Written{}).TableRows(); got == nil || len(got) != 0 {
		t.Errorf("no hrefs: %v", got)
	}
}

func TestConnectionPoint(t *testing.T) {
	c := &ConnectionPoint{EndDevice: "/edev/1", Href: "/edev/1/cp"}
	if s, _ := c.MarshalTextOutput(); !strings.HasSuffix(s, "not set") {
		t.Errorf("unset: %q", s)
	}
	c.ID = "NMI1"
	if s, _ := c.MarshalTextOutput(); s != "connection point /edev/1/cp: NMI1" {
		t.Errorf("set: %q", s)
	}
	if got := c.TableRows(); !reflect.DeepEqual(got, [][]string{{"/edev/1", "/edev/1/cp", "NMI1"}}) || len(c.TableHeaders()) != 3 {
		t.Errorf("table: %v", got)
	}
}

func TestEventForms(t *testing.T) {
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	end := at.Add(time.Hour)
	e := &Event{
		Time: at, Kind: "control", Status: "active", Href: "/h", Resource: "/r", MRID: "AB", LFDI: "CD",
		Address: "1.2.3.4", Message: "m", Code: 200, Start: &at, End: &end,
		Settings: []ieee.Setting{{Name: "opModConnect", Value: "true"}},
	}
	s, err := e.MarshalTextOutput()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"12:00:00.000 control", "active", "/h", "resource /r", "mrid AB", "lfdi CD", "1.2.3.4", " m", " 200", "opModConnect", "12:00:00..13:00:00"} {
		if !strings.Contains(s, want) {
			t.Errorf("%q has no %q", s, want)
		}
	}
	row := e.TableRows()[0]
	if len(row) != len(e.TableHeaders()) {
		t.Fatalf("row %v", row)
	}
	if row[2] != "active 200" || row[3] != "/h" || !strings.HasPrefix(row[4], "m ") || !strings.HasSuffix(row[4], "lfdi CD 1.2.3.4") {
		t.Errorf("row %q", row)
	}
	if row := (&Event{Time: at, Kind: "x", Resource: "/r"}).TableRows()[0]; row[3] != "/r" || row[2] != "" || row[4] != "" {
		t.Errorf("sparse row %q", row)
	}
}

func TestErrorMessage(t *testing.T) {
	if ErrorMessage(nil) != "" {
		t.Error("nil has a message")
	}
	if got := ErrorMessage(&ErrorInfo{Message: "m"}); got != "m" {
		t.Errorf("%q", got)
	}
}

// controlFree reports whether s has no byte below 0x20 but the renderer's
// own newlines, and no rune of U+007F–U+009F.
func controlFree(s string) bool {
	for _, r := range s {
		if r != '\n' && (r < 0x20 || (r >= 0x7f && r <= 0x9f)) {
			return false
		}
	}
	return true
}

// The strings of a server (hrefs, ids, messages that quote them) are shown
// escaped when they hold control characters: an event stays one line.
func TestRenderersEscapeControlCharacters(t *testing.T) {
	const bad = "/derp/1\x1b]0;title\a\r\n12:00:00.000 control_issued\u009b2J"
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	settings := []ieee.Setting{{Name: "opModVoltVar", Value: bad, Text: bad}}
	ev := &Event{Time: now, Kind: "log", Href: bad, Resource: bad, Status: bad, MRID: bad, LFDI: bad, Address: bad, Message: bad, Settings: settings}
	for _, m := range []interface{ MarshalTextOutput() (string, error) }{
		ev,
		&Controls{Controls: []Control{{Program: bad, Href: bad, Description: bad, Status: "Active", Settings: settings, ReplyTo: bad, ResponseRequired: bad}}},
		&Programs{Programs: []Program{{Href: bad, Description: bad, Default: settings}}},
		&Capability{URL: bad, Links: []Link{{Name: "EndDeviceListLink", Href: bad}}},
		&Registration{EndDevice: bad, LFDI: "L", SFDI: "S"},
		&WalkStep{Href: bad, Element: bad},
		&Posted{Action: "post", Target: bad, Location: bad, Detail: bad},
		&Written{Action: "put", Hrefs: []string{bad, bad}},
		&DERs{DERs: []DER{{Href: bad, Resources: []DERResource{{Name: "settings", Href: bad}}}}},
		&Pricing{Tariffs: []Tariff{{Href: bad, Description: bad, RateComponents: []RateComponent{{Href: bad, Description: bad}}}}},
		&ConnectionPoint{Href: bad, ID: bad},
	} {
		out, err := m.MarshalTextOutput()
		if err != nil {
			t.Fatal(err)
		}
		if !controlFree(out) {
			t.Errorf("%T: %q", m, out)
		}
	}
	if out, _ := ev.MarshalTextOutput(); strings.Contains(out, "\n") {
		t.Errorf("an event of two lines: %q", out)
	}
}
