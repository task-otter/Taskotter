// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
)

type (
	syncLock          = lockmodel.LockFile
	moduleRecord      = lockmodel.ModuleRecord
	managedFile       = managed.File
	orderedRequested  = lockmodel.OrderedRequested
	generatedRootTask = rootupd.GeneratedRootTask
	rootUpdateInput   = rootupd.RootUpdateInput
)
