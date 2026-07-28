package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/git"
	gh "github.com/task-otter/Taskotter/internal/github"
	"github.com/task-otter/Taskotter/internal/store"
)

const (
	testMainBranch   = "main"
	testRepository   = "owner/repo"
	testTargetFolder = "taskfiles"
)

type localStore struct {
	root string
}

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
	ref store.RefInfo,
) (*store.Snapshot, error) {
	return store.LocalSnapshot(localStore.root, ref)
}

type mockGitOps struct {
	unrelated          bool
	defaultBranch      string
	defaultBranchCalls int
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
	return "", nil
}
func (mockGitOps *mockGitOps) Stage(context.Context, []string) error    { return nil }
func (mockGitOps *mockGitOps) Commit(context.Context, string) error     { return nil }
func (mockGitOps *mockGitOps) Push(context.Context, string, bool) error { return nil }
func (mockGitOps *mockGitOps) DefaultBranch(context.Context) (string, error) {
	mockGitOps.defaultBranchCalls++
	if mockGitOps.defaultBranch != "" {
		return mockGitOps.defaultBranch, nil
	}

	return testMainBranch, nil
}

type mockPR struct {
	find        *gh.PullRequest
	create      *gh.PullRequest
	updated     int
	lastBase    string
	lastHead    string
	createdBase string
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

	return &gh.PullRequest{Number: 99, URL: "https://example/pr/99"}, nil
}

func (mockPR *mockPR) UpdatePRBody(context.Context, int, string) error {
	mockPR.updated++

	return nil
}

func fixtureRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", "tests", "fixtures", "store"))
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

	err := os.WriteFile(filepath.Join(workspace, "Taskfile.yml"), content, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	return workspace
}

func testConfig(workspace string) *config.Config {
	return &config.Config{
		Tasks:              []string{"go"},
		JSRuntime:          "",
		NodePackageManager: "",
		NodeVersionManager: "",
		IncludesDoc:        true,
		SyncRoot:           true,
		FailOnChanges:      false,
		StoreVersion:       "",
		TargetFolder:       testTargetFolder,
		RootTaskfile:       "Taskfile.yml",
		GitHubToken:        "",
		Workspace:          workspace,
		Repository:         "",
		GitHubOutput:       "",
		BaseBranch:         "",
		ConfigurationHash:  "",
		BranchName:         "",
	}
}

func TestOrchestratorNoChangeAfterApply(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	orchestrator := &app.Orchestrator{
		Logger:      nil,
		StoreClient: &localStore{root: fixtureRoot(t)},
		GitOps:      nil,
		PRClient:    nil,
	}

	first, err := orchestrator.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !first.Changed {
		t.Fatal("expected first run to change files")
	}

	second, err := orchestrator.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if second.Changed {
		t.Fatal("expected no changes on second run")
	}
}

func TestOrchestratorUnrelatedDirtyTreeFails(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)

	orchestrator := &app.Orchestrator{
		Logger:      nil,
		StoreClient: &localStore{root: fixtureRoot(t)},
		GitOps:      &mockGitOps{unrelated: true, defaultBranch: "", defaultBranchCalls: 0},
		PRClient:    nil,
	}

	err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	_, err = orchestrator.Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected unrelated changes error")
	}

	_, statErr := os.Stat(filepath.Join(workspace, "taskfiles/go/Taskfile.yml"))
	if statErr == nil {
		t.Fatal("workspace should not be mutated when git preconditions fail")
	}
}

func TestOrchestratorUpdatesExistingPR(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	cfg.Repository = testRepository

	pullReq := &mockPR{
		find:        &gh.PullRequest{Number: 7, URL: "https://example/pr/7"},
		create:      nil,
		updated:     0,
		lastBase:    "",
		lastHead:    "",
		createdBase: "",
	}
	gitOps := &mockGitOps{unrelated: false, defaultBranch: testMainBranch, defaultBranchCalls: 0}

	orchestrator := &app.Orchestrator{
		Logger:      nil,
		StoreClient: &localStore{root: fixtureRoot(t)},
		GitOps:      gitOps,
		PRClient:    pullReq,
	}

	err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	result, err := orchestrator.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if pullReq.updated != 1 {
		t.Fatalf("expected PR body update, got %d", pullReq.updated)
	}

	if pullReq.lastBase != testMainBranch {
		t.Fatalf("PR base = %q, want main", pullReq.lastBase)
	}

	if gitOps.defaultBranchCalls == 0 {
		t.Fatal("expected DefaultBranch before PR")
	}

	if result.PullRequestNumber != "7" {
		t.Fatalf("got PR number %q", result.PullRequestNumber)
	}
}

func TestOrchestratorCreatesPRWithResolvedBase(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	cfg.Repository = testRepository

	pullReq := &mockPR{
		find:        nil,
		create:      nil,
		updated:     0,
		lastBase:    "",
		lastHead:    "",
		createdBase: "",
	}
	gitOps := &mockGitOps{unrelated: false, defaultBranch: "develop", defaultBranchCalls: 0}

	orchestrator := &app.Orchestrator{
		Logger:      nil,
		StoreClient: &localStore{root: fixtureRoot(t)},
		GitOps:      gitOps,
		PRClient:    pullReq,
	}

	err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	_, err = orchestrator.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if pullReq.createdBase != "develop" {
		t.Fatalf("created PR base = %q, want develop", pullReq.createdBase)
	}
}

func TestOrchestratorCreatesPRAgainstTriggerBranch(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	cfg.Repository = "owner/repo"
	cfg.BaseBranch = "release/2026"

	pullReq := &mockPR{
		find:        nil,
		create:      nil,
		updated:     0,
		lastBase:    "",
		lastHead:    "",
		createdBase: "",
	}
	gitOps := &mockGitOps{unrelated: false, defaultBranch: "", defaultBranchCalls: 0}

	orchestrator := &app.Orchestrator{
		Logger:      nil,
		StoreClient: &localStore{root: fixtureRoot(t)},
		GitOps:      gitOps,
		PRClient:    pullReq,
	}

	err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	_, err = orchestrator.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if pullReq.createdBase != "release/2026" {
		t.Fatalf("created PR base = %q, want release/2026", pullReq.createdBase)
	}

	if gitOps.defaultBranchCalls != 0 {
		t.Fatalf("DefaultBranch called %d times, want 0", gitOps.defaultBranchCalls)
	}
}

func TestNewOrchestratorInvalidRepository(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks:              nil,
		JSRuntime:          "",
		NodePackageManager: "",
		NodeVersionManager: "",
		IncludesDoc:        false,
		SyncRoot:           false,
		FailOnChanges:      false,
		StoreVersion:       "",
		TargetFolder:       "",
		RootTaskfile:       "",
		GitHubToken:        "",
		Workspace:          "",
		Repository:         "not-a-valid-repo",
		GitHubOutput:       "",
		BaseBranch:         "",
		ConfigurationHash:  "",
		BranchName:         "",
	}

	_, err := app.NewOrchestrator(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected repository parse error")
	}
}

var (
	_ git.GitOps  = (*mockGitOps)(nil)
	_ gh.PRClient = (*mockPR)(nil)
)
