// SPDX-License-Identifier: MIT

package ieee

import (
	"strings"
	"testing"
)

func TestParseAndDescribe(t *testing.T) {
	b, err := ParseSettings([]string{"opModConnect=true", "opModMaxLimW=50", "opModExpLimW=1500", "opModTargetW=-40000", "opModFixedW=-12.5"})
	if err != nil {
		t.Fatal(err)
	}
	got := Describe(&b)
	for _, want := range []string{"opModConnect=true", "opModMaxLimW=50.00 %", "opModExpLimW=1500 W", "opModTargetW=-40000 W", "opModFixedW=-12.50 %"} {
		if !strings.Contains(got, want) {
			t.Errorf("Describe = %q, want %q in it", got, want)
		}
	}
	if b.OpModTargetW.Multiplier != 1 || b.OpModTargetW.Value != -4000 {
		t.Errorf("opModTargetW %+v", b.OpModTargetW)
	}
	for _, bad := range []string{"opModNothing=1", "opModConnect=maybe", "opModMaxLimW=50.123", "opModMaxLimW", "opModFreqWatt=/x"} {
		if _, err := ParseSettings([]string{bad}); err == nil {
			t.Errorf("ParseSettings(%q) succeeded", bad)
		}
	}
	if Describe(&b) == "" || len(Names()) < 20 {
		t.Errorf("Names %v", Names())
	}
}
