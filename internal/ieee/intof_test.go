// SPDX-License-Identifier: MIT

package ieee

import (
	"math"
	"reflect"
	"testing"
)

func TestIntOf(t *testing.T) {
	for v, want := range map[any]int64{int8(-3): -3, uint16(7): 7, uint64(math.MaxUint64): math.MaxInt64, "x": 0} {
		if got := intOf(reflect.ValueOf(v)); got != want {
			t.Errorf("intOf(%T %v) = %d, want %d", v, v, got, want)
		}
	}
}
