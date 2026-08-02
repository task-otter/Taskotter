// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"

	"github.com/task-otter/Taskotter/internal/pathutil"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/taskfile"
	yaml "go.yaml.in/yaml/v3"
)

const (
	storeMetadataFileName          = "metadata.yml"
	minModulesForGeneratedRootTask = 2
)

type storeTaskMetadata struct {
	Schema        string
	Module        string
	Taskfile      string
	ExportedTasks []string
	Variants      []string
}

func (meta *storeTaskMetadata) UnmarshalYAML(value *yaml.Node) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode store task metadata: %w", err)
	}

	return decodeYAMLFields(
		fields,
		yamlDecodeTarget{key: yamlKeySchema, out: &meta.Schema},
		yamlDecodeTarget{key: yamlKeyModule, out: &meta.Module},
		yamlDecodeTarget{key: yamlKeyTaskfile, out: &meta.Taskfile},
		yamlDecodeTarget{key: yamlKeyExportedTasks, out: &meta.ExportedTasks},
		yamlDecodeTarget{key: yamlKeyVariants, out: &meta.Variants},
	)
}

func loadStoreTaskMetadata(snapshot *store.Snapshot) (map[string]storeTaskMetadata, error) {
	out := make(map[string]storeTaskMetadata)
	root := filepath.Join(snapshot.RootDir, "taskfiles")

	err := filepath.WalkDir(root, func(abs string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", abs, walkErr)
		}

		if entry.IsDir() || entry.Name() != storeMetadataFileName {
			return nil
		}

		relDir, err := filepath.Rel(root, filepath.Dir(abs))
		if err != nil {
			return fmt.Errorf("metadata module path for %q: %w", abs, err)
		}

		module := filepath.ToSlash(relDir)

		data, err := pathutil.ReadRelativeFile(filepath.Dir(abs), storeMetadataFileName)
		if err != nil {
			return fmt.Errorf("read metadata for %q: %w", module, err)
		}

		var meta storeTaskMetadata

		err = yaml.Unmarshal(data, &meta)
		if err != nil {
			return fmt.Errorf("parse metadata for %q: %w", module, err)
		}

		if meta.Module == "" {
			meta.Module = module
		}

		out[module] = meta

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load store metadata: %w", err)
	}

	return out, nil
}

func resolveStoreTaskMetadata(
	sourceModule string,
	metadata map[string]storeTaskMetadata,
) (storeTaskMetadata, bool) {
	for current := sourceModule; current != ""; {
		if meta, foundMeta := metadata[current]; foundMeta {
			return meta, true
		}

		parent := path.Dir(current)

		if parent == "." || parent == current {
			break
		}

		current = parent
	}

	return storeTaskMetadata{
		Schema:        "",
		Module:        "",
		Taskfile:      "",
		ExportedTasks: nil,
		Variants:      nil,
	}, false
}

func commonStoreTaskNames(metadata map[string]storeTaskMetadata) map[string]struct{} {
	counts := make(map[string]int)

	for _, meta := range metadata {
		seen := make(map[string]struct{})

		for _, task := range meta.ExportedTasks {
			if task == "" {
				continue
			}

			seen[task] = struct{}{}
		}

		for task := range seen {
			counts[task]++
		}
	}

	common := make(map[string]struct{})

	for task, count := range counts {
		if count >= minModulesForGeneratedRootTask {
			common[task] = struct{}{}
		}
	}

	return common
}

func buildGeneratedRootTasks(
	requested []string,
	requestedRecords map[string]ModuleRecord,
	metadata map[string]storeTaskMetadata,
) []taskfile.GeneratedRootTask {
	common := commonStoreTaskNames(metadata)
	modulesByTask := make(map[string][]string)

	for _, logicalTask := range requested {
		record, foundRecord := requestedRecords[logicalTask]

		if !foundRecord {
			continue
		}

		meta, ok := resolveStoreTaskMetadata(record.SourceModule, metadata)

		if !ok {
			continue
		}

		for _, exported := range meta.ExportedTasks {
			if _, ok := common[exported]; !ok {
				continue
			}

			modulesByTask[exported] = append(modulesByTask[exported], logicalTask)
		}
	}

	names := make([]string, 0, len(modulesByTask))

	for name, modules := range modulesByTask {
		if len(modules) >= minModulesForGeneratedRootTask {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	generated := make([]taskfile.GeneratedRootTask, 0, len(names))

	for _, name := range names {
		generated = append(generated, taskfile.GeneratedRootTask{
			Name:    name,
			Modules: modulesByTask[name],
		})
	}

	return generated
}
