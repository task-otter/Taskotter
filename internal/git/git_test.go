package git_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/git"
)

const testMainBranch = "main"

type mockGitOps struct {
	branchExists bool
	branchErr    error
	lastMessage  string
	messageErr   error
}

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
		return "", mockGitOps.messageErr
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
	cloneDir := filepath.Join(root, "clone")

	err := os.MkdirAll(bareDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(cloneDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, bareDir, "init", "--bare", "-b", testMainBranch)
	runGit(t, cloneDir, "init", "-b", testMainBranch)
	runGit(t, cloneDir, "config", "user.email", "test@test.com")
	runGit(t, cloneDir, "config", "user.name", "Test")

	err = os.WriteFile(filepath.Join(cloneDir, "README.md"), []byte("init\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, "add", "README.md")
	runGit(t, cloneDir, "commit", "-m", "init")
	runGit(t, cloneDir, "remote", "add", "origin", bareDir)
	runGit(t, cloneDir, "push", "-u", "origin", testMainBranch)
	runGit(t, cloneDir, "fetch", "origin")

	return cloneDir
}

func TestDefaultBranchSymbolicRef(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	runGit(t, cloneDir, "remote", "set-head", "origin", "-a")

	client := git.NewClient(cloneDir)

	branch, err := client.DefaultBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if branch != testMainBranch {
		t.Fatalf("branch = %q, want main", branch)
	}
}

func TestDefaultBranchMissingOriginHEAD(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)

	originHEAD := filepath.Join(cloneDir, ".git", "refs", "remotes", "origin", "HEAD")

	err := os.Remove(originHEAD)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	client := git.NewClient(cloneDir)

	branch, err := client.DefaultBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if branch != testMainBranch {
		t.Fatalf("branch = %q, want main", branch)
	}
}

func TestDefaultBranchDirectRef(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	mainSHA := strings.TrimSpace(runGit(t, cloneDir, "rev-parse", "refs/remotes/origin/main"))

	originHEAD := filepath.Join(cloneDir, ".git", "refs", "remotes", "origin", "HEAD")

	err := os.WriteFile(originHEAD, []byte(mainSHA+"\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	client := git.NewClient(cloneDir)

	branch, err := client.DefaultBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if branch != testMainBranch {
		t.Fatalf("branch = %q, want main", branch)
	}
}

func TestStageForceAddsGitignoredMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cloneDir := filepath.Join(root, "clone")

	err := os.MkdirAll(cloneDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, "init", "-b", testMainBranch)
	runGit(t, cloneDir, "config", "user.email", "test@test.com")
	runGit(t, cloneDir, "config", "user.name", "Test")

	err = os.WriteFile(filepath.Join(cloneDir, ".gitignore"), []byte(".taskotter/\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(filepath.Join(cloneDir, "taskfiles/.taskotter"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(cloneDir, "taskfiles/.taskotter/metadata.yml"),
		[]byte("target_folder: taskfiles\n"),
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(cloneDir, "README.md"), []byte("init\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, "add", "README.md", ".gitignore")
	runGit(t, cloneDir, "commit", "-m", "init")

	client := git.NewClient(cloneDir)

	err = client.Stage(context.Background(), []string{"taskfiles/.taskotter/metadata.yml"})
	if err != nil {
		t.Fatal(err)
	}

	out := runGit(t, cloneDir, "status", "--porcelain")
	if !strings.Contains(out, "taskfiles/.taskotter/metadata.yml") {
		t.Fatalf("expected staged metadata, got status:\n%s", out)
	}
}

func TestClientGitWorkflowMethods(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	client := git.NewClient(cloneDir)
	ctx := context.Background()

	if err := client.EnsureSafeDirectory(ctx); err != nil {
		t.Fatal(err)
	}

	if err := git.WriteLocalIdentity(ctx, client); err != nil {
		t.Fatal(err)
	}

	if !git.IsGitRepo(cloneDir) {
		t.Fatal("expected temp checkout to be a git repo")
	}

	if git.IsGitRepo(t.TempDir()) {
		t.Fatal("plain temp dir is not a git repo")
	}

	if err := client.CheckoutBranch(ctx, "feature/test", true); err != nil {
		t.Fatal(err)
	}

	exists, err := client.BranchExists(ctx, "feature/test")
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

	if msg, err := client.LastCommitMessage(ctx, testMainBranch); err != nil || msg != "init" {
		t.Fatalf("LastCommitMessage() = %q, %v; want init, nil", msg, err)
	}

	if err := client.Commit(ctx, "nothing to commit"); err != nil {
		t.Fatal(err)
	}

	if err := client.Push(ctx, "feature/test", true); err != nil {
		t.Fatal(err)
	}
}

func TestHasUnrelatedChanges(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	client := git.NewClient(cloneDir)

	err := os.MkdirAll(filepath.Join(cloneDir, "taskfiles", "go"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(cloneDir, "taskfiles", "go", "Taskfile.yml"), []byte("version: '3'\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	unrelated, err := client.HasUnrelatedChanges(
		context.Background(),
		git.AllowedPathSet([]string{"taskfiles"}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if unrelated {
		t.Fatal("changes under allowed folder should not be unrelated")
	}

	err = os.WriteFile(filepath.Join(cloneDir, "notes.txt"), []byte("local\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	unrelated, err = client.HasUnrelatedChanges(
		context.Background(),
		git.AllowedPathSet([]string{"taskfiles"}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !unrelated {
		t.Fatal("change outside allowed paths should be unrelated")
	}
}

func TestEnsureBranchOwnedAllowsNewBranch(t *testing.T) {
	t.Parallel()

	mockOps := &mockGitOps{branchExists: false, lastMessage: ""}

	err := git.EnsureBranchOwned(context.Background(), mockOps, "taskotter/sync-abc")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureBranchOwnedAllowsTaskOtterBranch(t *testing.T) {
	t.Parallel()

	mockOps := &mockGitOps{branchExists: true, lastMessage: git.SyncCommitMessage}

	err := git.EnsureBranchOwned(context.Background(), mockOps, "taskotter/sync-abc")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureBranchOwnedRejectsForeignBranch(t *testing.T) {
	t.Parallel()

	mockOps := &mockGitOps{branchExists: true, lastMessage: "feat: custom work"}

	err := git.EnsureBranchOwned(context.Background(), mockOps, "taskotter/sync-abc")
	if err == nil {
		t.Fatal("expected branch ownership error")
	}
}

func TestEnsureBranchOwnedWrapsOperationErrors(t *testing.T) {
	t.Parallel()

	err := git.EnsureBranchOwned(
		context.Background(),
		&mockGitOps{branchErr: errors.New("branch check failed")},
		"taskotter/sync-abc",
	)
	if err == nil || !strings.Contains(err.Error(), "check branch exists") {
		t.Fatalf("expected branch exists wrapper, got %v", err)
	}

	err = git.EnsureBranchOwned(
		context.Background(),
		&mockGitOps{branchExists: true, messageErr: errors.New("log failed")},
		"taskotter/sync-abc",
	)
	if err == nil || !strings.Contains(err.Error(), "read last commit message") {
		t.Fatalf("expected last message wrapper, got %v", err)
	}
}

func TestValidateGitRefAcceptsSyncBranch(t *testing.T) {
	t.Parallel()

	err := git.ValidateGitRef("taskotter/sync-abc123")
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateGitRefRejectsInvalid(t *testing.T) {
	t.Parallel()

	longRef := strings.Repeat("a", 256)
	cases := []string{
		"",
		"-main",
		longRef,
		"branch with spaces",
		"branch;rm -rf /",
	}
	for _, ref := range cases {
		err := git.ValidateGitRef(ref)
		if err == nil {
			t.Fatalf("ValidateGitRef(%q) expected error", ref)
		}
	}
}

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
