// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package catalog

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
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

	if prefix != "" {
		if !isModule(entries) {
			return nil
		}

		modules[prefix] = struct{}{}
	}

	for index := range entries {
		entry := entries[index]

		if !entry.IsDir() {
			continue
		}

		name := path.Join(prefix, entry.Name())

		err = collect(filepath.Join(dir, entry.Name()), name, modules)
		if err != nil {
			return fmt.Errorf("collect modules under %q: %w", name, err)
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
