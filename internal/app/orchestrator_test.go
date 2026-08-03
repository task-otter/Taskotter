// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package app_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/git"
	gh "github.com/task-otter/Taskotter/internal/github"
	"github.com/task-otter/Taskotter/internal/store"
)

type (
	localStore struct {
		root string
	}

	mockGitOps struct {
		defaultBranch      string
		defaultBranchCalls int
		unrelated          bool
	}

	mockPR struct {
		find        *gh.PullRequest
		create      *gh.PullRequest
		lastBase    string
		lastHead    string
		createdBase string
		updated     int
	}
)

const (
	testMainBranch    = "main"
	testRepository    = "owner/repo"
	testTargetFolder  = "taskfiles"
	testDevelopBranch = "develop"
	testReleaseBranch = "release/2026"
)

var (
	_ git.Operations = (*mockGitOps)(nil)
	_ gh.PRClient    = (*mockPR)(nil)
)

func (localStore *localStore) ResolveRef(
	_ context.Context,
	requestedVersion string,
) (store.RefInfo, error) {
	return store.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: requestedVersion,
		SourceRef:        "refs/heads/" + testMainBranch,
		ResolvedCommit:   "deadbeef",
		DefaultBranch:    testMainBranch,
	}, nil
}

func (localStore *localStore) DownloadSnapshot(
	_ context.Context,
	ref *store.RefInfo,
) (*store.Snapshot, error) {
	snap, err := store.LocalSnapshot(localStore.root, ref)
	if err != nil {
		return nil, fmt.Errorf("local snapshot: %w", err)
	}

	return snap, nil
}

func (mockGitOps *mockGitOps) EnsureSafeDirectory(context.Context) error { return nil }

func (mockGitOps *mockGitOps) HasUnrelatedChanges(
	context.Context,
	map[string]struct{},
) (bool, error) {
	return mockGitOps.unrelated, nil
}
func (mockGitOps *mockGitOps) CheckoutBranch(context.Context, string, bool) error { return nil }
func (mockGitOps *mockGitOps) BranchExists(context.Context, string) (bool, error) {
	return false, nil
}

func (mockGitOps *mockGitOps) LastCommitMessage(context.Context, string) (string, error) {
	return consts.Empty, nil
}
func (mockGitOps *mockGitOps) Stage(context.Context, []string) error    { return nil }
func (mockGitOps *mockGitOps) Commit(context.Context, string) error     { return nil }
func (mockGitOps *mockGitOps) Push(context.Context, string, bool) error { return nil }
func (mockGitOps *mockGitOps) DefaultBranch(context.Context) (string, error) {
	mockGitOps.defaultBranchCalls++

	if mockGitOps.defaultBranch != consts.Empty {
		return mockGitOps.defaultBranch, nil
	}

	return testMainBranch, nil
}

func (mockPR *mockPR) FindOpenPR(_ context.Context, branch, base string) (*gh.PullRequest, error) {
	mockPR.lastHead = branch
	mockPR.lastBase = base

	return mockPR.find, nil
}

func (mockPR *mockPR) CreatePR(_ context.Context, _, base, _ string) (*gh.PullRequest, error) {
	mockPR.createdBase = base

	if mockPR.create != nil {
		return mockPR.create, nil
	}

	return &gh.PullRequest{Number: consts.Index99, URL: "https://example/pr/99"}, nil
}

func (mockPR *mockPR) UpdatePRBody(context.Context, int, string) error {
	mockPR.updated++

	return nil
}

func fixtureRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(
		filepath.Join(consts.PathParent, consts.PathParent, "tests", "fixtures", "store"),
	)
	if err != nil {
		t.Fatal(err)
	}

	return root
}

func workspaceWithRootTaskfile(t *testing.T) string {
	t.Helper()

	workspace := t.TempDir()

	content := []byte(`version: "3"
includes: {}
tasks:
  hello:
    cmds:
      - echo hello
`)

	err := os.WriteFile(filepath.Join(workspace, consts.Taskfile), content, consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	return workspace
}

func newMockPR(find, create *gh.PullRequest) *mockPR {
	return &mockPR{
		find:        find,
		create:      create,
		updated:     consts.IndexZero,
		lastBase:    consts.Empty,
		lastHead:    consts.Empty,
		createdBase: consts.Empty,
	}
}

func newMockGitOps(unrelated bool, defaultBranch string) *mockGitOps {
	return &mockGitOps{
		unrelated:          unrelated,
		defaultBranch:      defaultBranch,
		defaultBranchCalls: consts.IndexZero,
	}
}

func newTestOrchestrator(t *testing.T, gitOps git.Operations, pr gh.PRClient) *app.Orchestrator {
	t.Helper()

	return &app.Orchestrator{
		Logger:      nil,
		StoreClient: &localStore{root: fixtureRoot(t)},
		GitOps:      gitOps,
		PRClient:    pr,
	}
}

func initGitWorkspace(t *testing.T, workspace string) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(workspace, consts.GitDir), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}
}

