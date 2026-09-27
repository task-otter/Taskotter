// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package snapshot

import (
	"path/filepath"
	"testing"

	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
)

const snapshotLintModule = "lint"

// TestAdapterReturnsSnapshotValues verifies the behavior covered by this test.
func TestAdapterReturnsSnapshotValues(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	snap := &storedomain.Snapshot{
		RootDir: root,
		Ref: storedomain.RefInfo{
			DefaultBranch:  "main",
			ResolvedCommit: "abc123",
			SourceRef:      "refs/heads/main",
		},
	}

	adapter := New(snap)

	got := []string{
		adapter.DefaultBranch(),
		adapter.ResolvedCommit(),
		adapter.SourceRef(),
		adapter.WorkspaceRoot(),
		adapter.ModuleDir(snapshotLintModule),
	}

	wantModuleDir := filepath.Join(root, "taskfiles", snapshotLintModule)
	want := []string{
		snap.Ref.DefaultBranch,
		snap.Ref.ResolvedCommit,
		snap.Ref.SourceRef,
		root,
		wantModuleDir,
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
