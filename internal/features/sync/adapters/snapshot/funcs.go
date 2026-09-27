// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package snapshot

import (
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
)

// DefaultBranch implements ports.Snapshot.
func (port *Adapter) DefaultBranch() string {
	return storedomain.DefaultBranch(port.snap)
}

// ModuleDir implements ports.Snapshot.
func (port *Adapter) ModuleDir(sourceModule string) string {
	return storedomain.ModuleDir(port.snap, sourceModule)
}

// ResolvedCommit implements ports.Snapshot.
func (port *Adapter) ResolvedCommit() string {
	return storedomain.ResolvedCommit(port.snap)
}

// SourceRef implements ports.Snapshot.
func (port *Adapter) SourceRef() string {
	return storedomain.SourceRef(port.snap)
}

// WorkspaceRoot implements ports.Snapshot.
func (port *Adapter) WorkspaceRoot() string {
	return storedomain.WorkspaceRoot(port.snap)
}

// New adapts snap for PrepareSyncInput and related sync ports.
func New(snap *storedomain.Snapshot) *Adapter {
	return &Adapter{snap: snap}
}
