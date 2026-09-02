// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/git/adapters/cli"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	mockGitOps struct {
		branchErr    error
		messageErr   error
		lastMessage  string
		branchExists bool
	}

	branchOwnershipCase struct {
		mock *mockGitOps
		name string
	}

	clientFixture struct {
		t      *testing.T
		client *cli.Client
	}

	accessRepo struct {
		access, repository string
	}
)

const (
	testMainBranch = "main"

	testFeatureBranch = "feature/test"

	testSyncBranch = "taskotter/sync-abc"

	dirClone = "clone"

	gitSubcommandInit = "init"

	gitCmdCommit = "commit"

	gitCmdRemote = "remote"

	gitCmdConfig = "config"

	gitCmdAdd = "add"

	flagM = "-m"

	flagB = "-b"

	configUserEmail = "user.email"

	configUserName = "user.name"

	testGitEmail = "test@test.com"

	testGitUserName = "Test"

	pathRefs = "refs"

	pathRemotes = "remotes"

	pathHEAD = "HEAD"

	fileGitignore = ".gitignore"

	pathMetadataYML = "taskfiles/.taskotter/metadata.yml"

	testTaskfilesDir = "taskfiles"

	fmtBranchWantMain = "branch = %q, want main"

	httpExtraHeaderKey = "http.https://github.com/.extraheader"

	gitFlagLocal = "--local"

	gitRemoteGetURL = "get-url"

	gitBinaryName = "git"

	testAccessCred = "ghs_fixture_access"

	testOwnerRepo = "owner/repo"
)

// TestClientBranchMethods verifies branch checkout, existence checks, and last commit message.
func TestClientBranchMethods(t *testing.T) {
	t.Parallel()

	fixture := newClientFixture(t, setupRemoteRepo(t))

	fixture.createBranch(testFeatureBranch)
	fixture.expectBranchExists(testFeatureBranch)
	fixture.expectBranchMissing("missing-branch")
	fixture.expectLastCommitMessage(testMainBranch, gitSubcommandInit)
}

// TestClientCommitAndPush verifies committing and pushing a feature branch succeeds.
func TestClientCommitAndPush(t *testing.T) {
	t.Parallel()

	newClientFixture(t, setupRemoteRepo(t)).commitAndPush()
}

// TestConfigureCredentialsSetsOriginURL verifies checkout extraheader is cleared and origin uses the token URL.
func TestConfigureCredentialsSetsOriginURL(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	seedCheckoutExtraHeader(t, cloneDir)
	mustConfigureCredentials(t, cloneDir, &accessRepo{
		access:     testAccessCred,
		repository: testOwnerRepo,
	})
	assertExtraHeaderUnset(t, cloneDir)
	assertOriginURL(t, cloneDir, wantOriginAccessURL(testAccessCred, testOwnerRepo))
}

// TestConfigureCredentialsNoOpWhenEmpty verifies empty token or repository leaves origin unchanged.
func TestConfigureCredentialsNoOpWhenEmpty(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	before := originGetURL(t, cloneDir)
	mustConfigureCredentials(t, cloneDir, &accessRepo{
		access:     consts.Empty,
		repository: testOwnerRepo,
	})
	mustConfigureCredentials(t, cloneDir, &accessRepo{
		access:     testAccessCred,
		repository: consts.Empty,
	})
	assertOriginURL(t, cloneDir, before)
}

// TestConfigureCredentialsRejectsInvalidRepository verifies unsafe repository coordinates are rejected.
func TestConfigureCredentialsRejectsInvalidRepository(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	before := originGetURL(t, cloneDir)
	assertInvalidRepositoriesRejected(t, cloneDir)
	assertOriginURL(t, cloneDir, before)
}

// TestClientHelpers verifies safe directory setup, identity config, and repo detection.
func TestClientHelpers(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	newClientFixture(t, cloneDir).assertSafeDirectoryAndIdentity()
	assertGitRepoDetection(t, cloneDir)
}

// TestDefaultBranchDirectRef verifies the default branch resolves when origin/HEAD is a direct SHA.
func TestDefaultBranchDirectRef(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	mainSHA := strings.TrimSpace(runGit(t, cloneDir, "rev-parse", "refs/remotes/origin/main"))

	err := os.WriteFile(originHEADPath(cloneDir), []byte(mainSHA+"\n"), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	assertDefaultBranchMain(t, cloneDir)
}

// TestDefaultBranchMissingOriginHEAD verifies the default branch still resolves when origin/HEAD is missing.
func TestDefaultBranchMissingOriginHEAD(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)

	err := os.Remove(originHEADPath(cloneDir))

	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	assertDefaultBranchMain(t, cloneDir)
}

