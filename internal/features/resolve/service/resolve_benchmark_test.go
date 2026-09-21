// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

func BenchmarkResolveTransitive(b *testing.B) {
	d := deps()
	sources := []string{srcESLintPnpm, destPnpm, consts.Go}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.ResolveTransitive(sources, d)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildDestinationMap(b *testing.B) {
	sources := []string{srcESLintPnpm, consts.Go}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.BuildDestinationMap(sources)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolveAll(b *testing.B) {
	input := &service.ResolveAllInput{
		Tasks:          []string{consts.Go, taskPrettier},
		Catalog:        catalog(consts.Go, taskPrettier),
		PackageManager: consts.Empty,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := service.ResolveAll(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLevenshtein(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = service.Levenshtein("eslint/node/pnpm", "eslint/bun")
	}
}
