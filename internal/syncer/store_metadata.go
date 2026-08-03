// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/taskfile"
	yaml "go.yaml.in/yaml/v3"
)

type storeTaskMetadata struct {
	Schema        string
	Module        string
	Taskfile      string
	ExportedTasks []string
	Variants      []string
}

const storeMetadataFileName = "metadata.yml"

// minModulesForGeneratedRootTask is the minimum number of modules that must
// export a task before TaskOtter promotes it to a generated root task.
var minModulesForGeneratedRootTask = yamlMappingPairKeyValue

func (meta *storeTaskMetadata) UnmarshalYAML(value *yaml.Node) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode store task metadata: %w", err)
	}

	err = decodeYAMLFields(
		fields,
		yamlDecodeTarget{key: yamlKeySchema, out: &meta.Schema},
		yamlDecodeTarget{key: yamlKeyModule, out: &meta.Module},
		yamlDecodeTarget{key: yamlKeyTaskfile, out: &meta.Taskfile},
		yamlDecodeTarget{key: yamlKeyExportedTasks, out: &meta.ExportedTasks},
		yamlDecodeTarget{key: yamlKeyVariants, out: &meta.Variants},
	)
	if err != nil {
		return fmt.Errorf("decode store task metadata fields: %w", err)
	}

	return nil
}

func loadStoreTaskMetadata(snapshot *store.Snapshot) (map[string]storeTaskMetadata, error) {
	out := make(map[string]storeTaskMetadata)
	root := filepath.Join(snapshot.RootDir, taskfilesDirName)

	err := filepath.WalkDir(root, storeMetadataWalker(root, out))
	if err != nil {
		return nil, fmt.Errorf("load store metadata: %w", err)
	}

	return out, nil
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

func moduleNameFor(root, abs string) (string, error) {
	relDir, err := filepath.Rel(root, filepath.Dir(abs))
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

	var meta storeTaskMetadata

	err = yaml.Unmarshal(data, &meta)
	if err != nil {
		return emptyStoreTaskMetadata(), fmt.Errorf("parse metadata for %q: %w", module, err)
	}

	if meta.Module == consts.Empty {
		meta.Module = module
	}

	return meta, nil
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

func resolveStoreTaskMetadata(
	sourceModule string,
	metadata map[string]storeTaskMetadata,
) (storeTaskMetadata, bool) {
	current := sourceModule

	for current != consts.Empty {
		if meta, foundMeta := metadata[current]; foundMeta {
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
		if counts[task] >= minModulesForGeneratedRootTask {
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
		task := exportedTasks[i]

		if task == consts.Empty {
			continue
		}

		if _, ok := seen[task]; ok {
			continue
		}

		seen[task] = struct{}{}
		out = append(out, task)
	}

	return out
}

func buildGeneratedRootTasks(
	requested []string,
	requestedRecords map[string]ModuleRecord,
	metadata map[string]storeTaskMetadata,
) []taskfile.GeneratedRootTask {
	common := commonStoreTaskNames(metadata)
	modulesByTask := groupModulesByGeneratedTask(requested, requestedRecords, metadata, common)

	return finalizeGeneratedTasks(modulesByTask)
}

func groupModulesByGeneratedTask(
	requested []string,
	requestedRecords map[string]ModuleRecord,
	metadata map[string]storeTaskMetadata,
	common map[string]struct{},
) map[string][]string {
	modulesByTask := make(map[string][]string)

	for i := range requested {
		logicalTask := requested[i]
		record, foundRecord := requestedRecords[logicalTask]

		if !foundRecord {
			continue
		}

		meta, ok := resolveStoreTaskMetadata(record.SourceModule, metadata)

		if !ok {
			continue
		}

		addCommonExportedTasks(modulesByTask, meta.ExportedTasks, common, logicalTask)
	}

	return modulesByTask
}

func addCommonExportedTasks(
	modulesByTask map[string][]string,
	exportedTasks []string,
	common map[string]struct{},
	logicalTask string,
) {
	for i := range exportedTasks {
		exported := exportedTasks[i]

		if _, ok := common[exported]; !ok {
			continue
		}

		modulesByTask[exported] = append(modulesByTask[exported], logicalTask)
	}
}

func finalizeGeneratedTasks(modulesByTask map[string][]string) []taskfile.GeneratedRootTask {
	names := make([]string, consts.IndexZero, len(modulesByTask))

	for name := range modulesByTask {
		if len(modulesByTask[name]) >= minModulesForGeneratedRootTask {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	generated := make([]taskfile.GeneratedRootTask, consts.IndexZero, len(names))

	for i := range names {
		name := names[i]

		generated = append(generated, taskfile.GeneratedRootTask{
			Name:    name,
			Modules: modulesByTask[name],
		})
	}

	return generated
}
