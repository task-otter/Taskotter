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
	"github.com/task-otter/Taskotter/internal/testsupport"
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
)

var errRefresh = errors.New("refresh failed")

// TestNoOpHelpers verifies the no-op compatibility helpers stay callable.
func TestNoOpHelpers(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)
	WriteLocalIdentity()
	NewClient(t.TempDir()).EnsureSafeDirectory()
}

// TestDefaultBranchFailureJoinsRefreshError verifies the refresh error is joined when present.
func TestDefaultBranchFailureJoinsRefreshError(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	if !errors.Is(defaultBranchFailure(errRefresh), errRefresh) {
		t.Fatal("refresh error should be joined")
	}

	if !errors.Is(defaultBranchFailure(nil), errDefaultBranchDetectionFailed) {
		t.Fatal("detection error expected")
	}
}

// TestFirstPlausibleRefRejectsOriginHead verifies origin/HEAD alone yields no branch.
func TestFirstPlausibleRefRejectsOriginHead(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

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

// TestFirstPlausibleRefFindsBranch verifies the first usable remote ref is returned.
func TestFirstPlausibleRefFindsBranch(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	branch, err := firstPlausibleRefOrError("origin/HEAD\norigin/main\n")
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if branch != mainBranch {
		t.Fatalf(consts.BranchFmtErr, branch)
	}
}

// TestIsChangeAllowedMatchesExactPath verifies an exact allow-list entry is honored.
func TestIsChangeAllowedMatchesExactPath(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	allowed := map[string]struct{}{allowedFile: {}}

	if !isChangeAllowed(allowedFile, allowed) {
		t.Fatal("exact path should be allowed")
	}

	if isChangeAllowed("other.txt", allowed) {
		t.Fatal("unrelated path should not be allowed")
	}
}

// TestHexHelpers verifies the hex classification helpers accept the expected ranges.
func TestHexHelpers(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	if !isHexString("0aF9") {
		t.Fatal("0aF9 should be hex")
	}

	if isHexString("0g") {
		t.Fatal("0g should not be hex")
	}
}

// TestIsPlausibleDefaultBranchRejectsCommitSHA verifies commit-like branch names are rejected.
func TestIsPlausibleDefaultBranchRejectsCommitSHA(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	if isPlausibleDefaultBranch("0123456789abcdef") {
		t.Fatal("commit sha should not be a plausible branch")
	}

	if !isPlausibleDefaultBranch(mainBranch) {
		t.Fatal("main should be a plausible branch")
	}
}

// TestParseHEADBranchLine verifies the remote show output parser.
func TestParseHEADBranchLine(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	branch, ok := parseHEADBranchLine("* remote origin\n  HEAD branch: main\n")

	if !ok || branch != mainBranch {
		t.Fatalf(consts.BranchFmtErr, branch)
	}

	branch, ok = parseHEADBranchLine("  HEAD branch: \nnothing\n")

	if ok {
		t.Fatalf("parseHEADBranchLine() = %q, want no branch", branch)
	}
}

// TestParseStatusPathRejectsShortLines verifies short and blank status lines are ignored.
func TestParseStatusPathRejectsShortLines(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	path, ok := parseStatusPath("M")

	if ok {
		t.Fatalf(noPathFmt, path)
	}

	path, ok = parseStatusPath(statusPrefix + "   ")

	if ok {
		t.Fatalf(noPathFmt, path)
	}
}

// TestValidateStagePathsRejectsEscape verifies staging paths outside the workspace fail.
func TestValidateStagePathsRejectsEscape(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	err := validateStagePaths(t.TempDir(), []string{escapePath})
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestClientRejectsInvalidRefs verifies every ref-taking method validates its input.
func TestClientRejectsInvalidRefs(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	client := NewClient(t.TempDir())
	ctx := t.Context()

	assertRefRejected(t, client.CheckoutBranch(ctx, badRef))
	assertRefRejected(t, client.CreateOrResetBranch(ctx, badRef))
	assertRefRejected(t, client.Push(ctx, badRef))
	assertRefRejected(t, client.PushForceWithLease(ctx, badRef))
}

// TestBranchExistsRejectsInvalidRef verifies an invalid ref is reported before running git.
func TestBranchExistsRejectsInvalidRef(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	exists, err := NewClient(t.TempDir()).BranchExists(t.Context(), badRef)
	iox.Discard(exists)
	assertRefRejected(t, err)
}

// TestLastCommitMessageRejectsInvalidRef verifies an invalid ref is reported before running git.
func TestLastCommitMessageRejectsInvalidRef(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	msg, err := NewClient(t.TempDir()).LastCommitMessage(t.Context(), badRef)
	iox.Discard(msg)
	assertRefRejected(t, err)
}

// TestClientMethodsReportGitFailures verifies commands fail outside a repository.
func TestClientMethodsReportGitFailures(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	client := NewClient(t.TempDir())
	ctx := t.Context()

	assertFails(t, client.CheckoutBranch(ctx, mainBranch))
	assertFails(t, client.CreateOrResetBranch(ctx, mainBranch))
	assertFails(t, client.Commit(ctx, commitMsg))
	assertFails(t, client.Push(ctx, mainBranch))
	assertFails(t, client.PushForceWithLease(ctx, mainBranch))
}

// TestLastCommitMessageReportsGitFailure verifies git log failures are wrapped.
func TestLastCommitMessageReportsGitFailure(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	msg, err := NewClient(t.TempDir()).LastCommitMessage(t.Context(), mainBranch)
	iox.Discard(msg)
	assertFails(t, err)
}

// TestHasUnrelatedChangesReportsGitFailure verifies git status failures are wrapped.
func TestHasUnrelatedChangesReportsGitFailure(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	changed, err := NewClient(t.TempDir()).HasUnrelatedChanges(t.Context(), nil)
	iox.Discard(changed)
	assertFails(t, err)
}

// TestDefaultBranchReportsDetectionFailure verifies detection failure outside a repository.
func TestDefaultBranchReportsDetectionFailure(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	branch, err := NewClient(t.TempDir()).DefaultBranch(t.Context())
	iox.Discard(branch)

	if !errors.Is(err, errDefaultBranchDetectionFailed) {
		t.Fatalf(errFmt, err)
	}
}

// TestDefaultBranchFromRemoteShowReportsFailure verifies git remote show failures are wrapped.
func TestDefaultBranchFromRemoteShowReportsFailure(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	branch, err := defaultBranchFromRemoteShow(t.Context(), NewClient(t.TempDir()))
	iox.Discard(branch)
	assertFails(t, err)
}

// TestRefsAtOriginHEADReportsFailure verifies for-each-ref failures are wrapped.
func TestRefsAtOriginHEADReportsFailure(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	refs, err := refsAtOriginHEAD(t.Context(), NewClient(t.TempDir()), "deadbeef")
	iox.Discard(refs)
	assertFails(t, err)
}

// TestStageSkipsEmptyPaths verifies staging nothing is a no-op.
func TestStageSkipsEmptyPaths(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	err := NewClient(t.TempDir()).Stage(t.Context(), nil)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

// TestStageReportsInvalidPath verifies an escaping stage path is rejected.
func TestStageReportsInvalidPath(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	err := NewClient(t.TempDir()).Stage(t.Context(), []string{escapePath})
	assertFails(t, err)
}

// TestStageReportsGitFailure verifies git add failures outside a repository are wrapped.
func TestStageReportsGitFailure(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

	workspace := t.TempDir()
	writeStageFixture(t, workspace)

	err := NewClient(workspace).Stage(t.Context(), []string{stageFile})
	assertFails(t, err)
}

// TestConfigureCredentialsReportsRemoteFailure verifies remote set-url failures are wrapped.
func TestConfigureCredentialsReportsRemoteFailure(t *testing.T) {
	t.Parallel()
	lockGitTestState(t)

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

func lockGitTestState(t *testing.T) {
	t.Helper()
	t.Cleanup(testsupport.Lock())
}

// TestCommandsSucceedWithStubbedGit verifies the success paths of the mutating commands.
func TestCommandsSucceedWithStubbedGit(t *testing.T) {
	lockGitTestState(t)

	client := stubbedClient(t, stubOK)
	ctx := t.Context()

	failIfErr(t, client.CheckoutBranch(ctx, mainBranch))
	failIfErr(t, client.Commit(ctx, commitMsg))
	failIfErr(t, client.Push(ctx, mainBranch))
}

// TestDefaultBranchUsesAbbrevRef verifies the abbrev-ref fallback resolves the branch.
func TestDefaultBranchUsesAbbrevRef(t *testing.T) {
	lockGitTestState(t)
	assertDefaultBranch(t, stubAbbrev, mainBranch)
}

// TestDefaultBranchUsesRemoteShow verifies the remote show fallback resolves the branch.
func TestDefaultBranchUsesRemoteShow(t *testing.T) {
	lockGitTestState(t)
	assertDefaultBranch(t, stubShowOK, mainBranch)
}

// TestDefaultBranchReportsMissingHeadLine verifies remote show without a HEAD line fails.
func TestDefaultBranchReportsMissingHeadLine(t *testing.T) {
	lockGitTestState(t)

	client := stubbedClient(t, stubShowBad)

	branch, err := defaultBranchFromRemoteShow(t.Context(), client)
	iox.Discard(branch)

	if !errors.Is(err, errHEADBranchNotFound) {
		t.Fatalf("err = %v, want %v", err, errHEADBranchNotFound)
	}
}

// TestOriginHeadCommitReportsRefListFailure verifies a failing ref listing is reported.
func TestOriginHeadCommitReportsRefListFailure(t *testing.T) {
	lockGitTestState(t)
	assertOriginHeadCommitFails(t, stubBadRefs)
}

// TestOriginHeadCommitReportsMissingBranch verifies origin HEAD without a branch is reported.
func TestOriginHeadCommitReportsMissingBranch(t *testing.T) {
	lockGitTestState(t)
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

	return newClient(t.TempDir(), path)
}
