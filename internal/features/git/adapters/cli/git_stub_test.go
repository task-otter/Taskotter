// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	stubModeEnv = "TASKOTTER_GIT_STUB_MODE"
	stubOK      = "ok"
	stubAbbrev  = "abbrev"
	stubNoRefs  = "norefs"
	stubBadRefs = "badrefs"
	stubShowOK  = "showok"
	stubShowBad = "showbad"
	stubScript  = `#!/bin/sh
mode="$TASKOTTER_GIT_STUB_MODE"
args="$*"
case "$mode" in
ok) exit 0 ;;
abbrev)
  case "$args" in
  *symbolic-ref*) exit 1 ;;
  *--abbrev-ref*) echo "origin/main"; exit 0 ;;
  *) exit 1 ;;
  esac ;;
badrefs)
  case "$args" in
  *symbolic-ref*|*--abbrev-ref*) exit 1 ;;
  *rev-parse\ refs/remotes/origin/HEAD*) echo "0123456789abcdef0123456789abcdef01234567"; exit 0 ;;
  *for-each-ref*) exit 1 ;;
  *) exit 1 ;;
  esac ;;
norefs)
  case "$args" in
  *symbolic-ref*|*--abbrev-ref*) exit 1 ;;
  *rev-parse\ refs/remotes/origin/HEAD*) echo "0123456789abcdef0123456789abcdef01234567"; exit 0 ;;
  *for-each-ref*) echo "origin/HEAD"; exit 0 ;;
  *) exit 1 ;;
  esac ;;
showok)
  case "$args" in
  *"remote show origin"*) echo "* remote origin"; echo "  HEAD branch: main"; exit 0 ;;
  *) exit 1 ;;
  esac ;;
showbad)
  case "$args" in
  *"remote show origin"*) echo "* remote origin"; exit 0 ;;
  *) exit 1 ;;
  esac ;;
*) exit 1 ;;
esac
`
)

// TestCommandsSucceedWithStubbedGit verifies the success paths of the mutating commands.
//
//nolint:paralleltest // swaps the package-level gitBinary seam
func TestCommandsSucceedWithStubbedGit(t *testing.T) {
	client := stubbedClient(t, stubOK)
	ctx := t.Context()

	failIfErr(t, client.CheckoutBranch(ctx, mainBranch))
	failIfErr(t, client.Commit(ctx, commitMsg))
	failIfErr(t, client.Push(ctx, mainBranch))
}

// TestDefaultBranchUsesAbbrevRef verifies the abbrev-ref fallback resolves the branch.
//
//nolint:paralleltest // swaps the package-level gitBinary seam
func TestDefaultBranchUsesAbbrevRef(t *testing.T) {
	assertDefaultBranch(t, stubAbbrev, mainBranch)
}

// TestDefaultBranchUsesRemoteShow verifies the remote show fallback resolves the branch.
//
//nolint:paralleltest // swaps the package-level gitBinary seam
func TestDefaultBranchUsesRemoteShow(t *testing.T) {
	assertDefaultBranch(t, stubShowOK, mainBranch)
}

// TestDefaultBranchReportsMissingHeadLine verifies remote show without a HEAD line fails.
//
//nolint:paralleltest // swaps the package-level gitBinary seam
func TestDefaultBranchReportsMissingHeadLine(t *testing.T) {
	client := stubbedClient(t, stubShowBad)

	branch, err := client.defaultBranchFromRemoteShow(t.Context())
	iox.Discard(branch)

	if !errors.Is(err, errHEADBranchNotFound) {
		t.Fatalf("err = %v, want %v", err, errHEADBranchNotFound)
	}
}

// TestOriginHeadCommitReportsRefListFailure verifies a failing ref listing is reported.
//
//nolint:paralleltest // swaps the package-level gitBinary seam
func TestOriginHeadCommitReportsRefListFailure(t *testing.T) {
	assertOriginHeadCommitFails(t, stubBadRefs)
}

// TestOriginHeadCommitReportsMissingBranch verifies origin HEAD without a branch is reported.
//
//nolint:paralleltest // swaps the package-level gitBinary seam
func TestOriginHeadCommitReportsMissingBranch(t *testing.T) {
	assertOriginHeadCommitFails(t, stubNoRefs)
}

func assertDefaultBranch(t *testing.T, mode, want string) {
	t.Helper()

	client := stubbedClient(t, mode)

	branch, err := client.DefaultBranch(t.Context())
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if branch != want {
		t.Fatalf(consts.BranchFmtErr, branch)
	}
}

func assertOriginHeadCommitFails(t *testing.T, mode string) {
	t.Helper()

	client := stubbedClient(t, mode)

	branch, err := client.defaultBranchFromOriginHEADCommit(t.Context())
	iox.Discard(branch)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func failIfErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

func stubbedClient(t *testing.T, mode string) *Client {
	t.Helper()
	t.Setenv(stubModeEnv, mode)

	path := filepath.Join(t.TempDir(), "git-stub.sh")

	err := os.WriteFile(path, []byte(stubScript), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	original := gitBinary

	gitBinary = path

	t.Cleanup(func() { gitBinary = original })

	return NewClient(t.TempDir())
}
