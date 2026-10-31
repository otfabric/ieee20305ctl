// SPDX-License-Identifier: MIT

package site

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSeed(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// JSON is YAML; durations as strings or seconds.
	s, err := LoadSeed(write("seed.json", `{"poll_rate": 5, "devices": [{"lfdi": "3E4F45AB31EDFE5B67E343E5E4562E31984E23E5", "pin": 1, "role": "aggregator"}],
		"programs": [{"description": "P", "controls": [{"set": {"opModConnect": true}, "start": "90", "duration": "1h"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.PollRate != 5 || time.Duration(s.Programs[0].Controls[0].Start) != 90*time.Second ||
		time.Duration(s.Programs[0].Controls[0].Duration) != time.Hour {
		t.Errorf("seed %+v", s)
	}
	var d Duration
	if err := d.UnmarshalJSON([]byte(`"15m"`)); err != nil || time.Duration(d) != 15*time.Minute {
		t.Errorf("UnmarshalJSON: %v, %v", d, err)
	}
	for name, bad := range map[string]string{
		"unknown":  "pol_rate: 1\n",
		"device":   "devices:\n  - pin: 1\n",
		"role":     "devices:\n  - lfdi: AB\n    role: king\n",
		"edition":  "devices:\n  - lfdi: AB\n    edition: '2011'\n",
		"duration": "programs:\n  - description: P\n    controls:\n      - start: soon\n",
		"zero":     "programs:\n  - description: P\n    controls:\n      - set: {opModConnect: true}\n",
		"no-desc":  "programs:\n  - primacy: 1\n",
		"not yaml": ":\n:",
	} {
		if _, err := LoadSeed(write(name+".yaml", bad)); err == nil {
			t.Errorf("%s: LoadSeed succeeded", name)
		}
	}
	if _, err := LoadSeed(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("LoadSeed of nothing")
	}
	if _, err := mrid("XY", ""); err == nil {
		t.Error("a bad mrid")
	}
	if a, _ := mrid("", "a"); len(a) != 16 {
		t.Error("a derived mrid")
	}
}

// A number of seconds too large for a time.Duration used to wrap around,
// to a value that has nothing to do with what the file says.
func TestDurationOverflowIsRefused(t *testing.T) {
	for _, s := range []string{"3600000000000", "-3600000000000"} {
		var d Duration
		if err := d.parse(s); err == nil {
			t.Errorf("%s: parsed as %v", s, time.Duration(d))
		}
	}
	var d Duration
	if err := d.parse("9223372036"); err != nil || time.Duration(d) != 9223372036*time.Second {
		t.Errorf("the largest number of seconds: %v, %v", time.Duration(d), err)
	}
}
