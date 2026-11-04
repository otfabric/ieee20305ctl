// SPDX-License-Identifier: MIT

package ieee

import (
	"math"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
)

// Seconds returns d in whole seconds as the UInt32 of a duration on the
// wire: 0 for a negative d, and at most 2^32 - 1 s (about 136 years)
// instead of wrapping around.
func Seconds(d time.Duration) ieee20305.UInt32 {
	s := int64(d / time.Second)
	if s < 0 {
		return 0
	}
	if s > math.MaxUint32 {
		return math.MaxUint32
	}
	return ieee20305.UInt32(s)
}
