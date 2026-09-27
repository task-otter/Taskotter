// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"os"

	"github.com/task-otter/Taskotter/internal/features/store/domain"
)

type (
	catalogWalk struct {
		catalog map[string]struct{}
		dir     string
		prefix  string
		entries []os.DirEntry
	}

	localSnapshotArgs = struct {
		ref     *domain.RefInfo
		catalog map[string]struct{}
		deps    map[string][]string
		root    string
	}
)
