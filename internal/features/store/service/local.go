// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package service loads local store snapshots and catalog metadata.
package service

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

type (
	catalogWalk struct {
		catalog map[string]struct{}
		dir     string
		prefix  string
		entries []os.DirEntry
	}

	localSnapshotArgs struct {
		ref     *domain.RefInfo
		catalog map[string]struct{}
		deps    map[string][]string
		root    string
	}
)

const (
	fmtLoadCatalogErr = "load catalog: %w"
	fmtLoadDepsErr    = "load deps: %w"
)

var (
	errDepsMissingModule     = errors.New(".deps.yml references missing module")
	errDepsMissingDependency = errors.New(".deps.yml references missing dependency")
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
	catalog := make(map[string]struct{})

	err := collectModules(filepath.Join(root, "taskfiles"), consts.Empty, catalog)
	if err != nil {
		return nil, fmt.Errorf("collect modules: %w", err)
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

// collectModules walks the store taskfiles tree and records module names. A
// directory with a Taskfile.yml is a module, and its subdirectories are its
// variants. A directory without a Taskfile.yml is not a module and its
// subdirectories are not visited, so a Taskfile-less directory can no longer
// act as a namespace prefix.
func collectModules(dir, prefix string, catalog map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("load module catalog: %w", err)
	}

	isRoot := prefix == consts.Empty

	if !isModuleDir(entries) && !isRoot {
		return nil
	}

	if !isRoot {
		catalog[prefix] = struct{}{}
	}

	err = visitChildDirs(&catalogWalk{
		dir:     dir,
		prefix:  prefix,
		entries: entries,
		catalog: catalog,
	})
	if err != nil {
		return fmt.Errorf("visit child directories: %w", err)
	}

	return nil
}

func isModuleDir(entries []os.DirEntry) bool {
	return slices.ContainsFunc(entries, isTaskfileEntry)
}

func isTaskfileEntry(entry os.DirEntry) bool {
	return !entry.IsDir() && entry.Name() == "Taskfile.yml"
}

func visitChildDirs(walk *catalogWalk) error {
	for i := range walk.entries {
		entry := walk.entries[i]

		if !entry.IsDir() {
			continue
		}

		name := path.Join(walk.prefix, entry.Name())
		child := filepath.Join(walk.dir, entry.Name())

		err := collectModules(child, name, walk.catalog)
		if err != nil {
			return fmt.Errorf("collect modules under %q: %w", name, err)
		}
	}

	return nil
}