// TestDefaultBranchSymbolicRef verifies the default branch resolves via a symbolic HEAD ref.
func TestDefaultBranchSymbolicRef(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	runGit(t, cloneDir, gitCmdRemote, "set-head", consts.GitOrigin, "-a")
	assertDefaultBranchMain(t, cloneDir)
}

// TestEnsureBranchOwnedAllowsBranch verifies allowed branch ownership cases succeed.
func TestEnsureBranchOwnedAllowsBranch(t *testing.T) {
	t.Parallel()

	cases := allowedBranchOwnershipCases()

	for i := range cases {
		tc := &cases[i]

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assertEnsureBranchOwnedSuccess(t, tc.mock)
		})
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

	err := cli.EnsureBranchOwned(t.Context(), mockOps, testSyncBranch)
	if err == nil {
		t.Fatal("expected branch ownership error")
	}
}

// TestEnsureBranchOwnedWrapsOperationErrors verifies underlying git operation errors are wrapped.
func TestEnsureBranchOwnedWrapsOperationErrors(t *testing.T) {
	t.Parallel()

	assertEnsureBranchOwnedError(t, &mockGitOps{
		branchExists: false,
		branchErr:    os.ErrInvalid,
		lastMessage:  consts.Empty,
		messageErr:   nil,
	}, "check branch exists")

	assertEnsureBranchOwnedError(t, &mockGitOps{
		branchExists: true,
		branchErr:    nil,
		lastMessage:  consts.Empty,
		messageErr:   os.ErrPermission,
	}, "read last commit message")
}

// TestHasUnrelatedChanges verifies changes outside allowed paths are flagged as unrelated.
func TestHasUnrelatedChanges(t *testing.T) {
	t.Parallel()

	cloneDir := setupRemoteRepo(t)
	client := cli.NewClient(cloneDir)
	allowed := cli.AllowedPathSet([]string{testTaskfilesDir})

	writeAllowedTaskfile(t, cloneDir)
	assertNoUnrelatedChanges(t, client, allowed)

	err := os.WriteFile(filepath.Join(cloneDir, "notes.txt"), []byte("local\n"), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	assertHasUnrelatedChanges(t, client, allowed)
}

// TestStageForceAddsGitignoredMetadata verifies gitignored metadata files are force-staged.
func TestStageForceAddsGitignoredMetadata(t *testing.T) {
	t.Parallel()

	cloneDir := initStageTestRepo(t)
	writeStageTestFiles(t, cloneDir)

	client := cli.NewClient(cloneDir)

	err := client.Stage(t.Context(), []string{pathMetadataYML})
	if err != nil {
		t.Fatal(err)
	}

	out := runGit(t, cloneDir, "status", "--porcelain")

	if !strings.Contains(out, pathMetadataYML) {
		t.Fatalf("expected staged metadata, got status:\n%s", out)
	}
}

// TestValidateGitRefAcceptsSyncBranch verifies a well-formed sync branch ref is accepted.
func TestValidateGitRefAcceptsSyncBranch(t *testing.T) {
	t.Parallel()

	err := cli.ValidateGitRef("taskotter/sync-abc123")
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
		err := cli.ValidateGitRef(cases[i])
		if err == nil {
			t.Fatalf("ValidateGitRef(%q) expected error", cases[i])
		}
	}
}

// TestValidateStagePath verifies stage paths inside the workspace pass and traversal paths fail.
func TestValidateStagePath(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	assertValidStagePath(t, workspace, "taskfiles/go/Taskfile.yml")
	assertInvalidStagePath(t, workspace, "../outside")
	assertInvalidStagePath(t, workspace, "-f")
}

func allowedBranchOwnershipCases() []branchOwnershipCase {
	return []branchOwnershipCase{
		{
			name: "new branch",
			mock: &mockGitOps{
				branchExists: false,
				branchErr:    nil,
				lastMessage:  consts.Empty,
				messageErr:   nil,
			},
		},
		{
			name: "taskotter branch",
			mock: &mockGitOps{
				branchExists: true,
				branchErr:    nil,
				lastMessage:  cli.SyncCommitMessage,
				messageErr:   nil,
			},
		},
	}
}

func assertDefaultBranchMain(t *testing.T, cloneDir string) {
	t.Helper()

	client := cli.NewClient(cloneDir)

	branch, err := client.DefaultBranch(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if branch != testMainBranch {
		t.Fatalf(fmtBranchWantMain, branch)
	}
}

func assertExtraHeaderUnset(t *testing.T, cloneDir string) {
	t.Helper()

	cmd := exec.CommandContext(
		t.Context(),
		gitBinaryName,
		gitCmdConfig,
		gitFlagLocal,
		"--get-all",
		httpExtraHeaderKey,
	)

	cmd.Dir = cloneDir

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected extraheader unset, got %q", strings.TrimSpace(string(out)))
	}
}

