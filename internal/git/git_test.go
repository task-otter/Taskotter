// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/git"
)

type mockGitOps struct {
	branchErr    error
	messageErr   error
	lastMessage  string
	branchExists bool
}

const (
	testMainBranch    = "main"
	testFeatureBranch = "feature/test"
	testSyncBranch    = "taskotter/sync-abc"
	dirClone          = "clone"
	gitSubcommandInit = "init"
	gitCmdCommit      = "commit"
	gitCmdRemote      = "remote"
	gitCmdConfig      = "config"
	gitCmdAdd         = "add"
	flagM             = "-m"
	flagB             = "-b"
	configUserEmail   = "user.email"
	configUserName    = "user.name"
	testGitEmail      = "test@test.com"
	testGitUserName   = "Test"
	pathRefs          = "refs"
	pathRemotes       = "remotes"
	pathHEAD          = "HEAD"
	fileGitignore     = ".gitignore"
	pathMetadataYML   = "taskfiles/.taskotter/metadata.yml"
	testTaskfilesDir  = "taskfiles"
	fmtBranchWantMain = "branch = %q, want main"
)

func (mockGitOps *mockGitOps) EnsureSafeDirectory(context.Context) error { return nil }

func (mockGitOps *mockGitOps) HasUnrelatedChanges(
	context.Context,
	map[string]struct{},
) (bool, error) {
	return false, nil
}
func (mockGitOps *mockGitOps) CheckoutBranch(context.Context, string, bool) error { return nil }
func (mockGitOps *mockGitOps) BranchExists(context.Context, string) (bool, error) {
	if mockGitOps.branchErr != nil {
		return false, mockGitOps.branchErr
	}

	return mockGitOps.branchExists, nil
}

func (mockGitOps *mockGitOps) LastCommitMessage(context.Context, string) (string, error) {
	if mockGitOps.messageErr != nil {
		return consts.Empty, mockGitOps.messageErr
	}

	return mockGitOps.lastMessage, nil
}
func (mockGitOps *mockGitOps) Stage(context.Context, []string) error    { return nil }
func (mockGitOps *mockGitOps) Commit(context.Context, string) error     { return nil }
func (mockGitOps *mockGitOps) Push(context.Context, string, bool) error { return nil }
func (mockGitOps *mockGitOps) DefaultBranch(context.Context) (string, error) {
	return testMainBranch, nil
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)

	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}

	return string(out)
}

func setupRemoteRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	bareDir := filepath.Join(root, "bare.git")
	cloneDir := filepath.Join(root, dirClone)

	err := os.MkdirAll(bareDir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(cloneDir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, bareDir, gitSubcommandInit, "--bare", flagB, testMainBranch)
	runGit(t, cloneDir, gitSubcommandInit, flagB, testMainBranch)
	runGit(t, cloneDir, gitCmdConfig, configUserEmail, testGitEmail)
	runGit(t, cloneDir, gitCmdConfig, configUserName, testGitUserName)

	err = os.WriteFile(
		filepath.Join(cloneDir, consts.ReadmeMD),
		[]byte(gitSubcommandInit+"\n"),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, gitCmdAdd, consts.ReadmeMD)
	runGit(t, cloneDir, gitCmdCommit, flagM, gitSubcommandInit)
	runGit(t, cloneDir, gitCmdRemote, gitCmdAdd, consts.GitOrigin, bareDir)
	runGit(t, cloneDir, "push", "-u", consts.GitOrigin, testMainBranch)
	runGit(t, cloneDir, "fetch", consts.GitOrigin)

	return cloneDir
}

