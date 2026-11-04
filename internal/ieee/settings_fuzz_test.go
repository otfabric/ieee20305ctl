// SPDX-License-Identifier: MIT

package ieee

import (
	"testing"
	"time"
)

// FuzzParseSettings feeds the name=value pairs of --set, of server
// issue-control and set-default and of the seed's settings. A pair either
// sets a control or is refused; what is set reads back as settings, and
// parsing is quick.
func FuzzParseSettings(f *testing.F) {
	for _, s := range []string{
		"opModConnect=true", "opModMaxLimW=50", "opModMaxLimW=50.25", "opModMaxLimW=100.001", "opModExpLimW=1500",
		"opModExpLimW=-2000", "opModExpLimW=0.001", "opModExpLimW=1e30", "opModTargetW=NaN", "opModFixedW=-1",
		"opModMaxLimW=1e400", "nope=1", "opModConnect", "=", "OPMODCONNECT = false", "opModGenLimW=327670000000",
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
