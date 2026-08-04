// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package lockmodel

import (
	"errors"
	"fmt"
	"slices"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/yamlfmt"
	yaml "go.yaml.in/yaml/v3"
)

type (
	yamlDecodeTarget struct {
		Out any
		Key string
	}
)

const (
	errDecode                 = "decode %q: %w"
	errMarshalLockFile        = "marshal lock file: %w"
	yamlKeyConfiguration      = "configuration"
	yamlKeyDefaultBranch      = "default_branch"
	yamlKeyDependencies       = "dependencies"
	yamlKeyDestinationModule  = "destination_module"
	yamlKeyGeneratedRootTasks = "generated_root_tasks"
	yamlKeyIncludesDoc        = "includes_doc"
	yamlKeyManagedFiles       = "managed_files"
	yamlKeyNodePackageManager = "node_package_manager"
	yamlKeyNodeVersionManager = "node_version_manager"
	yamlKeyPath               = "path"
	yamlKeyRepository         = "repository"
	yamlKeyRequested          = "requested"
	yamlKeyRequestedVersion   = "requested_version"
	yamlKeyResolvedCommit     = "resolved_commit"
	yamlKeyResolvedModules    = "resolved_modules"
	yamlKeySHA256             = "sha256"
	yamlKeySource             = "source"
	yamlKeySourceModule       = "source_module"
	yamlKeySourcePath         = "source_path"
	yamlKeySourceRef          = "source_ref"
	yamlKeySyncRoot           = "sync_root"
	yamlKeyTargetFolder       = "target_folder"
	yamlKeyTasks              = "tasks"
	yamlMappingPairKeyValue   = consts.IndexTwo
)

var errYAMLMappingNodeExpected = errors.New("expected YAML mapping node")

// MarshalLock encodes a lock file using stable on-disk keys.
func MarshalLock(lock *LockFile) ([]byte, error) {
	data, err := yamlfmt.Marshal(EncodeLockFile(lock))
	if err != nil {
		return nil, fmt.Errorf(errMarshalLockFile, err)
	}

	return data, nil
}

// EncodeLockFile converts a lock file into a map with stable on-disk keys.
func EncodeLockFile(lock *LockFile) map[string]any {
	out := map[string]any{
		yamlKeySource:        marshalLockSource(&lock.Source),
		yamlKeyConfiguration: marshalLockConfiguration(&lock.Configuration),
		yamlKeyResolvedModules: map[string]any{
			yamlKeyRequested:    encodeOrderedRequested(lock.Requested),
			yamlKeyDependencies: encodeModuleRecords(lock.Dependencies),
		},
		yamlKeyManagedFiles: encodeManagedFiles(lock.ManagedFiles),
	}

	if len(lock.GeneratedRootTasks) > consts.IndexZero {
		out[yamlKeyGeneratedRootTasks] = lock.GeneratedRootTasks
	}

	return out
}

// UnmarshalYAML decodes the TaskOtter lock file from its stable on-disk keys.
func (lock *LockFile) UnmarshalYAML(value *yaml.Node) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode lock file: %w", err)
	}

	err = decodeLockSections(fields, lock)
	if err != nil {
		return fmt.Errorf("decode lock sections: %w", err)
	}

	err = decodeYAMLFields(
		fields,
		yamlDecodeTarget{Key: yamlKeyGeneratedRootTasks, Out: &lock.GeneratedRootTasks},
		yamlDecodeTarget{Key: yamlKeyManagedFiles, Out: &lock.ManagedFiles},
	)
	if err != nil {
		return fmt.Errorf("decode lock top-level fields: %w", err)
	}

	return nil
}

// UnmarshalYAML decodes a module record from the lock file's snake_case keys.
func (record *ModuleRecord) UnmarshalYAML(value *yaml.Node) error {
	err := unmarshalYAMLTargets(
		value, "module record",
		yamlDecodeTarget{Key: yamlKeySourceModule, Out: &record.SourceModule},
		yamlDecodeTarget{Key: yamlKeyDestinationModule, Out: &record.DestinationModule},
		yamlDecodeTarget{Key: yamlKeyPath, Out: &record.Path},
	)
	if err != nil {
		return fmt.Errorf("unmarshal module record: %w", err)
	}

	return nil
}

