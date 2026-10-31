// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
)

// FuzzValidateDocument feeds what `validate` reads from files, in both
// profiles. A result is valid with no error, or invalid with one; it never
// is both or neither.
func FuzzValidateDocument(f *testing.F) {
	ctl := &ieee20305.DERControl{}
	ctl.MRID = ieee20305.MRIDType{1, 2, 3, 4}
	ctl.Interval = ieee20305.DateTimeInterval{Start: 1700000000, Duration: 600}
	data, err := ieee20305.Marshal(ctl)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add([]byte(`<DERControl xmlns="urn:ieee:std:2030.5:ns"><mRID>XYZ</mRID></DERControl>`))
	f.Add([]byte(`<Time xmlns="urn:ieee:std:2030.5:ns"><currentTime>1</currentTime></Time>`))
	f.Add([]byte(`<nope/>`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, data []byte) {
		for name, p := range profiles {
			r := validateDocument(ValidationResult{File: "f"}, data, p.decode, p.elementName)
			if r.Valid != (r.Error == "") {
				t.Fatalf("%s: valid=%t, error %q", name, r.Valid, r.Error)
			}
			if r.Valid && r.Element == "" {
				t.Fatalf("%s: valid without an element: %q", name, data)
			}
		}
	})
}
