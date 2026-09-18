// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package rootupd

import (
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
)

type (
	// GeneratedRootTask describes a TaskOtter-managed root task that fans out to
	// matching tasks in synced module includes.
	GeneratedRootTask = ports.GeneratedRootTask

	// RootUpdateInput carries data for updating the root Taskfile includes section.
	RootUpdateInput = ports.RootUpdateInput
)
