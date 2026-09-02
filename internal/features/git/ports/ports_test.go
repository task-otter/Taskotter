// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/git/ports"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	stubChecker struct {
		branchErr   error
		messageErr  error
		lastMessage string
		exists      bool
	}
)

const (
	syncBranch  = "taskotter/sync-abc"
	wantErrText = "expected error"
)

var errStub = errors.New("stub failure")

// TestAllowedPathSetNormalizesSeparators verifies staged paths become slash-separated keys.
func TestAllowedPathSetNormalizesSeparators(t *testing.T) {
	t.Parallel()

	set := ports.AllowedPathSet([]string{filepath.Join("taskfiles", "go", "Taskfile.yml")})

	if _, ok := set["taskfiles/go/Taskfile.yml"]; !ok {
		t.Fatalf("set = %#v", set)
	}
}

// TestEnsureBranchOwnedAllowsNewBranch verifies a missing branch is always allowed.
func TestEnsureBranchOwnedAllowsNewBranch(t *testing.T) {
	t.Parallel()

	err := ports.EnsureBranchOwned(t.Context(), newChecker(false, consts.Empty), syncBranch)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

// TestEnsureBranchOwnedAllowsOwnedBranch verifies a TaskOtter-authored branch is reused.
func TestEnsureBranchOwnedAllowsOwnedBranch(t *testing.T) {
	t.Parallel()

	checker := newChecker(true, ports.SyncCommitMessage)

	err := ports.EnsureBranchOwned(t.Context(), checker, syncBranch)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

// TestEnsureBranchOwnedRejectsForeignBranch verifies a foreign branch is rejected.
func TestEnsureBranchOwnedRejectsForeignBranch(t *testing.T) {
	t.Parallel()

	err := ports.EnsureBranchOwned(t.Context(), newChecker(true, "other work"), syncBranch)
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestEnsureBranchOwnedWrapsCheckerErrors verifies lookup failures are reported.
func TestEnsureBranchOwnedWrapsCheckerErrors(t *testing.T) {
	t.Parallel()

	cases := []*stubChecker{
		{branchErr: errStub, messageErr: nil, lastMessage: consts.Empty, exists: false},
		{branchErr: nil, messageErr: errStub, lastMessage: consts.Empty, exists: true},
	}

	for i := range cases {
		err := ports.EnsureBranchOwned(t.Context(), cases[i], syncBranch)

		if !errors.Is(err, errStub) {
			t.Fatalf("err = %v, want %v", err, errStub)
		}
	}
}

// TestIsGitRepoDetectsGitDirectory verifies .git detection for both outcomes.
func TestIsGitRepoDetectsGitDirectory(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	if ports.IsGitRepo(workspace) {
		t.Fatal("empty directory should not be a git repo")
	}

	err := os.Mkdir(filepath.Join(workspace, consts.GitDir), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	if !ports.IsGitRepo(workspace) {
		t.Fatal("directory with .git should be a git repo")
	}
}

// TestWriteLocalIdentityIsCallable verifies the no-op identity helper stays callable.
func TestWriteLocalIdentityIsCallable(t *testing.T) {
	t.Parallel()
	ports.WriteLocalIdentity()
}

func (checker *stubChecker) BranchExists(context.Context, string) (bool, error) {
	if checker.branchErr != nil {
		return false, checker.branchErr
	}

	return checker.exists, nil
}

func (checker *stubChecker) LastCommitMessage(context.Context, string) (string, error) {
	if checker.messageErr != nil {
		return consts.Empty, checker.messageErr
	}

	return checker.lastMessage, nil
}

func newChecker(exists bool, message string) *stubChecker {
	return &stubChecker{
		branchErr:   nil,
		messageErr:  nil,
		lastMessage: message,
		exists:      exists,
	}
}
