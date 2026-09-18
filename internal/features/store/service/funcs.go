// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"fmt"

	storecatalog "github.com/task-otter/Taskotter/internal/features/store/catalog"
	"github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

// LocalSnapshot loads a store snapshot from an on-disk store root.
func LocalSnapshot(root string, ref *domain.RefInfo) (*domain.Snapshot, error) {
	catalog, deps, err := LoadCatalogAndDeps(root)
	if err != nil {
		return nil, fmt.Errorf("load local snapshot: %w", err)
	}

	return newLocalSnapshot(&localSnapshotArgs{
		root:    root,
		ref:     ref,
		catalog: catalog,
		deps:    deps,
	}), nil
}

func newLocalSnapshot(args *localSnapshotArgs) *domain.Snapshot {
	return &domain.Snapshot{
		RootDir: args.root,
		Catalog: args.catalog,
		Deps:    args.deps,
		Ref:     *args.ref,
		Cleanup: nil,
	}
}

// LoadCatalogAndDeps loads the module catalog and dependency map from a store root.
func LoadCatalogAndDeps(
	root string,
) (catalog map[string]struct{}, deps map[string][]string, err error) {
	catalog, err = loadCatalog(root)
	if err != nil {
		return nil, nil, fmt.Errorf(fmtLoadCatalogErr, err)
	}

	deps, err = loadDeps(root, catalog)
	if err != nil {
		return nil, nil, fmt.Errorf(fmtLoadDepsErr, err)
	}

	return catalog, deps, nil
}

func loadCatalog(root string) (map[string]struct{}, error) {
	catalog, err := storecatalog.Discover(root)
	if err != nil {
		return nil, fmt.Errorf("discover store catalog: %w", err)
	}

	return catalog, nil
}

func loadDeps(root string, catalog map[string]struct{}) (map[string][]string, error) {
	raw, err := parseDepsFile(root)
	if err != nil {
		return nil, fmt.Errorf("parse deps file: %w", err)
	}

	err = validateDeps(raw, catalog)
	if err != nil {
		return nil, fmt.Errorf("validate deps: %w", err)
	}

	return raw, nil
}

func parseDepsFile(root string) (map[string][]string, error) {
	data, err := pathutil.ReadRelativeFile(root, ".deps.yml")
	if err != nil {
		return nil, fmt.Errorf("read .deps.yml: %w", err)
	}

	var raw map[string][]string

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("parse .deps.yml: %w", err)
	}

	return raw, nil
}

func validateDeps(raw map[string][]string, catalog map[string]struct{}) error {
	for module := range raw {
		err := validateModuleDeps(module, raw[module], catalog)
		if err != nil {
			return fmt.Errorf("validate module deps for %q: %w", module, err)
		}
	}

	return nil
}

func validateModuleDeps(module string, deps []string, catalog map[string]struct{}) error {
	if _, ok := catalog[module]; !ok {
		return fmt.Errorf("%w %q", errDepsMissingModule, module)
	}

	for i := range deps {
		dep := deps[i]

		if _, ok := catalog[dep]; !ok {
			return fmt.Errorf("%w %q for module %q", errDepsMissingDependency, dep, module)
		}
	}

	return nil
}
