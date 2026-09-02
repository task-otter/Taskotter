// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
)

// MarshalLock encodes a lock file using stable on-disk keys.
func MarshalLock(lock *syncLock) []byte {
	return lockmodel.MarshalLock(lock)
}

// MarshalMetadata encodes metadata using stable on-disk keys.
func MarshalMetadata(meta *domain.Metadata) []byte {
	return domain.MarshalMetadata(meta)
}
