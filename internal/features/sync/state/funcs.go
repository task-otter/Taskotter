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
	metadata, err := loadStateFile(
		loadStateRequest[domain.Metadata]{
			workspace: workspace,
			rel:       rel,
			label:     "metadata",
			decode: func(data []byte, meta *domain.Metadata) error {
				return yaml.Unmarshal(data, meta)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf(loadMetadataErrorFormat, err)
	}

	return metadata, nil
}

// LoadLock reads and decodes synchronization state from the workspace.
func LoadLock(workspace, rel string) (*lockmodel.LockFile, error) {
	lock, err := loadStateFile(
		loadStateRequest[lockmodel.LockFile]{
			workspace: workspace,
			rel:       rel,
			label:     "lock file",
			decode:    lockmodel.DecodeLockFileYAML,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("load lock file: %w", err)
	}

	return lock, nil
}

func loadStateFile[value any](request loadStateRequest[value]) (*value, error) {
	data, err := pathutil.ReadRelativeFile(request.workspace, request.rel)
	if err != nil {
		return nil, fmt.Errorf("read %s %q: %w", request.label, request.rel, err)
	}

	var result value

	err = request.decode(data, &result)
	if err != nil {
		return nil, fmt.Errorf("parse %s %q: %w", request.label, request.rel, err)
	}

	return &result, nil
}
