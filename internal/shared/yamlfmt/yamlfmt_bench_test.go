// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlfmt

import (
	"fmt"
	"testing"
)

const (
	benchmarkSmallSize  = 10
	benchmarkMediumSize = 100
	benchmarkLargeSize  = 1000
	benchmarkFirstIndex = 0
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
		benchmarkMarshalValue(b, value)
	}
}

func benchmarkMarshalValue(b *testing.B, value map[string]string) {
	b.Helper()

	data, err := Marshal(value)

	if err != nil || len(data) == benchmarkFirstIndex {
		b.Fatalf("marshal yaml: %v", err)
	}
}
