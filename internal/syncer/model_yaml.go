// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"errors"
	"fmt"

	yaml "go.yaml.in/yaml/v3"
)

const yamlMappingPairKeyValue = 2

var errYAMLMappingNodeExpected = errors.New("expected YAML mapping node")

const (
	yamlKeyConfiguration      = "configuration"
	yamlKeyConfigurationHash  = "configuration_hash"
	yamlKeyDefaultBranch      = "default_branch"
	yamlKeyDependencies       = "dependencies"
	yamlKeyDestinationModule  = "destination_module"
	yamlKeyExportedTasks      = "exported_tasks"
	yamlKeyGeneratedRootTasks = "generated_root_tasks"
	yamlKeyIncludesDoc        = "includes_doc"
	yamlKeyLockFile           = "lock_file"
	yamlKeyManagedFiles       = "managed_files"
	yamlKeyModule             = "module"
	yamlKeyNodePackageManager = "node_package_manager"
	yamlKeyNodeVersionManager = "node_version_manager"
	yamlKeyPath               = "path"
	yamlKeyRepository         = "repository"
	yamlKeyRequested          = "requested"
	yamlKeyRequestedVersion   = "requested_version"
	yamlKeyResolvedCommit     = "resolved_commit"
	yamlKeyResolvedModules    = "resolved_modules"
	yamlKeySchema             = "schema"
	yamlKeySHA256             = "sha256"
	yamlKeySource             = "source"
	yamlKeySourceModule       = "source_module"
	yamlKeySourcePath         = "source_path"
	yamlKeySourceRef          = "source_ref"
	yamlKeySyncRoot           = "sync_root"
	yamlKeyTargetFolder       = "target_folder"
	yamlKeyTaskfile           = "taskfile"
	yamlKeyTasks              = "tasks"
	yamlKeyVariants           = "variants"
)

type yamlDecodeTarget struct {
	out any
	key string
}

// MarshalYAML encodes a module record using the lock file's snake_case keys.
func (record ModuleRecord) MarshalYAML() (any, error) {
	out := map[string]any{
		yamlKeySourceModule:      record.SourceModule,
		yamlKeyDestinationModule: record.DestinationModule,
	}

	if record.Path != "" {
		out[yamlKeyPath] = record.Path
	}

	return out, nil
}

// UnmarshalYAML decodes a module record from the lock file's snake_case keys.
func (record *ModuleRecord) UnmarshalYAML(value *yaml.Node) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode module record: %w", err)
	}

	return decodeYAMLFields(
		fields,
		yamlDecodeTarget{key: yamlKeySourceModule, out: &record.SourceModule},
		yamlDecodeTarget{key: yamlKeyDestinationModule, out: &record.DestinationModule},
		yamlDecodeTarget{key: yamlKeyPath, out: &record.Path},
	)
}

// MarshalYAML encodes a managed file using the lock file's snake_case keys.
func (managed ManagedFile) MarshalYAML() (any, error) {
	return map[string]any{
		yamlKeySourceModule:      managed.SourceModule,
		yamlKeyDestinationModule: managed.DestinationModule,
		yamlKeySourcePath:        managed.SourcePath,
		yamlKeyPath:              managed.Path,
		yamlKeySHA256:            managed.SHA256,
	}, nil
}

// UnmarshalYAML decodes a managed file from the lock file's snake_case keys.
func (managed *ManagedFile) UnmarshalYAML(value *yaml.Node) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode managed file: %w", err)
	}

	return decodeYAMLFields(
		fields,
		yamlDecodeTarget{key: yamlKeySourceModule, out: &managed.SourceModule},
		yamlDecodeTarget{key: yamlKeyDestinationModule, out: &managed.DestinationModule},
		yamlDecodeTarget{key: yamlKeySourcePath, out: &managed.SourcePath},
		yamlDecodeTarget{key: yamlKeyPath, out: &managed.Path},
		yamlDecodeTarget{key: yamlKeySHA256, out: &managed.SHA256},
	)
}

// MarshalYAML encodes the TaskOtter lock file using its stable on-disk keys.
func (lock LockFile) MarshalYAML() (any, error) {
	out := map[string]any{
		yamlKeySource: map[string]any{
			yamlKeyRepository:       lock.Source.Repository,
			yamlKeyRequestedVersion: lock.Source.RequestedVersion,
			yamlKeySourceRef:        lock.Source.SourceRef,
			yamlKeyResolvedCommit:   lock.Source.ResolvedCommit,
			yamlKeyDefaultBranch:    lock.Source.DefaultBranch,
		},
		yamlKeyConfiguration: map[string]any{
			yamlKeyTargetFolder:       lock.Configuration.TargetFolder,
			yamlKeyTasks:              lock.Configuration.Tasks,
			yamlKeyNodePackageManager: lock.Configuration.NodePackageManager,
			yamlKeyNodeVersionManager: lock.Configuration.NodeVersionManager,
			yamlKeyIncludesDoc:        lock.Configuration.IncludesDoc,
			yamlKeySyncRoot:           lock.Configuration.SyncRoot,
		},
		yamlKeyResolvedModules: map[string]any{
			yamlKeyRequested:    lock.ResolvedModules.Requested,
			yamlKeyDependencies: lock.ResolvedModules.Dependencies,
		},
		yamlKeyManagedFiles: lock.ManagedFiles,
	}

	if len(lock.GeneratedRootTasks) > 0 {
		out[yamlKeyGeneratedRootTasks] = lock.GeneratedRootTasks
	}

	return out, nil
}

