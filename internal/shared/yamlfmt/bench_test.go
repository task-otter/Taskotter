// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlfmt

import (
	"fmt"
	"testing"
)

const (
	benchmarkSmallSize   = 10
	benchmarkMediumSize  = 100
	benchmarkLargeSize   = 1000
	benchmarkFirstIndex  = 0
	benchmarkEmptyLength = 0 //nolint:goconst // benchmark sentinel is local to this benchmark
)

// BenchmarkMarshal measures YAML marshaling.
func BenchmarkMarshal(b *testing.B) {
	sizes := []int{benchmarkSmallSize, benchmarkMediumSize, benchmarkLargeSize}

	for index := range sizes {
		size := sizes[index]
		b.Run(fmt.Sprintf("entries_%d", size), func(b *testing.B) {
			runMarshalBenchmark(b, size)
		})
	}
}

//nolint:funlen // benchmark setup and measurement are clearer together
func runMarshalBenchmark(b *testing.B, size int) {
	b.Helper()

	value := make(map[string]string, size)

	for i := range size {
		value[fmt.Sprintf("key-%d", i+benchmarkFirstIndex)] = fmt.Sprintf(
			"value-%d",
			i+benchmarkFirstIndex,
		)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		data, err := Marshal(value)

		if err != nil || len(data) == benchmarkEmptyLength {
			b.Fatalf("marshal yaml: %v", err)
		}
	}
}
