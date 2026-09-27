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
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("read metadata %q: %w", rel, err)
	}

	var meta domain.Metadata

	err = yaml.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("parse metadata %q: %w", rel, err)
	}

	return &meta, nil
}

// LoadLock reads and decodes synchronization state from the workspace.
func LoadLock(workspace, rel string) (*lockmodel.LockFile, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("read lock file %q: %w", rel, err)
	}

	var lock lockmodel.LockFile

	err = lockmodel.DecodeLockFileYAML(data, &lock)
	if err != nil {
		return nil, fmt.Errorf("parse lock file %q: %w", rel, err)
	}

	return &lock, nil
}
