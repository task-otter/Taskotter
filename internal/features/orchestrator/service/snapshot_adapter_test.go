// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/orchestrator/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
)

const (
	testModuleLint = "lint"
)

// TestSnapshotAdapterReturnsSnapshotValues verifies the adapter delegates every snapshot property.
func TestSnapshotAdapterReturnsSnapshotValues(t *testing.T) {
	t.Parallel()

	snapshot := testSnapshot(t)
	adapter := service.NewSnapshotAdapter(snapshot)
	assertSnapshotValues(t, adapter, snapshot)
}

func testSnapshot(t *testing.T) *storedomain.Snapshot {
	t.Helper()

	root := t.TempDir()

	return &storedomain.Snapshot{RootDir: root, Ref: storedomain.RefInfo{
		DefaultBranch: "main", ResolvedCommit: "abc123", SourceRef: "refs/heads/main",
	}}
}

func assertSnapshotValues(
	t *testing.T,
	adapter *service.SnapshotAdapter,
	snapshot *storedomain.Snapshot,
) {
	t.Helper()

	testCases := snapshotCases(adapter, snapshot, snapshot.RootDir)
	assertSnapshotCases(t, testCases)
}

func snapshotCases(
	adapter *service.SnapshotAdapter,
	snapshot *storedomain.Snapshot,
	root string,
) []struct {
	name string
	got  string
	want string
} {
	return []struct {
		name string
		got  string
		want string
	}{
		{"default branch", adapter.DefaultBranch(), snapshot.Ref.DefaultBranch},
		{"resolved commit", adapter.ResolvedCommit(), snapshot.Ref.ResolvedCommit},
		{"source ref", adapter.SourceRef(), snapshot.Ref.SourceRef},
		{"workspace root", adapter.WorkspaceRoot(), root},
		moduleDirCase(adapter, root),
	}
}

func moduleDirCase(adapter *service.SnapshotAdapter, root string) struct {
	name string
	got  string
	want string
} {
	return struct {
		name string
		got  string
		want string
	}{"module dir", adapter.ModuleDir(testModuleLint), filepath.Join(root, "taskfiles", testModuleLint)}
}

func assertSnapshotCases(t *testing.T, testCases []struct {
	name string
	got  string
	want string
},
) {
	t.Helper()

	for idx := range testCases {
		testCase := &testCases[idx]

		if testCase.got != testCase.want {
			t.Fatalf("%s = %q, want %q", testCase.name, testCase.got, testCase.want)
		}
	}
}
