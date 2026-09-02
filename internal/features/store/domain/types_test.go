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

	if snapshot.DefaultBranch() != testBranch {
		t.Fatalf("DefaultBranch() = %q", snapshot.DefaultBranch())
	}

	if snapshot.ResolvedCommit() != testCommit {
		t.Fatalf("ResolvedCommit() = %q", snapshot.ResolvedCommit())
	}

	if snapshot.SourceRef() != testRef {
		t.Fatalf("SourceRef() = %q", snapshot.SourceRef())
	}

	if snapshot.WorkspaceRoot() != testRootDir {
		t.Fatalf("WorkspaceRoot() = %q", snapshot.WorkspaceRoot())
	}
}

// TestSnapshotCloseWithoutCleanup verifies closing a snapshot without cleanup succeeds.
func TestSnapshotCloseWithoutCleanup(t *testing.T) {
	t.Parallel()

	err := newSnapshot(nil).Close()
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	err = newSnapshot(func() error { return nil }).Close()
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

// TestSnapshotCloseReportsCleanupFailure verifies cleanup failures are wrapped.
func TestSnapshotCloseReportsCleanupFailure(t *testing.T) {
	t.Parallel()

	err := newSnapshot(func() error { return errCleanup }).Close()

	if !errors.Is(err, errCleanup) {
		t.Fatalf("err = %v, want %v", err, errCleanup)
	}
}

// TestSnapshotModuleDirJoinsStorePath verifies module directories resolve under the root.
func TestSnapshotModuleDirJoinsStorePath(t *testing.T) {
	t.Parallel()

	dir := newSnapshot(nil).ModuleDir(consts.Go)

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
