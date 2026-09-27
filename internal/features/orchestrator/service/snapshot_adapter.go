// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
)

type (
	// SnapshotAdapter adapts a store snapshot to the planner snapshot contract.
	SnapshotAdapter struct {
		snapshot *storedomain.Snapshot
	}
)

// NewSnapshotAdapter adapts a store snapshot for sync planning.
func NewSnapshotAdapter(snapshot *storedomain.Snapshot) *SnapshotAdapter {
	return &SnapshotAdapter{snapshot: snapshot}
}

// DefaultBranch returns the snapshot's default branch.
func (adapter *SnapshotAdapter) DefaultBranch() string {
	return storedomain.DefaultBranch(adapter.snapshot)
}

// ModuleDir returns a module path within the snapshot.
func (adapter *SnapshotAdapter) ModuleDir(sourceModule string) string {
	return storedomain.ModuleDir(adapter.snapshot, sourceModule)
}

// ResolvedCommit returns the snapshot's resolved commit.
func (adapter *SnapshotAdapter) ResolvedCommit() string {
	return storedomain.ResolvedCommit(adapter.snapshot)
}

// SourceRef returns the source reference recorded by the snapshot.
func (adapter *SnapshotAdapter) SourceRef() string {
	return storedomain.SourceRef(adapter.snapshot)
}

// WorkspaceRoot returns the snapshot's workspace root.
func (adapter *SnapshotAdapter) WorkspaceRoot() string {
	return storedomain.WorkspaceRoot(adapter.snapshot)
}