// UnmarshalYAML decodes the TaskOtter lock file from its stable on-disk keys.
func (lock *LockFile) UnmarshalYAML(value *yaml.Node) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode lock file: %w", err)
	}

	err = decodeLockSource(fields, lock)
	if err != nil {
		return err
	}

	err = decodeLockConfiguration(fields, lock)
	if err != nil {
		return err
	}

	err = decodeLockResolvedModules(fields, lock)
	if err != nil {
		return err
	}

	return decodeYAMLFields(
		fields,
		yamlDecodeTarget{key: yamlKeyGeneratedRootTasks, out: &lock.GeneratedRootTasks},
		yamlDecodeTarget{key: yamlKeyManagedFiles, out: &lock.ManagedFiles},
	)
}

func decodeLockSource(fields map[string]*yaml.Node, lock *LockFile) error {
	source, err := nestedYAMLFields(fields, yamlKeySource)
	if err != nil {
		return err
	}

	return decodeYAMLFields(
		source,
		yamlDecodeTarget{key: yamlKeyRepository, out: &lock.Source.Repository},
		yamlDecodeTarget{key: yamlKeyRequestedVersion, out: &lock.Source.RequestedVersion},
		yamlDecodeTarget{key: yamlKeySourceRef, out: &lock.Source.SourceRef},
		yamlDecodeTarget{key: yamlKeyResolvedCommit, out: &lock.Source.ResolvedCommit},
		yamlDecodeTarget{key: yamlKeyDefaultBranch, out: &lock.Source.DefaultBranch},
	)
}

func decodeLockConfiguration(fields map[string]*yaml.Node, lock *LockFile) error {
	cfg, err := nestedYAMLFields(fields, yamlKeyConfiguration)
	if err != nil {
		return err
	}

	return decodeYAMLFields(
		cfg,
		yamlDecodeTarget{key: yamlKeyTargetFolder, out: &lock.Configuration.TargetFolder},
		yamlDecodeTarget{key: yamlKeyTasks, out: &lock.Configuration.Tasks},
		yamlDecodeTarget{
			key: yamlKeyNodePackageManager,
			out: &lock.Configuration.NodePackageManager,
		},
		yamlDecodeTarget{
			key: yamlKeyNodeVersionManager,
			out: &lock.Configuration.NodeVersionManager,
		},
		yamlDecodeTarget{key: yamlKeyIncludesDoc, out: &lock.Configuration.IncludesDoc},
		yamlDecodeTarget{key: yamlKeySyncRoot, out: &lock.Configuration.SyncRoot},
	)
}

func decodeLockResolvedModules(fields map[string]*yaml.Node, lock *LockFile) error {
	resolved, err := nestedYAMLFields(fields, yamlKeyResolvedModules)
	if err != nil {
		return err
	}

	return decodeYAMLFields(
		resolved,
		yamlDecodeTarget{key: yamlKeyRequested, out: &lock.ResolvedModules.Requested},
		yamlDecodeTarget{key: yamlKeyDependencies, out: &lock.ResolvedModules.Dependencies},
	)
}

// MarshalYAML encodes TaskOtter metadata using its stable on-disk keys.
func (meta Metadata) MarshalYAML() (any, error) {
	return map[string]any{
		yamlKeyTargetFolder:      meta.TargetFolder,
		yamlKeyLockFile:          meta.LockFile,
		yamlKeyConfigurationHash: meta.ConfigurationHash,
	}, nil
}

// UnmarshalYAML decodes TaskOtter metadata from its stable on-disk keys.
func (meta *Metadata) UnmarshalYAML(value *yaml.Node) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode metadata: %w", err)
	}

	return decodeYAMLFields(
		fields,
		yamlDecodeTarget{key: yamlKeyTargetFolder, out: &meta.TargetFolder},
		yamlDecodeTarget{key: yamlKeyLockFile, out: &meta.LockFile},
		yamlDecodeTarget{key: yamlKeyConfigurationHash, out: &meta.ConfigurationHash},
	)
}

func yamlFields(value *yaml.Node) (map[string]*yaml.Node, error) {
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: got %v", errYAMLMappingNodeExpected, value.Kind)
	}

	fields := make(map[string]*yaml.Node, len(value.Content)/yamlMappingPairKeyValue)

	for idx := 0; idx < len(value.Content); idx += yamlMappingPairKeyValue {
		fields[value.Content[idx].Value] = value.Content[idx+1]
	}

	return fields, nil
}

func nestedYAMLFields(fields map[string]*yaml.Node, key string) (map[string]*yaml.Node, error) {
	node := fields[key]

	if node == nil {
		return map[string]*yaml.Node{}, nil
	}

	nested, err := yamlFields(node)
	if err != nil {
		return nil, fmt.Errorf("decode %q: %w", key, err)
	}

	return nested, nil
}

func decodeYAMLFields(fields map[string]*yaml.Node, targets ...yamlDecodeTarget) error {
	for _, target := range targets {
		err := decodeYAMLField(fields, target.key, target.out)
		if err != nil {
			return err
		}
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
		return fmt.Errorf("decode %q: %w", key, err)
	}

	return nil
}
