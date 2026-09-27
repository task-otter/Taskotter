// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"errors"
	"fmt"
	"os"

	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

// LoadMetadata reads synchronization metadata from the workspace.
func LoadMetadata(workspace, rel string) (*Metadata, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("load metadata: read %q: %w", rel, err)
	}

	var metadata Metadata

	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("load metadata: parse %q: %w", rel, err)
	}

	return &metadata, nil
}

// LoadLock reads synchronization lock state from the workspace.
func LoadLock(workspace, rel string) (*LockFile, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("load lock file: read %q: %w", rel, err)
	}

	var lock LockFile

	if err := lockmodel.DecodeLockFileYAML(data, &lock); err != nil {
		return nil, fmt.Errorf("load lock file: parse %q: %w", rel, err)
	}

	return &lock, nil
}

// LoadPrevious reads the current managed state, with the legacy metadata path
// as a compatibility fallback. Detailed migration cleanup remains owned by
// the plan/apply pipeline.
func LoadPrevious(workspace string, cfg *config.Config) (Previous, error) {
	metadataPath := config.MetadataPath(cfg)

	metadata, err := LoadMetadata(workspace, metadataPath)
	if err != nil {
		metadata, err = LoadMetadata(workspace, config.LegacyMetadataPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return Previous{MetadataPath: metadataPath, LockPath: config.LockFilePath(cfg)}, nil
			}

			return Previous{}, err
		}

		metadataPath = config.LegacyMetadataPath
	}

	lockPath := config.LockFilePath(cfg)

	lock, err := LoadLock(workspace, lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Previous{
				Metadata:     metadata,
				MetadataPath: metadataPath,
				LockPath:     lockPath,
			}, nil
		}

		return Previous{}, err
	}

	return Previous{
		Metadata:        metadata,
		Lock:            lock,
		MetadataPath:    metadataPath,
		LockPath:        lockPath,
		OldTargetFolder: metadata.TargetFolder,
	}, nil
}

func EncodeLock(lock *LockFile) ([]byte, error) {
	return lockmodel.MarshalLock(lock), nil
}

func EncodeMetadata(metadata *Metadata) ([]byte, error) {
	return MarshalMetadata(metadata), nil
}

func DecodeLock(data []byte) (*LockFile, error) {
	var lock LockFile

	err := lockmodel.DecodeLockFileYAML(data, &lock)
	if err != nil {
		return nil, err
	}

	return &lock, nil
}

func DecodeMetadata(data []byte) (*Metadata, error) {
	var metadata Metadata

	err := DecodeMetadataYAML(data, &metadata)
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}
