// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
)

type (
	// SnapshotAdapter adapts a store Snapshot to sync ports.Snapshot.
	SnapshotAdapter struct {
		snap *storedomain.Snapshot
	}
)

// DefaultBranch implements ports.Snapshot.
func (port *SnapshotAdapter) DefaultBranch() string {
	return storedomain.DefaultBranch(port.snap)
}

// ModuleDir implements ports.Snapshot.
func (port *SnapshotAdapter) ModuleDir(sourceModule string) string {
	return storedomain.ModuleDir(port.snap, sourceModule)
}

// ResolvedCommit implements ports.Snapshot.
func (port *SnapshotAdapter) ResolvedCommit() string {
	return storedomain.ResolvedCommit(port.snap)
}

// SourceRef implements ports.Snapshot.
func (port *SnapshotAdapter) SourceRef() string {
	return storedomain.SourceRef(port.snap)
}

// WorkspaceRoot implements ports.Snapshot.
func (port *SnapshotAdapter) WorkspaceRoot() string {
	return storedomain.WorkspaceRoot(port.snap)
}

// SnapshotPort adapts snap for PrepareSyncInput and related sync ports.
func SnapshotPort(snap *storedomain.Snapshot) *SnapshotAdapter {
	return &SnapshotAdapter{snap: snap}
}
