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

// LoadMetadata reads TaskOtter metadata from workspace-relative path rel.
func LoadMetadata(workspace, rel string) (*domain.Metadata, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("read metadata %q: %w", rel, err)
	}

	var meta domain.Metadata

	if err = yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata %q: %w", rel, err)
	}

	return &meta, nil
}

// LoadLock reads the TaskOtter lock file from workspace-relative path rel.
func LoadLock(workspace, rel string) (*lockmodel.LockFile, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("read lock file %q: %w", rel, err)
	}

	var lock lockmodel.LockFile

	if err = lockmodel.DecodeLockFileYAML(data, &lock); err != nil {
		return nil, fmt.Errorf("parse lock file %q: %w", rel, err)
	}

	return &lock, nil
}