// TestDefaultBranchSymbolicRef verifies the default branch resolves via a symbolic HEAD ref.
func TestDefaultBranchSymbolicRef(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	runGit(t, cloneDir, gitCmdRemote, "set-head", consts.GitOrigin, "-a")

	client := git.NewClient(cloneDir)

	branch, err := client.DefaultBranch(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if branch != testMainBranch {
		t.Fatalf(fmtBranchWantMain, branch)
	}
}

// TestDefaultBranchMissingOriginHEAD verifies the default branch still resolves when origin/HEAD is missing.
func TestDefaultBranchMissingOriginHEAD(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)

	originHEAD := filepath.Join(
		cloneDir,
		consts.GitDir,
		pathRefs,
		pathRemotes,
		consts.GitOrigin,
		pathHEAD,
	)

	err := os.Remove(originHEAD)

	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	client := git.NewClient(cloneDir)

	branch, err := client.DefaultBranch(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if branch != testMainBranch {
		t.Fatalf(fmtBranchWantMain, branch)
	}
}

// TestDefaultBranchDirectRef verifies the default branch resolves when origin/HEAD is a direct SHA.
func TestDefaultBranchDirectRef(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	mainSHA := strings.TrimSpace(runGit(t, cloneDir, "rev-parse", "refs/remotes/origin/main"))

	originHEAD := filepath.Join(
		cloneDir,
		consts.GitDir,
		pathRefs,
		pathRemotes,
		consts.GitOrigin,
		pathHEAD,
	)

	err := os.WriteFile(originHEAD, []byte(mainSHA+"\n"), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	client := git.NewClient(cloneDir)

	branch, err := client.DefaultBranch(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if branch != testMainBranch {
		t.Fatalf(fmtBranchWantMain, branch)
	}
}

// TestStageForceAddsGitignoredMetadata verifies gitignored metadata files are force-staged.
func TestStageForceAddsGitignoredMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cloneDir := filepath.Join(root, dirClone)

	err := os.MkdirAll(cloneDir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, gitSubcommandInit, flagB, testMainBranch)
	runGit(t, cloneDir, gitCmdConfig, configUserEmail, testGitEmail)
	runGit(t, cloneDir, gitCmdConfig, configUserName, testGitUserName)

	err = os.WriteFile(
		filepath.Join(cloneDir, fileGitignore),
		[]byte(".taskotter/\n"),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(filepath.Join(cloneDir, "taskfiles", ".taskotter"), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(cloneDir, pathMetadataYML),
		[]byte("target_folder: taskfiles\n"),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(cloneDir, consts.ReadmeMD),
		[]byte(gitSubcommandInit+"\n"),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, gitCmdAdd, consts.ReadmeMD, fileGitignore)
	runGit(t, cloneDir, gitCmdCommit, flagM, gitSubcommandInit)

	client := git.NewClient(cloneDir)

	err = client.Stage(t.Context(), []string{pathMetadataYML})
	if err != nil {
		t.Fatal(err)
	}

	out := runGit(t, cloneDir, "status", "--porcelain")

	if !strings.Contains(out, pathMetadataYML) {
		t.Fatalf("expected staged metadata, got status:\n%s", out)
	}
}

// TestClientHelpers verifies safe directory setup, identity config, and repo detection.
func TestClientHelpers(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	client := git.NewClient(cloneDir)
	ctx := t.Context()

	err := client.EnsureSafeDirectory(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = git.WriteLocalIdentity(ctx, client)
	if err != nil {
		t.Fatal(err)
	}

	if !git.IsGitRepo(cloneDir) {
		t.Fatal("expected temp checkout to be a git repo")
	}

	if git.IsGitRepo(t.TempDir()) {
		t.Fatal("plain temp dir is not a git repo")
	}
}

// TestClientBranchMethods verifies branch checkout, existence checks, and last commit message.
func TestClientBranchMethods(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	client := git.NewClient(cloneDir)
	ctx := t.Context()

	err := client.CheckoutBranch(ctx, testFeatureBranch, true)
	if err != nil {
		t.Fatal(err)
	}

	exists, err := client.BranchExists(ctx, testFeatureBranch)
	if err != nil {
		t.Fatal(err)
	}

	if !exists {
		t.Fatal("expected created branch to exist")
	}

	exists, err = client.BranchExists(ctx, "missing-branch")
	if err != nil {
		t.Fatal(err)
	}

	if exists {
		t.Fatal("missing branch should not exist")
	}

	msg, err := client.LastCommitMessage(ctx, testMainBranch)

	if err != nil || msg != gitSubcommandInit {
		t.Fatalf("LastCommitMessage() = %q, %v; want init, nil", msg, err)
	}
}

// TestClientCommitAndPush verifies committing and pushing a feature branch succeeds.
func TestClientCommitAndPush(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	client := git.NewClient(cloneDir)
	ctx := t.Context()

	err := client.CheckoutBranch(ctx, testFeatureBranch, true)
	if err != nil {
		t.Fatal(err)
	}

	err = client.Commit(ctx, "nothing to commit")
	if err != nil {
		t.Fatal(err)
	}

	err = client.Push(ctx, testFeatureBranch, true)
	if err != nil {
		t.Fatal(err)
	}
}

// TestHasUnrelatedChanges verifies changes outside allowed paths are flagged as unrelated.
func TestHasUnrelatedChanges(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	client := git.NewClient(cloneDir)

	err := os.MkdirAll(filepath.Join(cloneDir, testTaskfilesDir, consts.Go), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(cloneDir, testTaskfilesDir, consts.Go, consts.Taskfile),
		[]byte("version: '3'\n"),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	unrelated, err := client.HasUnrelatedChanges(
		t.Context(),
		git.AllowedPathSet([]string{testTaskfilesDir}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if unrelated {
		t.Fatal("changes under allowed folder should not be unrelated")
	}

	err = os.WriteFile(filepath.Join(cloneDir, "notes.txt"), []byte("local\n"), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	unrelated, err = client.HasUnrelatedChanges(
		t.Context(),
		git.AllowedPathSet([]string{testTaskfilesDir}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !unrelated {
		t.Fatal("change outside allowed paths should be unrelated")
	}
}

// TestEnsureBranchOwnedAllowsNewBranch verifies a branch that does not yet exist is allowed.
func TestEnsureBranchOwnedAllowsNewBranch(t *testing.T) {
	t.Parallel()

	mockOps := &mockGitOps{
		branchExists: false,
		branchErr:    nil,
		lastMessage:  consts.Empty,
		messageErr:   nil,
	}

	err := git.EnsureBranchOwned(t.Context(), mockOps, testSyncBranch)
	if err != nil {
		t.Fatal(err)
	}
}

// TestEnsureBranchOwnedAllowsTaskOtterBranch verifies a branch with a TaskOtter commit message is allowed.
func TestEnsureBranchOwnedAllowsTaskOtterBranch(t *testing.T) {
	t.Parallel()

	mockOps := &mockGitOps{
		branchExists: true,
		branchErr:    nil,
		lastMessage:  git.SyncCommitMessage,
		messageErr:   nil,
	}

	err := git.EnsureBranchOwned(t.Context(), mockOps, testSyncBranch)
	if err != nil {
		t.Fatal(err)
	}
}

// TestEnsureBranchOwnedRejectsForeignBranch verifies a branch with a non-TaskOtter commit is rejected.
func TestEnsureBranchOwnedRejectsForeignBranch(t *testing.T) {
	t.Parallel()

	mockOps := &mockGitOps{
		branchExists: true,
		branchErr:    nil,
		lastMessage:  "feat: custom work",
		messageErr:   nil,
	}

	err := git.EnsureBranchOwned(t.Context(), mockOps, testSyncBranch)
	if err == nil {
		t.Fatal("expected branch ownership error")
	}
}

// TestEnsureBranchOwnedWrapsOperationErrors verifies underlying git operation errors are wrapped.
func TestEnsureBranchOwnedWrapsOperationErrors(t *testing.T) {
	t.Parallel()

	err := git.EnsureBranchOwned(
		t.Context(),
		&mockGitOps{
			branchExists: false,
			branchErr:    os.ErrInvalid,
			lastMessage:  consts.Empty,
			messageErr:   nil,
		},
		testSyncBranch,
	)

	if err == nil || !strings.Contains(err.Error(), "check branch exists") {
		t.Fatalf("expected branch exists wrapper, got %v", err)
	}

	err = git.EnsureBranchOwned(
		t.Context(),
		&mockGitOps{
			branchExists: true,
			branchErr:    nil,
			lastMessage:  consts.Empty,
			messageErr:   os.ErrPermission,
		},
		testSyncBranch,
	)

	if err == nil || !strings.Contains(err.Error(), "read last commit message") {
		t.Fatalf("expected last message wrapper, got %v", err)
	}
}

// TestValidateGitRefAcceptsSyncBranch verifies a well-formed sync branch ref is accepted.
func TestValidateGitRefAcceptsSyncBranch(t *testing.T) {
	t.Parallel()

	err := git.ValidateGitRef("taskotter/sync-abc123")
	if err != nil {
		t.Fatal(err)
	}
}

// TestValidateGitRefRejectsInvalid verifies malformed or unsafe git ref values are rejected.
func TestValidateGitRefRejectsInvalid(t *testing.T) {
	t.Parallel()

	longRef := strings.Repeat("a", consts.Index256)

	cases := []string{
		consts.Empty,
		"-main",
		longRef,
		"branch with spaces",
		"branch;rm -rf /",
	}

	for i := range cases {
		err := git.ValidateGitRef(cases[i])
		if err == nil {
			t.Fatalf("ValidateGitRef(%q) expected error", cases[i])
		}
	}
}

// TestValidateStagePath verifies stage paths inside the workspace pass and traversal paths fail.
func TestValidateStagePath(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	err := git.ValidateStagePath(workspace, "taskfiles/go/Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}

	err = git.ValidateStagePath(workspace, "../outside")
	if err == nil {
		t.Fatal("expected traversal rejection")
	}

	err = git.ValidateStagePath(workspace, "-f")
	if err == nil {
		t.Fatal("expected flag-like path rejection")
	}
}
