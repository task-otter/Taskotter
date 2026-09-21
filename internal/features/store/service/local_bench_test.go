// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	// benchFileSpec describes one file written into the synthetic store tree.
	benchFileSpec struct {
		dir  string
		name string
		body string
	}
)

const (
	benchStoreModules  = 60
	benchStoreVariants = 3
	benchModulePrefix  = "mod"
	benchVariantPrefix = "v"
	benchDirPerm       = 0o750
	benchDepsHeader    = "---\n"
	benchDepsEntryFmt  = "%s: [%s]\n"
	benchReadmeBody    = "# module\n"
	benchMetadataBody  = "schema: taskotter.dev/taskfile-metadata/v1\n"
	benchTaskfileBody  = "version: \"3\"\nvars:\n  TOOL_VERSION: \"1.0.0\"\n" +
		"tasks:\n  lint:\n    cmds:\n      - echo lint\n"
)

func benchModuleName(idx int) string {
	return benchModulePrefix + strconv.Itoa(idx)
}

func benchWriteStoreFile(b *testing.B, spec *benchFileSpec) {
	b.Helper()

	err := os.MkdirAll(spec.dir, benchDirPerm)
	if err != nil {
		b.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(spec.dir, spec.name), []byte(spec.body), consts.FilePerm644)
	if err != nil {
		b.Fatal(err)
	}
}

func benchWriteStoreModule(b *testing.B, dir string) {
	b.Helper()

	benchWriteStoreFile(b, &benchFileSpec{dir: dir, name: consts.Taskfile, body: benchTaskfileBody})
	benchWriteStoreFile(b, &benchFileSpec{dir: dir, name: consts.Metadata, body: benchMetadataBody})
	benchWriteStoreFile(b, &benchFileSpec{dir: dir, name: consts.ReadmeMD, body: benchReadmeBody})
}

func benchWriteStoreVariants(b *testing.B, moduleDir string) {
	b.Helper()

	for variant := range benchStoreVariants {
		name := benchVariantPrefix + strconv.Itoa(variant)

		benchWriteStoreModule(b, filepath.Join(moduleDir, name))
	}
}

func benchDepsDocument() string {
	var doc strings.Builder

	iox.Discard(iox.WriteStringFull(&doc, benchDepsHeader))

	for idx := range benchStoreModules {
		name := benchModuleName(idx)
		dep := benchModuleName((idx + consts.IndexOne) % benchStoreModules)

		iox.Discard(iox.Fprintf(&doc, benchDepsEntryFmt, name, dep))
	}

	return doc.String()
}

// benchStoreRoot writes a synthetic store tree with modules, variants, and a deps file.
func benchStoreRoot(b *testing.B) string {
	b.Helper()

	root := b.TempDir()
	taskfiles := filepath.Join(root, taskfilesDir)

	for idx := range benchStoreModules {
		moduleDir := filepath.Join(taskfiles, benchModuleName(idx))

		benchWriteStoreModule(b, moduleDir)
		benchWriteStoreVariants(b, moduleDir)
	}

	deps := &benchFileSpec{dir: root, name: depsFile, body: benchDepsDocument()}

	benchWriteStoreFile(b, deps)

	return root
}

// BenchmarkLoadCatalogAndDeps measures walking a store tree and parsing its dependency graph.
func BenchmarkLoadCatalogAndDeps(b *testing.B) {
	root := benchStoreRoot(b)

	for b.Loop() {
		//nolint:unqueryvet // LoadCatalogAndDeps reads the store from disk, not a database
		catalog, deps, err := storesvc.LoadCatalogAndDeps(root)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard2(catalog, deps)
	}
}
