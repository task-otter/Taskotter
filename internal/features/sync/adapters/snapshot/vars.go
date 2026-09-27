// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package snapshot

import (
	syncports "github.com/task-otter/Taskotter/internal/features/sync/ports"
)

var _ syncports.Snapshot = (*Adapter)(nil)
