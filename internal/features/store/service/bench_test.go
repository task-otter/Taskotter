// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const (
	benchmarkSmallSize    = 10
	benchmarkMediumSize   = 100
	benchmarkLargeSize    = 1000
	benchmarkModuleFmt    = "module-%d"
	benchmarkTaskfilesDir = "taskfiles"
	benchmarkTaskfile     = "Taskfile.yml"
	benchmarkDepsFile     = ".deps.yml"
	benchmarkDirMode      = 0o750
	benchmarkFileMode     = 0o600
	benchmarkEmptyLength  = 0
)

// BenchmarkLoadCatalogAndDeps measures store discovery and dependency loading.
func BenchmarkLoadCatalogAndDeps(b *testing.B) {
	sizes := []int{benchmarkSmallSize, benchmarkMediumSize, benchmarkLargeSize}

	for index := range sizes {
		size := sizes[index]
		b.Run(fmt.Sprintf("modules_%d", size), func(b *testing.B) { runStoreBenchmark(b, size) })
	}
}

func runStoreBenchmark(b *testing.B, size int) {
	root := benchmarkStore(b, size)
	b.ResetTimer()

	for range b.N {
		benchmarkStoreLoad(b, root, size)
	}
}

func benchmarkStore(b *testing.B, size int) string {
	b.Helper()

	root := b.TempDir()

	err := os.MkdirAll(filepath.Join(root, benchmarkTaskfilesDir), benchmarkDirMode)
	if err != nil {
		b.Fatal(err)
	}

	deps := benchmarkStoreModules(b, root, size)

	err = os.WriteFile(filepath.Join(root, benchmarkDepsFile), deps, benchmarkFileMode)
	if err != nil {
		b.Fatal(err)
	}

	return root
}

func benchmarkStoreModules(b *testing.B, root string, size int) []byte {
	b.Helper()

	deps := make([]byte, benchmarkEmptyLength, size*24)

	for index := range size {
		module := fmt.Sprintf(benchmarkModuleFmt, index)
		benchmarkStoreModule(b, root, module)

		deps = append(deps, []byte(module+": []\n")...)
	}

	return deps
}

func benchmarkStoreModule(b *testing.B, root, module string) {
	b.Helper()

	dir := filepath.Join(root, benchmarkTaskfilesDir, module)

	err := os.MkdirAll(dir, benchmarkDirMode)
	if err != nil {
		b.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(dir, benchmarkTaskfile),
		[]byte("version: \"3\"\n"),
		benchmarkFileMode,
	)
	if err != nil {
		b.Fatal(err)
	}
}

func benchmarkStoreLoad(b *testing.B, root string, size int) {
	b.Helper()

	catalog, loaded, err := LoadCatalogAndDeps(root)
	if err != nil {
		b.Fatalf("load catalog and deps: %v", err)
	}

	if len(catalog) != size || len(loaded) != size {
		b.Fatalf("loaded catalog/deps = %d/%d, want %d/%d", len(catalog), len(loaded), size, size)
	}
}
