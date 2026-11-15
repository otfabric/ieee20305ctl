// SPDX-License-Identifier: MIT

package ieee

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
)

// FuzzParseSettings feeds the name=value pairs of --set, of server
// issue-control and set-default and of the seed's settings. A pair either
// sets a control or is refused; what is set reads back as settings, and
// parsing is quick. What is accepted is within the ranges of IEEE 2030.5:
// a multiplier of -9 to 9 with value × 10^multiplier the number given, a
// PerCent of 0 to 10000 and a SignedPerCent of -10000 to 10000.
func FuzzParseSettings(f *testing.F) {
	for _, s := range []string{
		"opModConnect=true", "opModMaxLimW=50", "opModMaxLimW=50.25", "opModMaxLimW=100.001", "opModExpLimW=1500",
		"opModExpLimW=-2000", "opModExpLimW=0.001", "opModExpLimW=1e30", "opModTargetW=NaN", "opModFixedW=-1",
		"opModMaxLimW=1e400", "nope=1", "opModConnect", "=", "OPMODCONNECT = false", "opModGenLimW=327670000000",
		"opModTargetW=32767e9", "opModTargetW=32768e9", "opModMaxLimW=100.01", "opModFixedW=-100.01", "opModMaxLimW=0.29",
		"opModFixedVar=10001", "opModTargetV=1e300",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, pair string) {
		b, err := ParseSettings([]string{pair})
		if err != nil {
			return
		}
		_ = Settings(&b)
		_ = Describe(&b)
		_ = CSIPAUSOnly(&b)
		_, raw, _ := strings.Cut(pair, "=")
		x, perr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		walk(reflect.ValueOf(&b), func(name string, fv reflect.Value) {
			if fv.Kind() != reflect.Pointer || fv.IsNil() || fv.Elem().Kind() != reflect.Struct {
				return
			}
			value, mult, _ := fieldsOf(fv.Elem())
			if !value.IsValid() {
				return
			}
			n := intOf(value)
			switch {
			case mult.IsValid():
				m := intOf(mult)
				if m < -9 || m > 9 {
					t.Fatalf("%q: %s has multiplier %d", pair, name, m)
				}
				if perr != nil {
					t.Fatalf("%q: a power from a value that is not a number", pair)
				}
				if got := float64(n) * math.Pow10(int(m)); math.Abs(got-x) > 1e-9*math.Abs(x) {
					t.Fatalf("%q: %s is %d e%d = %g, not %g", pair, name, n, m, got, x)
				}
			case value.Type() == reflect.TypeFor[ieee20305.PerCent]():
				if n < 0 || n > 10000 {
					t.Fatalf("%q: %s is %d hundredths of a percent", pair, name, n)
				}
			case value.Type() == reflect.TypeFor[ieee20305.SignedPerCent]():
				if n < -10000 || n > 10000 {
					t.Fatalf("%q: %s is %d hundredths of a percent", pair, name, n)
				}
			}
		})
	})
}

// An infinite value of a control of a power is refused, not scaled forever:
// ParseFloat takes "Inf", and a seed's .inf is "+Inf" by the time it is a
// pair.
func TestParseSettingsInfinity(t *testing.T) {
	for _, v := range []string{"Inf", "+Inf", "-inf", "infinity"} {
		done := make(chan error, 1)
		go func() {
			_, err := ParseSettings([]string{"opModExpLimW=" + v})
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("opModExpLimW=%s was accepted", v)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("ParseSettings(opModExpLimW=%s) did not return", v)
		}
	}
}
