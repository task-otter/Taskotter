// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"strconv"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	benchCatalogModules = 120
	benchRequestedTasks = 24
	benchDepsFanOut     = 4
	benchModulePrefix   = "module-"
	benchUnknownTask    = "eslnt-node-pnmp"
	benchUnknownWant    = "expected unknown task to fail"
)

func benchModuleName(idx int) string {
	return benchModulePrefix + strconv.Itoa(idx)
}

// benchCatalog builds a store catalog with plain modules plus one Node variant chain.
func benchCatalog() map[string]struct{} {
	catalog := make(map[string]struct{}, benchCatalogModules)

	for idx := range benchCatalogModules {
		catalog[benchModuleName(idx)] = struct{}{}
	}

	catalog[srcESLintPnpm] = struct{}{}

	return catalog
}

func benchModuleNames(count int) []string {
	names := make([]string, consts.IndexZero, count)

	for idx := range count {
		names = append(names, benchModuleName(idx))
	}

	return names
}

func benchTasks() []string {
	return append(benchModuleNames(benchRequestedTasks), taskESLint)
}

func benchDependenciesOf(idx int) []string {
	out := make([]string, consts.IndexZero, benchDepsFanOut)

	for step := consts.IndexOne; step <= benchDepsFanOut; step++ {
		next := idx + step

		if next < benchCatalogModules {
			out = append(out, benchModuleName(next))
		}
	}

	return out
}

// benchDeps builds a dependency graph where every module depends on later modules.
func benchDeps() map[string][]string {
	deps := make(map[string][]string, benchCatalogModules)

	for idx := range benchCatalogModules {
		deps[benchModuleName(idx)] = benchDependenciesOf(idx)
	}

	return deps
}

// BenchmarkResolveAll measures resolving a full task list against a store catalog.
func BenchmarkResolveAll(b *testing.B) {
	input := &service.ResolveAllInput{
		Catalog:        benchCatalog(),
		PackageManager: config.PMPnpm,
		Tasks:          benchTasks(),
	}

	for b.Loop() {
		out, err := service.ResolveAll(input)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(out)
	}
}

// BenchmarkResolveUnknownTask measures the close-match suggestion path of a failed resolution.
func BenchmarkResolveUnknownTask(b *testing.B) {
	input := &service.ResolveInput{
		Task:           benchUnknownTask,
		Catalog:        benchCatalog(),
		PackageManager: config.PMPnpm,
	}

	for b.Loop() {
		res, err := service.Resolve(input)
		if err == nil {
			b.Fatal(benchUnknownWant)
		}

		iox.Discard2(res, err)
	}
}

// BenchmarkResolveTransitive measures transitive dependency resolution on a dense graph.
func BenchmarkResolveTransitive(b *testing.B) {
	sources := benchModuleNames(benchRequestedTasks)
	deps := benchDeps()

	for b.Loop() {
		out, err := service.ResolveTransitive(sources, deps)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(out)
	}
}

// BenchmarkBuildDestinationMap measures destination normalization for resolved modules.
func BenchmarkBuildDestinationMap(b *testing.B) {
	sources := append(benchModuleNames(benchRequestedTasks), srcESLintPnpm)

	for b.Loop() {
		out, err := service.BuildDestinationMap(sources)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(out)
	}
}

func BenchmarkLevenshtein(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = service.Levenshtein("eslint/node/pnpm", "eslint/bun")
	}
}