func assertInvalidRepositoriesRejected(t *testing.T, cloneDir string) {
	t.Helper()

	cases := []string{
		"owner",
		"owner/repo/extra",
		"owner/..",
		"owner/repo with spaces",
	}

	client := cli.NewClient(cloneDir)

	for i := range cases {
		err := client.ConfigureCredentials(t.Context(), testAccessCred, cases[i])
		if err == nil {
			t.Fatalf("ConfigureCredentials(%q) expected error", cases[i])
		}
	}
}

func assertOriginURL(t *testing.T, cloneDir, want string) {
	t.Helper()

	got := originGetURL(t, cloneDir)

	if got != want {
		t.Fatalf("origin URL = %q, want %q", got, want)
	}
}

func mustConfigureCredentials(t *testing.T, cloneDir string, in *accessRepo) {
	t.Helper()

	err := cli.NewClient(cloneDir).ConfigureCredentials(t.Context(), in.access, in.repository)
	if err != nil {
		t.Fatal(err)
	}
}

func originGetURL(t *testing.T, cloneDir string) string {
	t.Helper()

	return strings.TrimSpace(runGit(t, cloneDir, gitCmdRemote, gitRemoteGetURL, consts.GitOrigin))
}

func seedCheckoutExtraHeader(t *testing.T, cloneDir string) {
	t.Helper()

	runGit(
		t,
		cloneDir,
		gitCmdConfig,
		gitFlagLocal,
		httpExtraHeaderKey,
		"AUTHORIZATION: basic fake",
	)
}

func wantOriginAccessURL(access, repository string) string {
	return "https://x-access-token:" + access + "@github.com/" + repository + ".git"
}

func assertEnsureBranchOwnedSuccess(t *testing.T, mockOps *mockGitOps) {
	t.Helper()

	err := cli.EnsureBranchOwned(t.Context(), mockOps, testSyncBranch)
	if err != nil {
		t.Fatal(err)
	}
}

func assertEnsureBranchOwnedError(t *testing.T, mockOps *mockGitOps, wantMsg string) {
	t.Helper()

	err := cli.EnsureBranchOwned(t.Context(), mockOps, testSyncBranch)

	if err == nil || !strings.Contains(err.Error(), wantMsg) {
		t.Fatalf("expected %q wrapper, got %v", wantMsg, err)
	}
}

func assertGitRepoDetection(t *testing.T, cloneDir string) {
	t.Helper()

	if !cli.IsGitRepo(cloneDir) {
		t.Fatal("expected temp checkout to be a git repo")
	}

	if cli.IsGitRepo(t.TempDir()) {
		t.Fatal("plain temp dir is not a git repo")
	}
}

func assertHasUnrelatedChanges(t *testing.T, client *cli.Client, allowed map[string]struct{}) {
	t.Helper()

	unrelated, err := client.HasUnrelatedChanges(t.Context(), allowed)
	if err != nil {
		t.Fatal(err)
	}

	if !unrelated {
		t.Fatal("change outside allowed paths should be unrelated")
	}
}

func assertInvalidStagePath(t *testing.T, workspace, path string) {
	t.Helper()

	err := cli.ValidateStagePath(workspace, path)
	if err == nil {
		t.Fatalf("ValidateStagePath(%q) expected error", path)
	}
}

func assertNoUnrelatedChanges(t *testing.T, client *cli.Client, allowed map[string]struct{}) {
	t.Helper()

	unrelated, err := client.HasUnrelatedChanges(t.Context(), allowed)
	if err != nil {
		t.Fatal(err)
	}

	if unrelated {
		t.Fatal("changes under allowed folder should not be unrelated")
	}
}

func assertValidStagePath(t *testing.T, workspace, path string) {
	t.Helper()

	err := cli.ValidateStagePath(workspace, path)
	if err != nil {
		t.Fatal(err)
	}
}

