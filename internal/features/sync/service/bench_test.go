// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"fmt"
	"testing"
)

const (
	benchmarkSmallSize  = 10
	benchmarkMediumSize = 100
	benchmarkLargeSize  = 1000
)

// BenchmarkSortManagedFiles measures managed-file sorting.
func BenchmarkSortManagedFiles(b *testing.B) {
	for _, size := range []int{benchmarkSmallSize, benchmarkMediumSize, benchmarkLargeSize} {
		b.Run(fmt.Sprintf("files_%d", size), func(b *testing.B) {
			runSortManagedFilesBenchmark(b, size)
		})
	}
}

func runSortManagedFilesBenchmark(b *testing.B, size int) {
	b.Helper()

	files := make([]managedFile, 0, size)

	for i := range files {
		files = append(files, managedFile{
			Path:              fmt.Sprintf("taskfiles/module-%04d/Taskfile.yml", size-i),
			SourceModule:      fmt.Sprintf("module-%04d", size-i),
			DestinationModule: fmt.Sprintf("module-%04d", size-i),
		})
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		candidate := append([]managedFile(nil), files...)
		sortManagedFiles(candidate)

		if len(candidate) != size {
			b.Fatalf("sorted file count = %d, want %d", len(candidate), size)
		}
	}
}
