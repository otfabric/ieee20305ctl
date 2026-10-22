// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/ieee20305ctl/internal/errs"
)

const (
	validEndDevice = `<EndDevice xmlns="urn:ieee:std:2030.5:ns" href="/edev/3"><sFDI>987654321005</sFDI><changedTime>1379905200</changedTime></EndDevice>`
	// An sFDI beyond the 36 bits and check digit of SFDIType.
	invalidEndDevice = `<EndDevice xmlns="urn:ieee:std:2030.5:ns"><sFDI>999999999999999</sFDI><changedTime>0</changedTime></EndDevice>`
	// A CSIP-AUS DOE mode bitmap is one byte (HexBinary8); this one is two.
	csipCapability = `<DERCapability xmlns="urn:ieee:std:2030.5:ns" xmlns:csipaus="https://csipaus.org/ns/v1.3"><modesSupported>00000001</modesSupported><rtgMaxW><multiplier>0</multiplier><value>5000</value></rtgMaxW><type>4</type><csipaus:doeModesSupported>0F0F</csipaus:doeModesSupported></DERCapability>`
)

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func file(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func code(err error) int {
	var c *errs.ExitCodeError
	if errors.As(err, &c) {
		return c.Code
	}
	return -1
}

func TestValidateValid(t *testing.T) {
	out, err := runRoot(t, "validate", file(t, "ed.xml", validEndDevice))
	if err != nil || !strings.Contains(out, "valid EndDevice") {
		t.Errorf("out = %q, err = %v", out, err)
	}
}

func TestValidateInvalid(t *testing.T) {
	good := file(t, "good.xml", validEndDevice)
	bad := file(t, "bad.xml", invalidEndDevice)
	junk := file(t, "junk.xml", "<EndDevice")
	unknown := file(t, "unknown.xml", `<Nope xmlns="urn:ieee:std:2030.5:ns"/>`)
	out, err := runRoot(t, "validate", good, bad, junk, unknown, filepath.Join(t.TempDir(), "missing.xml"))
	if code(err) != errs.ExitInvalid || !strings.Contains(err.Error(), "4 of 5") {
		t.Errorf("err = %v, want exit status 5 for 4 of 5", err)
	}
	for _, want := range []string{"good.xml: valid EndDevice", "bad.xml: invalid: ", "sFDI", "junk.xml: invalid", "unknown document element", "missing.xml: invalid"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestValidateJSON(t *testing.T) {
	out, err := runRoot(t, "validate", "-o", "json", file(t, "good.xml", validEndDevice), file(t, "bad.xml", invalidEndDevice))
	if code(err) != errs.ExitInvalid {
		t.Errorf("err = %v", err)
	}
	var env struct {
		Status string
		Error  string
		Data   []ValidationResult
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if env.Status != "error" || len(env.Data) != 2 || !env.Data[0].Valid || env.Data[1].Valid || env.Data[0].Element != "EndDevice" {
		t.Errorf("JSON result = %s", out)
	}
}

func TestValidateProfile(t *testing.T) {
	doc := file(t, "cap.xml", csipCapability)
	// Read as IEEE 2030.5, the CSIP-AUS element is an extension and not checked.
	if _, err := runRoot(t, "validate", doc); err != nil {
		t.Errorf("ieee20305 profile: %v", err)
	}
	out, err := runRoot(t, "validate", "--profile", "csipaus", doc)
	if code(err) != errs.ExitInvalid || !strings.Contains(out, "doeModesSupported") {
		t.Errorf("csipaus profile: out = %q, err = %v", out, err)
	}
	if _, err := runRoot(t, "validate", "--profile", "nope", doc); code(err) != errs.ExitUsage {
		t.Errorf("unknown profile: err = %v", err)
	}
}
