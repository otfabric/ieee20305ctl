// SPDX-License-Identifier: MIT

package site

import (
	"strings"
	"testing"
	"time"
)

// Numbers a seed field cannot hold, unknown fields at any depth and
// durations out of range are refused, not wrapped or ignored.
func TestParseSeedRefusesHostileValues(t *testing.T) {
	for name, bad := range map[string]string{
		"poll_rate negative":      "poll_rate: -1\n",
		"poll_rate overflow":      "poll_rate: 4294967296\n",
		"pin overflow":            "devices:\n  - lfdi: AB\n    pin: 4294967296\n",
		"primacy overflow":        "programs:\n  - description: P\n    primacy: 256\n",
		"currency overflow":       "tariffs:\n  - description: T\n    currency: 65536\n",
		"multiplier overflow":     "tariffs:\n  - description: T\n    price_multiplier: 128\n",
		"price overflow":          "tariffs:\n  - description: T\n    rate_components:\n      - description: C\n        intervals:\n          - price: 2147483648\n",
		"tou_tier overflow":       "tariffs:\n  - description: T\n    rate_components:\n      - description: C\n        intervals:\n          - tou_tier: 256\n",
		"unknown nested field":    "programs:\n  - description: P\n    controls:\n      - sett: {opModConnect: true}\n        duration: 1h\n",
		"unknown interval field":  "tariffs:\n  - description: T\n    rate_components:\n      - description: C\n        intervals:\n          - prize: 1\n",
		"duration seconds huge":   "programs:\n  - description: P\n    controls:\n      - duration: 9223372037\n",
		"duration overflow":       "programs:\n  - description: P\n    controls:\n      - duration: 9999999999h\n",
		"duration NaN":            "programs:\n  - description: P\n    controls:\n      - duration: NaN\n",
		"duration negative":       "programs:\n  - description: P\n    controls:\n      - duration: -1h\n",
		"duration zero":           "programs:\n  - description: P\n    controls:\n      - duration: 0s\n",
		"start a map":             "programs:\n  - description: P\n    controls:\n      - start: {a: 1}\n        duration: 1h\n",
		"programs a scalar":       "programs: 5\n",
		"mrid too long":           "programs:\n  - description: P\n    mrid: " + strings.Repeat("AB", 17) + "\n",
		"a sequence at the root":  "- 1\n- 2\n",
		"a tag of another type":   "poll_rate: !!binary aGVsbG8=\n",
		"description not unicode": "programs:\n  - description: \"\\xff\"\n    controls: [{duration: 1h}]\n",
	} {
		t.Run(name, func(t *testing.T) {
			s, err := ParseSeed([]byte(bad))
			if name == "mrid too long" || name == "description not unicode" {
				// The mRID is checked when the server applies the seed; a
				// description is a string. They may parse.
				return
			}
			if err == nil {
				t.Fatalf("accepted %q: %+v", bad, s)
			}
		})
	}
}

// A seed of YAML aliases that expand to a huge document (a billion laughs),
// or of deep nesting, is refused or parsed quickly, not expanded.
func TestParseSeedBoundedOnHostileYAML(t *testing.T) {
	laughs := "a: &a [\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\"]\n"
	prev := "a"
	for _, n := range []string{"b", "c", "d", "e", "f", "g", "h", "i"} {
		laughs += n + ": &" + n + " [" + strings.TrimSuffix(strings.Repeat("*"+prev+",", 9), ",") + "]\n"
		prev = n
	}
	for name, doc := range map[string]string{
		"aliases":   "programs:\n  - description: P\n    default:\n" + indent(laughs, "      "),
		"deep flow": "programs:\n  - description: P\n    default:\n      x: " + strings.Repeat("[", 100000) + strings.Repeat("]", 100000) + "\n",
		"deep map":  "programs:\n  - description: P\n    default:\n      x: " + strings.Repeat("{a: ", 10000) + "1" + strings.Repeat("}", 10000) + "\n",
		"long key":  "programs:\n  - description: P\n    default:\n      " + strings.Repeat("k", 1<<20) + ": 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				_, _ = ParseSeed([]byte(doc)) // refused or parsed: either is fine, quickly
			}()
			select {
			case <-done:
			case <-time.After(60 * time.Second):
				t.Fatal("ParseSeed did not finish in a minute")
			}
		})
	}
}

func indent(s, by string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(strings.TrimSuffix(s, "\n"), "\n") {
		b.WriteString(by + line + "\n")
	}
	return b.String()
}

// A tariff interval of no time, or of negative time, is not a seed the
// server can serve: it is refused like a control without a positive
// duration.
func TestParseSeedRefusesEmptyTariffIntervals(t *testing.T) {
	for _, d := range []string{"0s", "-1h"} {
		seed := "tariffs:\n  - description: T\n    rate_components:\n      - description: C\n        intervals:\n          - description: I\n            duration: " + d + "\n"
		if _, err := ParseSeed([]byte(seed)); err == nil {
			t.Errorf("a tariff interval of duration %s was accepted", d)
		}
	}
}

// A seed file of two YAML documents is refused: the second would be dropped
// without a word.
func TestParseSeedRefusesSecondDocument(t *testing.T) {
	seed := "poll_rate: 5\n---\nprograms:\n  - description: P\n    controls: [{duration: 1h}]\n"
	if _, err := ParseSeed([]byte(seed)); err == nil {
		t.Error("a seed of two documents was accepted")
	}
}
