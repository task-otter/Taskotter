// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

// LoadMetadata reads and decodes module metadata from the workspace.
func LoadMetadata(workspace, rel string) (*domain.Metadata, error) {
	return loadStateFile[domain.Metadata](
		workspace,
		rel,
		"metadata",
		func(data []byte, meta *domain.Metadata) error {
			return yaml.Unmarshal(data, meta)
		},
	)
}

// LoadLock reads and decodes synchronization state from the workspace.
func LoadLock(workspace, rel string) (*lockmodel.LockFile, error) {
	return loadStateFile[lockmodel.LockFile](
		workspace,
		rel,
		"lock file",
		lockmodel.DecodeLockFileYAML,
	)
}

func loadStateFile[T any](
	workspace, rel, label string,
	decode func([]byte, *T) error,
) (*T, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("read %s %q: %w", label, rel, err)
	}

	var value T

	err = decode(data, &value)
	if err != nil {
		return nil, fmt.Errorf("parse %s %q: %w", label, rel, err)
	}

	return &value, nil
}
