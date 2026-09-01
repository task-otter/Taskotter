// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"strings"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	// fixtureStore holds the catalog and dependency graph of the fixture store.
	fixtureStore struct {
		catalog map[string]struct{}
		deps    map[string][]string
	}
)

const (
	fixtureStoreRoot = "../../../../tests/fixtures/store"

	moduleESLintNodePnpm = "eslint/node/pnpm"

	moduleESLint = "eslint"

	noDependencies = 0
)

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
