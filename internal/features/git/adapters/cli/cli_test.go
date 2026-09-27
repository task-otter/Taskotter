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
	wantErrText  = "expected error"
	badRef       = "bad ref"
	mainBranch   = "main"
	statusPrefix = " M "
	errFmt       = "err = %v"
	allowedFile  = "taskfiles/Taskfile.yml"
	noPathFmt    = "parseStatusPath() = %q, want no path"
	escapePath   = "../escape"
	commitMsg    = "message"
	stageFile    = "file.txt"
	stubModeEnv  = "TASKOTTER_GIT_STUB_MODE"
	stubOK       = "ok"
	stubAbbrev   = "abbrev"
	stubNoRefs   = "norefs"
	stubBadRefs  = "badrefs"
	stubShowOK   = "showok"
	stubShowBad  = "showbad"
	stubScript   = `#!/bin/sh
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

var errRefresh = errors.New("refresh failed")

// TestNoOpHelpers verifies the expected behavior.
func TestNoOpHelpers(t *testing.T) {
	t.Parallel()
	WriteLocalIdentity()
	NewClient(t.TempDir()).EnsureSafeDirectory()
}

// TestDefaultBranchFailureJoinsRefreshError verifies the expected behavior.
func TestDefaultBranchFailureJoinsRefreshError(t *testing.T) {
	t.Parallel()

	if !errors.Is(defaultBranchFailure(errRefresh), errRefresh) {
		t.Fatal("refresh error should be joined")
	}

	if !errors.Is(defaultBranchFailure(nil), errDefaultBranchDetectionFailed) {
		t.Fatal("detection error expected")
	}
}

// TestFirstPlausibleRefRejectsOriginHead verifies the expected behavior.
func TestFirstPlausibleRefRejectsOriginHead(t *testing.T) {
	t.Parallel()

	branch, ok := firstPlausibleRef("origin/HEAD\n\n")

	if ok {
		t.Fatalf("firstPlausibleRef() = %q, want no branch", branch)
	}

	branch, err := firstPlausibleRefOrError(originHEADShortRef)
	iox.Discard(branch)

	if !errors.Is(err, errNoRemoteBranchAtOriginHEAD) {
		t.Fatalf(errFmt, err)
	}
}

// TestFirstPlausibleRefFindsBranch verifies the expected behavior.
func TestFirstPlausibleRefFindsBranch(t *testing.T) {
	t.Parallel()

	branch, err := firstPlausibleRefOrError("origin/HEAD\norigin/main\n")
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if branch != mainBranch {
		t.Fatalf(consts.BranchFmtErr, branch)
	}
}

// TestIsChangeAllowedMatchesExactPath verifies the expected behavior.
func TestIsChangeAllowedMatchesExactPath(t *testing.T) {
	t.Parallel()

	allowed := map[string]struct{}{allowedFile: {}}

	if !isChangeAllowed(allowedFile, allowed) {
		t.Fatal("exact path should be allowed")
	}

	if isChangeAllowed("other.txt", allowed) {
		t.Fatal("unrelated path should not be allowed")
	}
}

// TestHexHelpers verifies the expected behavior.
func TestHexHelpers(t *testing.T) {
	t.Parallel()

	if !isHexString("0aF9") {
		t.Fatal("0aF9 should be hex")
	}

	if isHexString("0g") {
		t.Fatal("0g should not be hex")
	}
}

// TestIsPlausibleDefaultBranchRejectsCommitSHA verifies the expected behavior.
func TestIsPlausibleDefaultBranchRejectsCommitSHA(t *testing.T) {
	t.Parallel()

	if isPlausibleDefaultBranch("0123456789abcdef") {
		t.Fatal("commit sha should not be a plausible branch")
	}

	if !isPlausibleDefaultBranch(mainBranch) {
		t.Fatal("main should be a plausible branch")
	}
}

// TestParseHEADBranchLine verifies the expected behavior.
func TestParseHEADBranchLine(t *testing.T) {
	t.Parallel()

	branch, ok := parseHEADBranchLine("* remote origin\n  HEAD branch: main\n")

	if !ok || branch != mainBranch {
		t.Fatalf(consts.BranchFmtErr, branch)
	}

	branch, ok = parseHEADBranchLine("  HEAD branch: \nnothing\n")

	if ok {
		t.Fatalf("parseHEADBranchLine() = %q, want no branch", branch)
	}
}

// TestParseStatusPathRejectsShortLines verifies the expected behavior.
func TestParseStatusPathRejectsShortLines(t *testing.T) {
	t.Parallel()

	path, ok := parseStatusPath("M")

	if ok {
		t.Fatalf(noPathFmt, path)
	}

	path, ok = parseStatusPath(statusPrefix + "   ")

	if ok {
		t.Fatalf(noPathFmt, path)
	}
}

// TestValidateStagePathsRejectsEscape verifies the expected behavior.
func TestValidateStagePathsRejectsEscape(t *testing.T) {
	t.Parallel()

	err := validateStagePaths(t.TempDir(), []string{escapePath})
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestClientRejectsInvalidRefs verifies the expected behavior.
func TestClientRejectsInvalidRefs(t *testing.T) {
	t.Parallel()

	client := NewClient(t.TempDir())
	ctx := t.Context()

	assertRefRejected(t, client.CheckoutBranch(ctx, badRef))
	assertRefRejected(t, client.CreateOrResetBranch(ctx, badRef))
	assertRefRejected(t, client.Push(ctx, badRef))
	assertRefRejected(t, client.PushForceWithLease(ctx, badRef))
}

// TestBranchExistsRejectsInvalidRef verifies the expected behavior.
func TestBranchExistsRejectsInvalidRef(t *testing.T) {
	t.Parallel()

	exists, err := NewClient(t.TempDir()).BranchExists(t.Context(), badRef)
	iox.Discard(exists)
	assertRefRejected(t, err)
}

// TestLastCommitMessageRejectsInvalidRef verifies the expected behavior.
func TestLastCommitMessageRejectsInvalidRef(t *testing.T) {
	t.Parallel()

	msg, err := NewClient(t.TempDir()).LastCommitMessage(t.Context(), badRef)
	iox.Discard(msg)
	assertRefRejected(t, err)
}

// TestClientMethodsReportGitFailures verifies the expected behavior.
func TestClientMethodsReportGitFailures(t *testing.T) {
	t.Parallel()

	client := NewClient(t.TempDir())
	ctx := t.Context()

	assertFails(t, client.CheckoutBranch(ctx, mainBranch))
	assertFails(t, client.CreateOrResetBranch(ctx, mainBranch))
	assertFails(t, client.Commit(ctx, commitMsg))
	assertFails(t, client.Push(ctx, mainBranch))
	assertFails(t, client.PushForceWithLease(ctx, mainBranch))
}

// TestLastCommitMessageReportsGitFailure verifies the expected behavior.
func TestLastCommitMessageReportsGitFailure(t *testing.T) {
	t.Parallel()

	msg, err := NewClient(t.TempDir()).LastCommitMessage(t.Context(), mainBranch)
	iox.Discard(msg)
	assertFails(t, err)
}

// TestHasUnrelatedChangesReportsGitFailure verifies the expected behavior.
func TestHasUnrelatedChangesReportsGitFailure(t *testing.T) {
	t.Parallel()

	changed, err := NewClient(t.TempDir()).HasUnrelatedChanges(t.Context(), nil)
	iox.Discard(changed)
	assertFails(t, err)
}

// TestDefaultBranchReportsDetectionFailure verifies the expected behavior.
func TestDefaultBranchReportsDetectionFailure(t *testing.T) {
	t.Parallel()

	branch, err := NewClient(t.TempDir()).DefaultBranch(t.Context())
	iox.Discard(branch)

	if !errors.Is(err, errDefaultBranchDetectionFailed) {
		t.Fatalf(errFmt, err)
	}
}

// TestDefaultBranchFromRemoteShowReportsFailure verifies the expected behavior.
func TestDefaultBranchFromRemoteShowReportsFailure(t *testing.T) {
	t.Parallel()

	branch, err := defaultBranchFromRemoteShow(t.Context(), NewClient(t.TempDir()))
	iox.Discard(branch)
	assertFails(t, err)
}

// TestRefsAtOriginHEADReportsFailure verifies the expected behavior.
func TestRefsAtOriginHEADReportsFailure(t *testing.T) {
	t.Parallel()

	refs, err := refsAtOriginHEAD(t.Context(), NewClient(t.TempDir()), "deadbeef")
	iox.Discard(refs)
	assertFails(t, err)
}

// TestStageSkipsEmptyPaths verifies the expected behavior.
func TestStageSkipsEmptyPaths(t *testing.T) {
	t.Parallel()

	err := NewClient(t.TempDir()).Stage(t.Context(), nil)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

// TestStageReportsInvalidPath verifies the expected behavior.
func TestStageReportsInvalidPath(t *testing.T) {
	t.Parallel()

	err := NewClient(t.TempDir()).Stage(t.Context(), []string{escapePath})
	assertFails(t, err)
}

// TestStageReportsGitFailure verifies the expected behavior.
func TestStageReportsGitFailure(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeStageFixture(t, workspace)

	err := NewClient(workspace).Stage(t.Context(), []string{stageFile})
	assertFails(t, err)
}

// TestConfigureCredentialsReportsRemoteFailure verifies the expected behavior.
func TestConfigureCredentialsReportsRemoteFailure(t *testing.T) {
	t.Parallel()

	err := NewClient(t.TempDir()).ConfigureCredentials(t.Context(), "token", "owner/repo")
	assertFails(t, err)
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func assertRefRejected(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected ref validation error")
	}
}

func writeStageFixture(t *testing.T, workspace string) {
	t.Helper()

	err := os.WriteFile(filepath.Join(workspace, stageFile), []byte("data"), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}
}

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

	branch, err := defaultBranchFromRemoteShow(t.Context(), client)
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

	branch, err := defaultBranchFromOriginHEADCommit(t.Context(), client)
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
