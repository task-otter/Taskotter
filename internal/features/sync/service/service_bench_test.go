// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"fmt"
	"testing"
)

const (
	benchmarkSmallSize   = 10
	benchmarkMediumSize  = 100
	benchmarkLargeSize   = 1000
	benchmarkEmptyLength = 0
	benchmarkModuleFmt   = "module-%04d"
)

// BenchmarkSortManagedFiles measures managed-file sorting.
func BenchmarkSortManagedFiles(b *testing.B) {
	sizes := []int{benchmarkSmallSize, benchmarkMediumSize, benchmarkLargeSize}

	for index := range sizes {
		size := sizes[index]
		b.Run(fmt.Sprintf("files_%d", size), func(b *testing.B) {
			runSortManagedFilesBenchmark(b, size)
		})
	}
}

func runSortManagedFilesBenchmark(b *testing.B, size int) {
	b.Helper()

	files := benchmarkManagedFiles(size)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		candidate := append([]managedFile(nil), files...)
		sortManagedFiles(candidate)

		if len(candidate) != size {
			benchmarkSortedFileCountFailure(b, len(candidate), size)
		}
	}
}

func benchmarkManagedFiles(size int) []managedFile {
	files := make([]managedFile, benchmarkEmptyLength, size)

	for i := range size {
		files = append(files, managedFile{
			Path:              fmt.Sprintf("taskfiles/"+benchmarkModuleFmt+"/Taskfile.yml", size-i),
			SourceModule:      fmt.Sprintf(benchmarkModuleFmt, size-i),
			DestinationModule: fmt.Sprintf(benchmarkModuleFmt, size-i),
		})
	}

	return files
}

func benchmarkSortedFileCountFailure(b *testing.B, got, want int) {
	b.Helper()
	b.Fatalf("sorted file count = %d, want %d", got, want)
}
