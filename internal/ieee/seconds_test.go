// SPDX-License-Identifier: MIT

package ieee

import (
	"math"
	"testing"
	"time"
)

func TestSeconds(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want uint32
	}{{-time.Hour, 0}, {0, 0}, {1500 * time.Millisecond, 1}, {time.Hour, 3600}, {math.MaxInt64, math.MaxUint32}} {
		if got := Seconds(c.d); uint32(got) != c.want {
			t.Errorf("Seconds(%v) = %d, want %d", c.d, got, c.want)
		}
	}
}