// UnmarshalYAML decodes ordered requested modules from the lock file.
func (requested *OrderedRequested) UnmarshalYAML(value *yaml.Node) error {
	var raw map[string]ModuleRecord

	err := value.Decode(&raw)
	if err != nil {
		return fmt.Errorf("decode ordered requested modules: %w", err)
	}

	*requested = OrderedRequested(raw)

	return nil
}

func decodeLockConfiguration(fields map[string]*yaml.Node, lock *LockFile) error {
	cfg, err := nestedYAMLFields(fields, yamlKeyConfiguration)
	if err != nil {
		return fmt.Errorf("nested configuration fields: %w", err)
	}

	err = decodeLockConfigurationFields(cfg, lock)
	if err != nil {
		return fmt.Errorf("decode lock configuration fields: %w", err)
	}

	return nil
}

func decodeLockConfigurationFields(cfg map[string]*yaml.Node, lock *LockFile) error {
	err := decodeYAMLFields(
		cfg,
		yamlDecodeTarget{Key: yamlKeyTargetFolder, Out: &lock.Configuration.TargetFolder},
		yamlDecodeTarget{Key: yamlKeyTasks, Out: &lock.Configuration.Tasks},
	)
	if err != nil {
		return fmt.Errorf("decode configuration fields: %w", err)
	}

	err = decodeLockConfigurationRuntime(cfg, lock)
	if err != nil {
		return fmt.Errorf("decode lock configuration runtime: %w", err)
	}

	return nil
}

func decodeLockConfigurationRuntime(cfg map[string]*yaml.Node, lock *LockFile) error {
	err := decodeYAMLFields(
		cfg,
		yamlDecodeTarget{
			Key: yamlKeyNodePackageManager,
			Out: &lock.Configuration.NodePackageManager,
		},
		yamlDecodeTarget{
			Key: yamlKeyNodeVersionManager,
			Out: &lock.Configuration.NodeVersionManager,
		},
		yamlDecodeTarget{Key: yamlKeyIncludesDoc, Out: &lock.Configuration.IncludesDoc},
		yamlDecodeTarget{Key: yamlKeySyncRoot, Out: &lock.Configuration.SyncRoot},
	)
	if err != nil {
		return fmt.Errorf("decode configuration runtime fields: %w", err)
	}

	return nil
}

func decodeLockResolvedModules(fields map[string]*yaml.Node, lock *LockFile) error {
	resolved, err := nestedYAMLFields(fields, yamlKeyResolvedModules)
	if err != nil {
		return fmt.Errorf("nested resolved modules fields: %w", err)
	}

	err = decodeYAMLFields(
		resolved,
		yamlDecodeTarget{Key: yamlKeyRequested, Out: &lock.Requested},
		yamlDecodeTarget{Key: yamlKeyDependencies, Out: &lock.Dependencies},
	)
	if err != nil {
		return fmt.Errorf("decode resolved modules fields: %w", err)
	}

	return nil
}

func decodeLockSections(fields map[string]*yaml.Node, lock *LockFile) error {
	err := decodeLockSource(fields, lock)
	if err != nil {
		return fmt.Errorf("decode lock source: %w", err)
	}

	err = decodeLockConfiguration(fields, lock)
	if err != nil {
		return fmt.Errorf("decode lock configuration: %w", err)
	}

	err = decodeLockResolvedModules(fields, lock)
	if err != nil {
		return fmt.Errorf("decode lock resolved modules: %w", err)
	}

	return nil
}

func decodeLockSource(fields map[string]*yaml.Node, lock *LockFile) error {
	source, err := nestedYAMLFields(fields, yamlKeySource)
	if err != nil {
		return fmt.Errorf("nested source fields: %w", err)
	}

	err = decodeYAMLFields(
		source,
		yamlDecodeTarget{Key: yamlKeyRepository, Out: &lock.Source.Repository},
		yamlDecodeTarget{Key: yamlKeyRequestedVersion, Out: &lock.Source.RequestedVersion},
		yamlDecodeTarget{Key: yamlKeySourceRef, Out: &lock.Source.SourceRef},
		yamlDecodeTarget{Key: yamlKeyResolvedCommit, Out: &lock.Source.ResolvedCommit},
		yamlDecodeTarget{Key: yamlKeyDefaultBranch, Out: &lock.Source.DefaultBranch},
	)
	if err != nil {
		return fmt.Errorf("decode source fields: %w", err)
	}

	return nil
}

