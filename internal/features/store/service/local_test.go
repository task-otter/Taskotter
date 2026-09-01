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

const (
	fixtureStoreRoot = "../../../../tests/fixtures/store"

	moduleESLintNodePnpm = "eslint/node/pnpm"

	moduleESLint = "eslint"
)

func loadFixtureCatalog(t *testing.T) (map[string]struct{}, map[string][]string) {
	t.Helper()

	catalog, deps, err := storesvc.LoadCatalogAndDeps(fixtureStoreRoot)
	if err != nil {
		t.Fatalf("load catalog and deps: %v", err)
	}

	return catalog, deps
}

// TestCatalogContainsFlattenedVariants verifies the store's flattened node variant
// modules and its flat package-manager modules are cataloged.
func TestCatalogContainsFlattenedVariants(t *testing.T) {
	t.Parallel()

	catalog, _ := loadFixtureCatalog(t)

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

	catalog, _ := loadFixtureCatalog(t)

	for module := range catalog {
		if strings.HasPrefix(module, "go/docs") {
			t.Errorf("Taskfile-less directory cataloged as module: %q", module)
		}
	}
}

// TestDepsValidateAgainstCatalog verifies every .deps.yml entry resolves against the catalog.
func TestDepsValidateAgainstCatalog(t *testing.T) {
	t.Parallel()

	_, deps := loadFixtureCatalog(t)

	if len(deps[moduleESLintNodePnpm]) == 0 {
		t.Fatalf("expected dependencies for %q, got %#v", moduleESLintNodePnpm, deps)
	}
}

// TestResolveFlattenedNodeVariant verifies a logical task resolves to the flattened
// store module and normalizes back to the bare destination folder.
func TestResolveFlattenedNodeVariant(t *testing.T) {
	t.Parallel()

	catalog, _ := loadFixtureCatalog(t)

	res, err := resolvesvc.Resolve(&resolvesvc.ResolveInput{
		Task:           moduleESLint,
		Catalog:        catalog,
		PackageManager: config.PMPnpm,
	})
	if err != nil {
		t.Fatalf("resolve %q: %v", moduleESLint, err)
	}

	if res.SourceModule != moduleESLintNodePnpm {
		t.Fatalf("SourceModule = %q, want %q", res.SourceModule, moduleESLintNodePnpm)
	}

	dest, err := resolvesvc.Normalize(res.SourceModule)
	if err != nil {
		t.Fatalf("normalize %q: %v", res.SourceModule, err)
	}

	if dest != moduleESLint {
		t.Fatalf("destination = %q, want %q", dest, moduleESLint)
	}
}
