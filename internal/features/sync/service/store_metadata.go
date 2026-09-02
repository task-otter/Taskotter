// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

type (
	storeTaskMetadata struct {
		Schema        string
		Module        string
		Taskfile      string
		ExportedTasks []string
		Variants      []string
	}
)

const (
	storeMetadataFileName = "metadata.yml"

	// storeMetadataSchema is the only metadata.yml schema this version understands.
	storeMetadataSchema = "taskotter.dev/taskfile-metadata/v1"
)

var errUnsupportedStoreMetadataSchema = errors.New("unsupported metadata.yml schema")

func appendUniqueExportedTask(out []string, seen map[string]struct{}, task string) []string {
	if task == consts.Empty {
		return out
	}

	if _, ok := seen[task]; ok {
		return out
	}

	seen[task] = struct{}{}

	return append(out, task)
}

func climbToParentModule(current string) (string, bool) {
	parent := path.Dir(current)

	if parent == "." || parent == current {
		return consts.Empty, false
	}

	return parent, true
}

func commonStoreTaskNames(metadata map[string]storeTaskMetadata) map[string]struct{} {
	counts := countExportedTasksPerModule(metadata)
	common := make(map[string]struct{})

	for task := range counts {
		if counts[task] >= domain.YAMLMappingPairKeyValue {
			common[task] = struct{}{}
		}
	}

	return common
}

func countExportedTasksPerModule(metadata map[string]storeTaskMetadata) map[string]int {
	counts := make(map[string]int)

	for key := range metadata {
		meta := metadata[key]
		tasks := dedupExportedTasks(meta.ExportedTasks)

		for i := range tasks {
			counts[tasks[i]]++
		}
	}

	return counts
}

func dedupExportedTasks(exportedTasks []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, consts.IndexZero, len(exportedTasks))

	for i := range exportedTasks {
		out = appendUniqueExportedTask(out, seen, exportedTasks[i])
	}

	return out
}

func emptyStoreTaskMetadata() storeTaskMetadata {
	return storeTaskMetadata{
		Schema:        consts.Empty,
		Module:        consts.Empty,
		Taskfile:      consts.Empty,
		ExportedTasks: nil,
		Variants:      nil,
	}
}

func loadOneStoreMetadataFile(root, abs string, out map[string]storeTaskMetadata) error {
	module, err := moduleNameFor(root, abs)
	if err != nil {
		return fmt.Errorf("module name for %q: %w", abs, err)
	}

	meta, err := readStoreTaskMetadata(abs, module)
	if err != nil {
		return fmt.Errorf("read store task metadata %q: %w", abs, err)
	}

	out[module] = meta

	return nil
}

func loadStoreTaskMetadata(snapshot ports.Snapshot) (map[string]storeTaskMetadata, error) {
	out := make(map[string]storeTaskMetadata)
	root := filepath.Join(snapshot.WorkspaceRoot(), taskfilesDirName)

	err := walkDir(root, storeMetadataWalker(root, out))
	if err != nil {
		return nil, fmt.Errorf("load store metadata: %w", err)
	}

	return out, nil
}

func moduleNameFor(root, abs string) (string, error) {
	relDir, err := relPath(root, filepath.Dir(abs))
	if err != nil {
		return consts.Empty, fmt.Errorf("metadata module path for %q: %w", abs, err)
	}

	return filepath.ToSlash(relDir), nil
}

func readStoreTaskMetadata(abs, module string) (storeTaskMetadata, error) {
	data, err := pathutil.ReadRelativeFile(filepath.Dir(abs), storeMetadataFileName)
	if err != nil {
		return emptyStoreTaskMetadata(), fmt.Errorf("read metadata for %q: %w", module, err)
	}

	meta, err := parseStoreTaskMetadata(data, module)
	if err != nil {
		return emptyStoreTaskMetadata(), fmt.Errorf("metadata for %q: %w", module, err)
	}

	return meta, nil
}

func parseStoreTaskMetadata(data []byte, module string) (storeTaskMetadata, error) {
	var meta storeTaskMetadata

	err := yaml.Unmarshal(data, &meta)
	if err != nil {
		return emptyStoreTaskMetadata(), fmt.Errorf("parse metadata: %w", err)
	}

	if meta.Schema != storeMetadataSchema {
		return emptyStoreTaskMetadata(), fmt.Errorf(
			"%w %q in %q: expected %q",
			errUnsupportedStoreMetadataSchema, meta.Schema, module, storeMetadataSchema,
		)
	}

	if meta.Module == consts.Empty {
		meta.Module = module
	}

	return meta, nil
}

func resolveStoreTaskMetadata(src string, meta storeTaskMetaMap) (storeTaskMetadata, bool) {
	current := src

	for current != consts.Empty {
		if meta, foundMeta := meta[current]; foundMeta {
			return meta, true
		}

		parent, ok := climbToParentModule(current)

		if !ok {
			break
		}

		current = parent
	}

	return emptyStoreTaskMetadata(), false
}

func storeMetadataWalker(root string, out map[string]storeTaskMetadata) fs.WalkDirFunc {
	return func(abs string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf(errWalk, abs, walkErr)
		}

		if entry.IsDir() || entry.Name() != storeMetadataFileName {
			return nil
		}

		return loadOneStoreMetadataFile(root, abs, out)
	}
}

func (meta *storeTaskMetadata) UnmarshalYAML(value *yaml.Node) error {
	err := domain.UnmarshalYAMLMapping(value, "store task metadata", map[string]any{
		domain.YAMLKeySchema:        &meta.Schema,
		domain.YAMLKeyModule:        &meta.Module,
		domain.YAMLKeyTaskfile:      &meta.Taskfile,
		domain.YAMLKeyExportedTasks: &meta.ExportedTasks,
		domain.YAMLKeyVariants:      &meta.Variants,
	})
	if err != nil {
		return fmt.Errorf("unmarshal store task metadata: %w", err)
	}

	return nil
}
