// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
)

// MarshalLock encodes a lock file using stable on-disk keys.
func MarshalLock(lock *syncLock) ([]byte, error) {
	data, err := lockmodel.MarshalLock(lock)
	if err != nil {
		return nil, fmt.Errorf("marshal lock: %w", err)
	}

	return data, nil
}

// MarshalMetadata encodes metadata using stable on-disk keys.
func MarshalMetadata(meta *domain.Metadata) ([]byte, error) {
	data, err := domain.MarshalMetadata(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	return data, nil
}
