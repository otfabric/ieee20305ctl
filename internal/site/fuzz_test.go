// SPDX-License-Identifier: MIT

package site

import (
	"testing"
	"time"
)

// FuzzParseSeed feeds seed files, YAML or JSON. A seed either parses and
// validates or is refused with an error, and what is accepted holds what
// Validate promises, whatever the numbers in the file were.
func FuzzParseSeed(f *testing.F) {
	for _, s := range []string{
		"",
		"pol_rate: 1\n",
		`{"poll_rate": 5, "devices": [{"lfdi": "3E4F45AB31EDFE5B67E343E5E4562E31984E23E5", "pin": 1, "role": "aggregator"}],
		"programs": [{"description": "P", "controls": [{"set": {"opModConnect": true}, "start": "90", "duration": "1h"}]}]}`,
		"devices:\n  - lfdi: AB\n    role: king\n",
		"devices:\n  - lfdi: AB\n    edition: '2011'\n",
		"programs:\n  - description: P\n    controls:\n      - start: soon\n",
		"programs:\n  - description: P\n    default: {opModMaxLimW: 50}\n",
		"tariffs:\n  - description: T\n    currency: 36\n    rate_components:\n      - description: C\n",
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := ParseSeed(data)
		if err != nil {
			if s != nil {
				t.Fatalf("a seed and an error: %v", err)
			}
			return
		}
		if s == nil {
			t.Fatal("no seed and no error")
		}
		for _, p := range s.Programs {
			for _, c := range p.Controls {
				if c.Duration <= 0 {
					t.Fatalf("accepted a control of %v", time.Duration(c.Duration))
				}
			}
		}
		if err := s.Validate(); err != nil {
			t.Fatalf("a parsed seed does not validate: %v", err)
		}
	})
}