func decodeYAMLField(fields map[string]*yaml.Node, key string, out any) error {
	node := fields[key]

	if node == nil {
		return nil
	}

	err := node.Decode(out)
	if err != nil {
		return fmt.Errorf(errDecode, key, err)
	}

	return nil
}

func decodeYAMLFields(fields map[string]*yaml.Node, targets ...yamlDecodeTarget) error {
	for i := range targets {
		target := &targets[i]

		err := decodeYAMLField(fields, target.Key, target.Out)
		if err != nil {
			return fmt.Errorf("decode field %q: %w", target.Key, err)
		}
	}

	return nil
}

func encodeManagedFile(file *managed.File) map[string]string {
	return map[string]string{
		yamlKeySourceModule:      file.SourceModule,
		yamlKeyDestinationModule: file.DestinationModule,
		yamlKeySourcePath:        file.SourcePath,
		yamlKeyPath:              file.Path,
		yamlKeySHA256:            file.SHA256,
	}
}

func encodeManagedFiles(files []managed.File) []map[string]string {
	managedFiles := make([]map[string]string, consts.IndexZero, len(files))

	for i := range files {
		managedFiles = append(managedFiles, encodeManagedFile(&files[i]))
	}

	return managedFiles
}

func encodeModuleRecord(record *ModuleRecord) map[string]string {
	out := map[string]string{
		yamlKeySourceModule:      record.SourceModule,
		yamlKeyDestinationModule: record.DestinationModule,
	}

	if record.Path != consts.Empty {
		out[yamlKeyPath] = record.Path
	}

	return out
}

func encodeModuleRecords(records []ModuleRecord) []map[string]string {
	out := make([]map[string]string, consts.IndexZero, len(records))

	for i := range records {
		out = append(out, encodeModuleRecord(&records[i]))
	}

	return out
}

func encodeOrderedRequested(requested OrderedRequested) map[string]map[string]string {
	if len(requested) == consts.IndexZero {
		return map[string]map[string]string{}
	}

	keys := sortedRequestedKeys(requested)
	out := make(map[string]map[string]string, len(requested))

	for i := range keys {
		rec := requested[keys[i]]

		out[keys[i]] = encodeModuleRecord(&rec)
	}

	return out
}

func marshalLockConfiguration(cfg *LockConfiguration) map[string]any {
	return map[string]any{
		yamlKeyTargetFolder:       cfg.TargetFolder,
		yamlKeyTasks:              cfg.Tasks,
		yamlKeyNodePackageManager: cfg.NodePackageManager,
		yamlKeyNodeVersionManager: cfg.NodeVersionManager,
		yamlKeyIncludesDoc:        cfg.IncludesDoc,
		yamlKeySyncRoot:           cfg.SyncRoot,
	}
}

func marshalLockSource(source *LockSource) map[string]any {
	return map[string]any{
		yamlKeyRepository:       source.Repository,
		yamlKeyRequestedVersion: source.RequestedVersion,
		yamlKeySourceRef:        source.SourceRef,
		yamlKeyResolvedCommit:   source.ResolvedCommit,
		yamlKeyDefaultBranch:    source.DefaultBranch,
	}
}

func nestedYAMLFields(fields map[string]*yaml.Node, key string) (map[string]*yaml.Node, error) {
	node := fields[key]

	if node == nil {
		return map[string]*yaml.Node{}, nil
	}

	nested, err := yamlFields(node)
	if err != nil {
		return nil, fmt.Errorf(errDecode, key, err)
	}

	return nested, nil
}

func sortedRequestedKeys(requested OrderedRequested) []string {
	keys := make([]string, consts.IndexZero, len(requested))

	for key := range requested {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

func unmarshalYAMLTargets(value *yaml.Node, label string, targets ...yamlDecodeTarget) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}

	err = decodeYAMLFields(fields, targets...)
	if err != nil {
		return fmt.Errorf("decode %s fields: %w", label, err)
	}

	return nil
}

func yamlFields(value *yaml.Node) (map[string]*yaml.Node, error) {
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: got %v", errYAMLMappingNodeExpected, value.Kind)
	}

	fields := make(map[string]*yaml.Node, len(value.Content)/yamlMappingPairKeyValue)

	for idx := consts.IndexZero; idx < len(value.Content); idx += yamlMappingPairKeyValue {
		fields[value.Content[idx].Value] = value.Content[idx+consts.IndexOne]
	}

	return fields, nil
}
