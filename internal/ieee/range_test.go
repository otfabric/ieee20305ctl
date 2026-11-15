// SPDX-License-Identifier: MIT

package ieee

import (
	"fmt"
	"testing"
)

// A power takes a multiplier of -9 to 9: the largest magnitude that fits is
// 32767 × 10^9, and anything larger is refused, not stored with another
// multiplier.
func TestParseSettingsPowerBounds(t *testing.T) {
	for _, c := range []struct {
		in    string
		ok    bool
		value int64
		mult  int64
	}{
		{"0", true, 0, 0},
		{"1", true, 1, 0},
		{"32767", true, 32767, 0},
		{"0.001", true, 1, -3},
		{"32767e8", true, 32767, 8},
		{"32767e9", true, 32767, 9},
		{"-32767e9", true, -32767, 9},
		{"32767000000001", false, 0, 0},
		{"32768e9", false, 0, 0},
		{"-32768e9", false, 0, 0},
		{"32767e10", false, 0, 0},
		{"1e14", false, 0, 0},
		{"1e30", false, 0, 0},
		{"-1e30", false, 0, 0},
		{"1e300", false, 0, 0},
		{"1.7976931348623157e308", false, 0, 0},
		{"0.0001", false, 0, 0},
	} {
		b, err := ParseSettings([]string{"opModTargetW=" + c.in})
		if !c.ok {
			if err == nil {
				t.Errorf("opModTargetW=%s was accepted: %+v", c.in, b.OpModTargetW)
			}
			continue
		}
		if err != nil {
			t.Errorf("opModTargetW=%s: %v", c.in, err)
			continue
		}
		if got := b.OpModTargetW; int64(got.Value) != c.value || int64(got.Multiplier) != c.mult {
			t.Errorf("opModTargetW=%s = %d e%d, want %d e%d", c.in, got.Value, got.Multiplier, c.value, c.mult)
		}
	}
	// The site limits of CSIP-AUS and the unsigned powers are bounded alike.
	for _, pair := range []string{"opModExpLimW=1e20", "opModMaxLimWInject=1e20", "opModTargetV=4e15", "opModGenLimW=-1e17"} {
		if b, err := ParseSettings([]string{pair}); err == nil {
			t.Errorf("%s was accepted: %s", pair, Describe(&b))
		}
	}
}

// A percentage is 0 to 100.00 % (PerCent), or -100.00 to 100.00 %
// (SignedPerCent), in hundredths; a value outside is refused.
func TestParseSettingsPercentBounds(t *testing.T) {
	for _, c := range []struct {
		pair string
		ok   bool
		want int64
	}{
		{"opModMaxLimW=0", true, 0},
		{"opModMaxLimW=0.01", true, 1},
		{"opModMaxLimW=99.99", true, 9999},
		{"opModMaxLimW=100", true, 10000},
		{"opModMaxLimW=100.00", true, 10000},
		{"opModMaxLimW=100.01", false, 0},
		{"opModMaxLimW=101", false, 0},
		{"opModMaxLimW=655.35", false, 0},
		{"opModMaxLimW=-0.01", false, 0},
		{"opModMaxLimW=-1", false, 0},
		{"opModMaxLimW=1e10", false, 0},
		{"opModMaxLimW=Inf", false, 0},
		{"opModMaxLimW=NaN", false, 0},
		{"opModMaxLimW=50.001", false, 0},
		{"opModMaxLimPctVAInject=100.01", false, 0},
		{"opModFixedW=-100", true, -10000},
		{"opModFixedW=-100.00", true, -10000},
		{"opModFixedW=100", true, 10000},
		{"opModFixedW=-100.01", false, 0},
		{"opModFixedW=100.01", false, 0},
		{"opModFixedW=327.67", false, 0},
		{"opModFixedW=-327.68", false, 0},
		{"opModFixedV=100.01", false, 0},
	} {
		b, err := ParseSettings([]string{c.pair})
		if !c.ok {
			if err == nil {
				t.Errorf("%s was accepted: %s", c.pair, Describe(&b))
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.pair, err)
			continue
		}
		var got int64
		switch {
		case b.OpModMaxLimW != nil:
			got = int64(b.OpModMaxLimW.Value)
		case b.OpModFixedW != nil:
			got = int64(b.OpModFixedW.Value)
		}
		if got != c.want {
			t.Errorf("%s = %d hundredths, want %d", c.pair, got, c.want)
		}
	}
	// The controls whose value is a PerCent or SignedPerCent without being
	// a percentage of the tool (opModFixedVar, opModMaxLimPctVar*) take it
	// in hundredths, and have the same range.
	for _, c := range []struct {
		pair string
		ok   bool
	}{
		{"opModFixedVar=10000", true}, {"opModFixedVar=-10000", true}, {"opModFixedVar=10001", false}, {"opModFixedVar=-10001", false},
		{"opModMaxLimPctVarInject=10000", true}, {"opModMaxLimPctVarInject=10001", false}, {"opModMaxLimPctVarAbsorb=-1", false},
	} {
		if _, err := ParseSettings([]string{c.pair}); (err == nil) != c.ok {
			t.Errorf("%s: %v, want accepted %v", c.pair, err, c.ok)
		}
	}
}

// Every percentage with two decimals from -100.00 to 100.00 is taken, with
// its exact hundredths: 0.29 × 100 is not an integer in binary.
func TestParseSettingsTwoDecimalPercentages(t *testing.T) {
	for i := -10000; i <= 10000; i++ {
		sign := ""
		if i < 0 {
			sign = "-"
		}
		a := i
		if a < 0 {
			a = -a
		}
		s := fmt.Sprintf("%s%d.%02d", sign, a/100, a%100)
		b, err := ParseSettings([]string{"opModFixedW=" + s})
		if err != nil {
			t.Fatalf("opModFixedW=%s: %v", s, err)
		}
		if int(b.OpModFixedW.Value) != i {
			t.Fatalf("opModFixedW=%s = %d hundredths, want %d", s, b.OpModFixedW.Value, i)
		}
		if i < 0 {
			continue
		}
		b, err = ParseSettings([]string{"opModMaxLimW=" + s})
		if err != nil {
			t.Fatalf("opModMaxLimW=%s: %v", s, err)
		}
		if int(b.OpModMaxLimW.Value) != i {
			t.Fatalf("opModMaxLimW=%s = %d hundredths, want %d", s, b.OpModMaxLimW.Value, i)
		}
	}
}