func commitReadmeToOrigin(t *testing.T, cloneDir, bareDir string) {
	t.Helper()

	err := os.WriteFile(
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
}

func commitStageTestBaseline(t *testing.T, cloneDir string) {
	t.Helper()

	err := os.WriteFile(
		filepath.Join(cloneDir, consts.ReadmeMD),
		[]byte(gitSubcommandInit+"\n"),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, gitCmdAdd, consts.ReadmeMD, fileGitignore)
	runGit(t, cloneDir, gitCmdCommit, flagM, gitSubcommandInit)
}

func configureTestGitUser(t *testing.T, cloneDir string) {
	t.Helper()

	runGit(t, cloneDir, gitCmdConfig, configUserEmail, testGitEmail)
	runGit(t, cloneDir, gitCmdConfig, configUserName, testGitUserName)
}

func initBareRemote(t *testing.T, root string) string {
	t.Helper()

	bareDir := filepath.Join(root, "bare.git")

	err := os.MkdirAll(bareDir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, bareDir, gitSubcommandInit, "--bare", flagB, testMainBranch)

	return bareDir
}

func initCloneWithReadme(t *testing.T, root, bareDir string) string {
	t.Helper()

	cloneDir := filepath.Join(root, dirClone)

	err := os.MkdirAll(cloneDir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, gitSubcommandInit, flagB, testMainBranch)
	configureTestGitUser(t, cloneDir)
	commitReadmeToOrigin(t, cloneDir, bareDir)

	return cloneDir
}

func initStageTestRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	cloneDir := filepath.Join(root, dirClone)

	err := os.MkdirAll(cloneDir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, cloneDir, gitSubcommandInit, flagB, testMainBranch)
	runGit(t, cloneDir, gitCmdConfig, configUserEmail, testGitEmail)
	runGit(t, cloneDir, gitCmdConfig, configUserName, testGitUserName)

	return cloneDir
}

func newClientFixture(t *testing.T, cloneDir string) *clientFixture {
	t.Helper()

	return &clientFixture{t: t, client: cli.NewClient(cloneDir)}
}

func originHEADPath(cloneDir string) string {
	return filepath.Join(
		cloneDir,
		consts.GitDir,
		pathRefs,
		pathRemotes,
		consts.GitOrigin,
		pathHEAD,
	)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	//nolint:gosec // test helper runs git in isolated temp repos
	cmd := exec.CommandContext(t.Context(), gitBinaryName, args...)

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
	bareDir := initBareRemote(t, root)

	return initCloneWithReadme(t, root, bareDir)
}

func writeAllowedTaskfile(t *testing.T, cloneDir string) {
	t.Helper()

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
}

func writeGitignoreFile(t *testing.T, cloneDir string) {
	t.Helper()

	err := os.WriteFile(
		filepath.Join(cloneDir, fileGitignore),
		[]byte(".taskotter/\n"),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func writeMetadataFile(t *testing.T, cloneDir string) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(cloneDir, "taskfiles", ".taskotter"), consts.FilePerm755)
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
}

func writeStageTestFiles(t *testing.T, cloneDir string) {
	t.Helper()

	writeGitignoreFile(t, cloneDir)
	writeMetadataFile(t, cloneDir)
	commitStageTestBaseline(t, cloneDir)
}

func (fixture *clientFixture) assertSafeDirectoryAndIdentity() {
	fixture.t.Helper()

	fixture.client.EnsureSafeDirectory()
	cli.WriteLocalIdentity()
}

func (fixture *clientFixture) commitAndPush() {
	fixture.t.Helper()

	err := fixture.client.CreateOrResetBranch(fixture.t.Context(), testFeatureBranch)
	if err != nil {
		fixture.t.Fatal(err)
	}

	err = fixture.client.Commit(fixture.t.Context(), "nothing to commit")
	if err != nil {
		fixture.t.Fatal(err)
	}

	err = fixture.client.PushForceWithLease(fixture.t.Context(), testFeatureBranch)
	if err != nil {
		fixture.t.Fatal(err)
	}
}

func (fixture *clientFixture) createBranch(branch string) {
	fixture.t.Helper()

	err := fixture.client.CreateOrResetBranch(fixture.t.Context(), branch)
	if err != nil {
		fixture.t.Fatal(err)
	}
}

func (fixture *clientFixture) expectBranchExists(branch string) {
	fixture.t.Helper()

	exists, err := fixture.client.BranchExists(fixture.t.Context(), branch)
	if err != nil {
		fixture.t.Fatal(err)
	}

	if !exists {
		fixture.t.Fatalf("BranchExists(%q) = false, want true", branch)
	}
}

func (fixture *clientFixture) expectBranchMissing(branch string) {
	fixture.t.Helper()

	exists, err := fixture.client.BranchExists(fixture.t.Context(), branch)
	if err != nil {
		fixture.t.Fatal(err)
	}

	if exists {
		fixture.t.Fatalf("BranchExists(%q) = true, want false", branch)
	}
}

func (fixture *clientFixture) expectLastCommitMessage(branch, want string) {
	fixture.t.Helper()

	msg, err := fixture.client.LastCommitMessage(fixture.t.Context(), branch)

	if err != nil || msg != want {
		fixture.t.Fatalf("LastCommitMessage() = %q, %v; want %q, nil", msg, err, want)
	}
}

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
