// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

type (
	loadStateArgs struct {
		decode func([]byte) error
		label  string
		rel    string
		root   string
	}
)

// LoadMetadata reads synchronization metadata from the workspace.
func LoadMetadata(workspace, rel string) (*Metadata, error) {
	var metadata Metadata

	err := loadStateFile(&loadStateArgs{
		root: workspace, rel: rel, label: metadataLabel,
		decode: func(data []byte) error { return yaml.Unmarshal(data, &metadata) },
	})
	if err != nil {
		return nil, fmt.Errorf("load metadata: %w", err)
	}

	return &metadata, nil
}

// LoadLock reads synchronization lock state from the workspace.
func LoadLock(workspace, rel string) (*lockmodel.LockFile, error) {
	var lock lockmodel.LockFile

	err := loadStateFile(&loadStateArgs{
		root: workspace, rel: rel, label: "lock file",
		decode: func(data []byte) error { return lockmodel.DecodeLockFileYAML(data, &lock) },
	})
	if err != nil {
		return nil, fmt.Errorf("load lock file: %w", err)
	}

	return &lock, nil
}

// EncodeLock serializes a lock file.
func EncodeLock(lock *lockmodel.LockFile) []byte {
	return lockmodel.MarshalLock(lock)
}

// EncodeMetadata serializes synchronization metadata.
func EncodeMetadata(metadata *Metadata) []byte {
	return MarshalMetadata(metadata)
}

// DecodeLock deserializes a lock file.
func DecodeLock(data []byte) (*lockmodel.LockFile, error) {
	var lock lockmodel.LockFile

	err := lockmodel.DecodeLockFileYAML(data, &lock)
	if err != nil {
		return nil, fmt.Errorf("decode lock: %w", err)
	}

	return &lock, nil
}

// DecodeMetadata deserializes synchronization metadata.
func DecodeMetadata(data []byte) (*Metadata, error) {
	var metadata Metadata

	err := DecodeMetadataYAML(data, &metadata)
	if err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}

	return &metadata, nil
}

func loadStateFile(args *loadStateArgs) error {
	data, err := pathutil.ReadRelativeFile(args.root, args.rel)
	if err != nil {
		return fmt.Errorf("load %s: read %q: %w", args.label, args.rel, err)
	}

	err = args.decode(data)
	if err != nil {
		return fmt.Errorf("load %s: parse %q: %w", args.label, args.rel, err)
	}

	return nil
}
