// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package ports defines sync feature outbound dependencies.
package ports

import (
	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
)

type (
	// Snapshot provides module paths and store ref metadata without importing store adapters.
	Snapshot interface {
		ModuleDir(sourceModule string) string
		WorkspaceRoot() string
		SourceRef() string
		ResolvedCommit() string
		DefaultBranch() string
	}

	// TaskfileOps rewrites and updates Taskfile YAML without the sync service importing adapters.
	TaskfileOps interface {
		NewRootTemplate() []byte
		RewriteIncludes(content []byte, sourceToDest map[string]string, fromDest string) ([]byte, error)
		UpdateRootTaskfile(content []byte, input *rootupd.RootUpdateInput) ([]byte, error)
	}
)
