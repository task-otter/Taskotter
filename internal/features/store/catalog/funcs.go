// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package catalog

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
)

type (
	collectChildrenParams struct {
		modules map[string]struct{}
		dir     string
		prefix  string
		entries []os.DirEntry
	}
)

// Discover returns modules whose directories contain their own Taskfile.yml.
// Directories without a Taskfile are intentionally not traversed.
func Discover(root string) (map[string]struct{}, error) {
	modules := make(map[string]struct{})

	err := collect(filepath.Join(root, taskfilesDir), "", modules)
	if err != nil {
		return nil, fmt.Errorf("collect modules: %w", err)
	}

	return modules, nil
}

func collect(dir, prefix string, modules map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("load module catalog: %w", err)
	}

	if shouldRegister(prefix, entries) {
		modules[prefix] = struct{}{}
	}

	return collectChildren(&collectChildrenParams{
		dir: dir, prefix: prefix, entries: entries, modules: modules,
	})
}

func shouldRegister(prefix string, entries []os.DirEntry) bool {
	return prefix != "" && isModule(entries)
}

func collectChildren(params *collectChildrenParams) error {
	for index := range params.entries {
		entry := params.entries[index]

		if !entry.IsDir() {
			continue
		}

		name := path.Join(params.prefix, entry.Name())

		childErr := collect(filepath.Join(params.dir, entry.Name()), name, params.modules)
		if childErr != nil {
			return fmt.Errorf("collect modules under %q: %w", name, childErr)
		}
	}

	return nil
}

func isModule(entries []os.DirEntry) bool {
	for index := range entries {
		if !entries[index].IsDir() && entries[index].Name() == "Taskfile.yml" {
			return true
		}
	}

	return false
}
