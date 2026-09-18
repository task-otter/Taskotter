// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/store/domain"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	taskfilesDir = "taskfiles"
	depsFile     = ".deps.yml"
	moduleGo     = "go"
	wantErrText  = "expected error"
	taskfileBody = "version: \"3\"\n"
	depsGoEmpty  = "go: []\n"
)

// TestLocalSnapshotLoadsFixtureStore verifies a snapshot is built from an on-disk store.
func TestLocalSnapshotLoadsFixtureStore(t *testing.T) {
	t.Parallel()

	snapshot, err := storesvc.LocalSnapshot(fixtureStoreRoot, fixtureRefInfo())
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if domain.WorkspaceRoot(snapshot) != fixtureStoreRoot {
		t.Fatalf("WorkspaceRoot() = %q", domain.WorkspaceRoot(snapshot))
	}
}

// TestLocalSnapshotReportsMissingStore verifies a root without a taskfiles tree fails.
func TestLocalSnapshotReportsMissingStore(t *testing.T) {
	t.Parallel()

	snapshot, err := storesvc.LocalSnapshot(t.TempDir(), fixtureRefInfo())
	iox.Discard(snapshot)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestLoadCatalogAndDepsReportsMissingDepsFile verifies a store without .deps.yml fails.
func TestLoadCatalogAndDepsReportsMissingDepsFile(t *testing.T) {
	t.Parallel()
	assertCatalogLoadFails(t, newStore(t, consts.Empty))
}

// TestLoadCatalogAndDepsReportsInvalidDepsFile verifies malformed YAML is reported.
func TestLoadCatalogAndDepsReportsInvalidDepsFile(t *testing.T) {
	t.Parallel()
	assertCatalogLoadFails(t, newStore(t, "not: [valid"))
}

// TestLoadCatalogAndDepsReportsUnknownModule verifies deps for unknown modules are rejected.
func TestLoadCatalogAndDepsReportsUnknownModule(t *testing.T) {
	t.Parallel()
	assertCatalogLoadFails(t, newStore(t, "missing:\n  - go\n"))
}

// TestLoadCatalogAndDepsReportsUnknownDependency verifies unknown dependencies are rejected.
func TestLoadCatalogAndDepsReportsUnknownDependency(t *testing.T) {
	t.Parallel()
	assertCatalogLoadFails(t, newStore(t, "go:\n  - missing\n"))
}

// TestLoadCatalogAndDepsAcceptsValidDeps verifies a consistent store loads cleanly.
func TestLoadCatalogAndDepsAcceptsValidDeps(t *testing.T) {
	t.Parallel()

	catalog, deps, err := storesvc.LoadCatalogAndDeps(newStore(t, depsGoEmpty))
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if _, ok := catalog[moduleGo]; !ok {
		t.Fatalf("catalog = %#v", catalog)
	}

	if len(deps[moduleGo]) != noDependencies {
		t.Fatalf("deps = %#v", deps)
	}
}

// TestLoadCatalogAndDepsReportsUnreadableChildDir verifies unreadable subdirectories fail.
func TestLoadCatalogAndDepsReportsUnreadableChildDir(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == consts.IndexZero {
		t.Skip("running as root: permission errors are not reproducible")
	}

	root := newStore(t, depsGoEmpty)
	blocked := filepath.Join(root, taskfilesDir, moduleGo, "blocked")

	mkdirOrFail(t, blocked)
	chmodOrFail(t, blocked, consts.IndexZero)
	t.Cleanup(func() { iox.Discard(os.Chmod(blocked, consts.FilePerm755)) })

	assertCatalogLoadFails(t, root)
}

func assertCatalogLoadFails(t *testing.T, root string) {
	t.Helper()

	catalog, deps, err := storesvc.LoadCatalogAndDeps(root)
	iox.Discard2(catalog, deps)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func chmodOrFail(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	err := os.Chmod(path, mode)
	if err != nil {
		t.Fatal(err)
	}
}

func fixtureRefInfo() *domain.RefInfo {
	return &domain.RefInfo{
		Repository:       consts.Empty,
		RequestedVersion: consts.Empty,
		SourceRef:        "refs/heads/main",
		ResolvedCommit:   "abc",
		DefaultBranch:    "main",
	}
}

func mkdirOrFail(t *testing.T, dir string) {
	t.Helper()

	err := os.MkdirAll(dir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}
}

// newStore writes a minimal store tree with a single "go" module and returns its root.
// An empty depsYAML omits the .deps.yml file entirely.
func newStore(t *testing.T, depsYAML string) string {
	t.Helper()

	root := t.TempDir()
	moduleDir := filepath.Join(root, taskfilesDir, moduleGo)

	mkdirOrFail(t, moduleDir)
	writeFileOrFail(t, filepath.Join(moduleDir, consts.Taskfile), taskfileBody)

	if depsYAML != consts.Empty {
		writeFileOrFail(t, filepath.Join(root, depsFile), depsYAML)
	}

	return root
}

func writeFileOrFail(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}
}

func loadFixtureCatalog(t *testing.T) fixtureStore {
	t.Helper()

	catalog, deps, err := storesvc.LoadCatalogAndDeps(fixtureStoreRoot)
	if err != nil {
		t.Fatalf("load catalog and deps: %v", err)
	}

	return fixtureStore{catalog: catalog, deps: deps}
}

func resolveFixtureESLintSource(t *testing.T) string {
	t.Helper()

	res, err := resolvesvc.Resolve(&resolvesvc.ResolveInput{
		Task:           moduleESLint,
		Catalog:        loadFixtureCatalog(t).catalog,
		PackageManager: config.PMPnpm,
	})
	if err != nil {
		t.Fatalf("resolve %q: %v", moduleESLint, err)
	}

	return res.SourceModule
}

// TestCatalogContainsFlattenedVariants verifies the store's flattened node variant
// modules and its flat package-manager modules are cataloged.
func TestCatalogContainsFlattenedVariants(t *testing.T) {
	t.Parallel()

	catalog := loadFixtureCatalog(t).catalog

	wants := []string{moduleESLint, moduleESLintNodePnpm, "eslint/node", "pnpm", "go"}

	for i := range wants {
		if _, ok := catalog[wants[i]]; !ok {
			t.Errorf("missing module %q in catalog %#v", wants[i], catalog)
		}
	}
}

// TestCatalogSkipsTaskfilelessDirectories verifies a directory without a Taskfile.yml
// is neither cataloged nor descended into, so it cannot act as a namespace prefix.
func TestCatalogSkipsTaskfilelessDirectories(t *testing.T) {
	t.Parallel()

	catalog := loadFixtureCatalog(t).catalog

	for module := range catalog {
		if strings.HasPrefix(module, "go/docs") {
			t.Errorf("Taskfile-less directory cataloged as module: %q", module)
		}
	}
}

// TestDepsValidateAgainstCatalog verifies every .deps.yml entry resolves against the catalog.
func TestDepsValidateAgainstCatalog(t *testing.T) {
	t.Parallel()

	deps := loadFixtureCatalog(t).deps

	if len(deps[moduleESLintNodePnpm]) == noDependencies {
		t.Fatalf("expected dependencies for %q, got %#v", moduleESLintNodePnpm, deps)
	}
}

// TestResolveFlattenedNodeVariant verifies a logical task resolves to the flattened
// store module and normalizes back to the bare destination folder.
func TestResolveFlattenedNodeVariant(t *testing.T) {
	t.Parallel()

	source := resolveFixtureESLintSource(t)

	if source != moduleESLintNodePnpm {
		t.Fatalf("SourceModule = %q, want %q", source, moduleESLintNodePnpm)
	}

	dest, err := resolvesvc.Normalize(source)
	if err != nil {
		t.Fatalf("normalize %q: %v", source, err)
	}

	if dest != moduleESLint {
		t.Fatalf("destination = %q, want %q", dest, moduleESLint)
	}
}
