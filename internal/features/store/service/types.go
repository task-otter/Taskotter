// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"github.com/task-otter/Taskotter/internal/features/store/domain"
)

type (
	localSnapshotArgs = struct {
		ref     *domain.RefInfo
		catalog map[string]struct{}
		deps    map[string][]string
		root    string
	}
)
