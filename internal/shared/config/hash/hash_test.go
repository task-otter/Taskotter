// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package hash

import (
	"testing"
)

const testPrefixLength = 12

// BenchmarkCompute measures configuration hash generation throughput.
func BenchmarkCompute(b *testing.B) {
	input := Input{
		Tasks:              []string{"go", "golangci-lint", "yamlfix"},
		NodePackageManager: "pnpm",
		TargetFolder:       "taskfiles",
		StoreVersion:       "v0.0.12",
		IncludesDoc:        true,
		SyncRoot:           false,
	}

	b.ReportAllocs()

	for range b.N {
		_, _, err := Compute(&input, testPrefixLength)
		if err != nil {
			b.Fatal(err)
		}
	}
}