func runOrchestrator(t *testing.T, orchestrator *app.Orchestrator, cfg *config.Config) *app.Result {
	t.Helper()

	result, err := orchestrator.Run(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func assertPathNotExist(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("expected %q not to exist", path)
	}
}

func assertPRUpdated(
	t *testing.T,
	pullReq *mockPR,
	gitOps *mockGitOps,
	result *app.Result,
	wantBase, wantNumber string,
) {
	t.Helper()

	if pullReq.updated != consts.IndexOne {
		t.Fatalf("expected PR body update, got %d", pullReq.updated)
	}

	if pullReq.lastBase != wantBase {
		t.Fatalf("PR base = %q, want %s", pullReq.lastBase, wantBase)
	}

	if gitOps.defaultBranchCalls == consts.IndexZero {
		t.Fatal("expected DefaultBranch before PR")
	}

	if result.PullRequestNumber != wantNumber {
		t.Fatalf("got PR number %q", result.PullRequestNumber)
	}
}

func assertPRCreated(
	t *testing.T,
	pullReq *mockPR,
	gitOps *mockGitOps,
	wantBase string,
	wantDefaultBranchCalls int,
) {
	t.Helper()

	if pullReq.createdBase != wantBase {
		t.Fatalf("created PR base = %q, want %s", pullReq.createdBase, wantBase)
	}

	if gitOps.defaultBranchCalls != wantDefaultBranchCalls {
		t.Fatalf(
			"DefaultBranch called %d times, want %d",
			gitOps.defaultBranchCalls,
			wantDefaultBranchCalls,
		)
	}
}

func testConfig(workspace string) *config.Config {
	return &config.Config{
		Tasks:              []string{consts.Go},
		JSRuntime:          consts.Empty,
		NodePackageManager: consts.Empty,
		NodeVersionManager: consts.Empty,
		IncludesDoc:        true,
		SyncRoot:           true,
		FailOnChanges:      false,
		StoreVersion:       consts.Empty,
		TargetFolder:       testTargetFolder,
		RootTaskfile:       consts.Taskfile,
		GitHubToken:        consts.Empty,
		Workspace:          workspace,
		Repository:         consts.Empty,
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}
}

func testConfigWithRepo(workspace, repo string) *config.Config {
	cfg := testConfig(workspace)

	cfg.Repository = repo

	return cfg
}

// TestOrchestratorNoChangeAfterApply verifies a second run reports no changes after apply.
func TestOrchestratorNoChangeAfterApply(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	orchestrator := newTestOrchestrator(t, nil, nil)

	first := runOrchestrator(t, orchestrator, cfg)

	if !first.Changed {
		t.Fatal("expected first run to change files")
	}

	second := runOrchestrator(t, orchestrator, cfg)

	if second.Changed {
		t.Fatal("expected no changes on second run")
	}
}

// TestOrchestratorUnrelatedDirtyTreeFails verifies unrelated dirty changes fail the run.
func TestOrchestratorUnrelatedDirtyTreeFails(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	orchestrator := newTestOrchestrator(t, newMockGitOps(true, consts.Empty), nil)

	initGitWorkspace(t, workspace)

	_, err := orchestrator.Run(t.Context(), cfg)
	if err == nil {
		t.Fatal("expected unrelated changes error")
	}

	assertPathNotExist(t, filepath.Join(workspace, "taskfiles", "go", "Taskfile.yml"))
}

// TestOrchestratorUpdatesExistingPR verifies an existing open pull request is updated.
func TestOrchestratorUpdatesExistingPR(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfigWithRepo(workspace, testRepository)

	pullReq := newMockPR(
		&gh.PullRequest{Number: consts.IndexSeven, URL: "https://example/pr/7"},
		nil,
	)
	gitOps := newMockGitOps(false, testMainBranch)
	orchestrator := newTestOrchestrator(t, gitOps, pullReq)

	initGitWorkspace(t, workspace)

	result := runOrchestrator(t, orchestrator, cfg)

	assertPRUpdated(t, pullReq, gitOps, result, testMainBranch, "7")
}

// TestOrchestratorCreatesPRWithResolvedBase verifies the PR base resolves to the default branch.
func TestOrchestratorCreatesPRWithResolvedBase(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfigWithRepo(workspace, testRepository)

	pullReq := newMockPR(nil, nil)
	gitOps := newMockGitOps(false, testDevelopBranch)
	orchestrator := newTestOrchestrator(t, gitOps, pullReq)

	initGitWorkspace(t, workspace)
	runOrchestrator(t, orchestrator, cfg)

	if pullReq.createdBase != testDevelopBranch {
		t.Fatalf("created PR base = %q, want develop", pullReq.createdBase)
	}
}

// TestOrchestratorCreatesPRAgainstTriggerBranch verifies the PR targets the configured base branch.
func TestOrchestratorCreatesPRAgainstTriggerBranch(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfigWithRepo(workspace, testRepository)

	cfg.BaseBranch = testReleaseBranch

	pullReq := newMockPR(nil, nil)
	gitOps := newMockGitOps(false, consts.Empty)
	orchestrator := newTestOrchestrator(t, gitOps, pullReq)

	initGitWorkspace(t, workspace)
	runOrchestrator(t, orchestrator, cfg)

	assertPRCreated(t, pullReq, gitOps, testReleaseBranch, consts.IndexZero)
}

// TestNewOrchestratorInvalidRepository verifies an invalid repository coordinate fails construction.
func TestNewOrchestratorInvalidRepository(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks:              nil,
		JSRuntime:          consts.Empty,
		NodePackageManager: consts.Empty,
		NodeVersionManager: consts.Empty,
		IncludesDoc:        false,
		SyncRoot:           false,
		FailOnChanges:      false,
		StoreVersion:       consts.Empty,
		TargetFolder:       consts.Empty,
		RootTaskfile:       consts.Empty,
		GitHubToken:        consts.Empty,
		Workspace:          consts.Empty,
		Repository:         "not-a-valid-repo",
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}

	_, err := app.NewOrchestrator(t.Context(), cfg)
	if err == nil {
		t.Fatal("expected repository parse error")
	}
}
