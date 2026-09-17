// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"fmt"
	"testing"
)

const (
	benchmarkSmallSize  = 10
	benchmarkMediumSize = scoreIdenticalString
	benchmarkLargeSize  = scoreExactMatch
	benchmarkModuleFmt  = "module-%d"
	benchmarkOffset     = 1
)

// BenchmarkResolveTransitive measures dependency traversal at several graph sizes.
func BenchmarkResolveTransitive(b *testing.B) {
	benchmarkSizes(b, "modules", runResolveTransitiveBenchmark)
}

func runResolveTransitiveBenchmark(b *testing.B, size int) {
	b.Helper()

	deps := benchmarkDependencies(size)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		resolved, err := ResolveTransitive([]string{"module-0"}, deps)
		if err != nil {
			b.Fatalf("resolve transitive: %v", err)
		}

		if len(resolved) != size-benchmarkOffset {
			b.Fatalf("resolved count = %d, want %d", len(resolved), size-benchmarkOffset)
		}
	}
}

func benchmarkDependencies(size int) map[string][]string {
	deps := make(map[string][]string, size)

	for index := range size {
		module := fmt.Sprintf(benchmarkModuleFmt, index)

		deps[module] = nil

		if index+benchmarkOffset < size {
			deps[module] = []string{fmt.Sprintf(benchmarkModuleFmt, index+benchmarkOffset)}
		}
	}

	return deps
}

// BenchmarkBuildDestinationMap measures normalization across several catalog sizes.
func BenchmarkBuildDestinationMap(b *testing.B) {
	benchmarkSizes(b, "sources", runDestinationMapBenchmark)
}

func benchmarkSizes(b *testing.B, label string, run func(*testing.B, int)) {
	b.Helper()

	for _, size := range []int{benchmarkSmallSize, benchmarkMediumSize, benchmarkLargeSize} {
		b.Run(fmt.Sprintf("%s_%d", label, size), func(b *testing.B) { run(b, size) })
	}
}

func runDestinationMapBenchmark(b *testing.B, size int) {
	b.Helper()

	sources := make([]string, 0, size)

	for index := range sources {
		sources = append(sources, fmt.Sprintf("tool-%d/node/pnpm", index))
	}

	b.ReportAllocs()
	b.ResetTimer()
	runDestinationMapIterations(b, sources, size)
}

func runDestinationMapIterations(b *testing.B, sources []string, size int) {
	b.Helper()

	for range b.N {
		result, err := BuildDestinationMap(sources)
		if err != nil {
			b.Fatalf("build destination map: %v", err)
		}

		if len(result) != size {
			b.Fatalf("destination count = %d, want %d", len(result), size)
		}
	}
}
