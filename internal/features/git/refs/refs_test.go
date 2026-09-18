// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package refs

import (
	"testing"
)

// BenchmarkValidate measures Git ref validation throughput.
func BenchmarkValidate(b *testing.B) {
	for range b.N {
		err := Validate("taskotter/sync-abcdef123456")
		if err != nil {
			b.Fatal(err)
		}
	}
}
