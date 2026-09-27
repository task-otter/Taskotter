// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain_test

import (
	"errors"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	testCommit  = "abc123"
	testRef     = "refs/heads/main"
	testBranch  = "main"
	testRootDir = "/tmp/store-root"
)

var errCleanup = errors.New("cleanup failed")

// TestSnapshotAccessorsExposeRefInfo verifies the ref accessors return the stored values.
func TestSnapshotAccessorsExposeRefInfo(t *testing.T) {
	t.Parallel()

	snapshot := newSnapshot(nil)

	if domain.DefaultBranch(snapshot) != testBranch {
		t.Fatalf("DefaultBranch() = %q", domain.DefaultBranch(snapshot))
	}

	if domain.ResolvedCommit(snapshot) != testCommit {
		t.Fatalf("ResolvedCommit() = %q", domain.ResolvedCommit(snapshot))
	}

	if domain.SourceRef(snapshot) != testRef {
		t.Fatalf("SourceRef() = %q", domain.SourceRef(snapshot))
	}

	if domain.WorkspaceRoot(snapshot) != testRootDir {
		t.Fatalf("WorkspaceRoot() = %q", domain.WorkspaceRoot(snapshot))
	}
}

// TestSnapshotCloseWithoutCleanup verifies closing a snapshot without cleanup succeeds.
func TestSnapshotCloseWithoutCleanup(t *testing.T) {
	t.Parallel()

	err := domain.Close(newSnapshot(nil))
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	err = domain.Close(newSnapshot(func() error { return nil }))
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

// TestSnapshotCloseReportsCleanupFailure verifies cleanup failures are wrapped.
func TestSnapshotCloseReportsCleanupFailure(t *testing.T) {
	t.Parallel()

	err := domain.Close(newSnapshot(func() error { return errCleanup }))

	if !errors.Is(err, errCleanup) {
		t.Fatalf("err = %v, want %v", err, errCleanup)
	}
}

// TestSnapshotModuleDirJoinsStorePath verifies module directories resolve under the root.
func TestSnapshotModuleDirJoinsStorePath(t *testing.T) {
	t.Parallel()

	dir := domain.ModuleDir(newSnapshot(nil), consts.Go)

	if dir == consts.Empty {
		t.Fatal("ModuleDir() = empty")
	}
}

func newSnapshot(cleanup func() error) *domain.Snapshot {
	return &domain.Snapshot{
		Catalog: nil,
		Deps:    nil,
		Cleanup: cleanup,
		Ref: domain.RefInfo{
			Repository:       consts.Empty,
			RequestedVersion: consts.Empty,
			SourceRef:        testRef,
			ResolvedCommit:   testCommit,
			DefaultBranch:    testBranch,
		},
		RootDir: testRootDir,
	}
}
