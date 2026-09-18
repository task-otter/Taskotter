// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package hash

import (
	"errors"
	"testing"
)

const (
	testPrefixLength = 12
)

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

func TestCompute(t *testing.T) {
	t.Parallel()

	full, branch, err := Compute(&Input{Tasks: []string{"go"}}, testPrefixLength)

	if err != nil || len(full) != 64 || len(branch) != len("taskotter/sync-")+testPrefixLength {
		t.Fatalf("Compute() = %q, %q, %v", full, branch, err)
	}

	for _, prefix := range []int{-1, 65} {
		if _, _, err := Compute(&Input{}, prefix); !errors.Is(err, ErrInvalidPrefixLength) {
			t.Fatalf("prefix %d err = %v", prefix, err)
		}
	}
}
