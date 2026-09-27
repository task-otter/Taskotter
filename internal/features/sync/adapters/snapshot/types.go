// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package snapshot

import (
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
)

type (
	// Adapter adapts a store Snapshot to sync ports.Snapshot.
	Adapter struct {
		snap *storedomain.Snapshot
	}
)
