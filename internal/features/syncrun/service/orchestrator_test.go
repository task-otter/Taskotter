// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prports "github.com/task-otter/Taskotter/internal/features/pr/ports"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	"github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	localStore struct {
		root string
	}

	pathSet = map[string]struct{}

	mockGitOps struct {
		defaultBranch      string
		defaultBranchCalls int
		unrelated          bool
	}

	mockPR struct {
		find        *prdomain.PullRequest
		create      *prdomain.PullRequest
		lastBase    string
		createdBase string
		updated     int
	}

	assertPRUpdatedInput struct {
		t        *testing.T
		pullReq  *mockPR
		gitOps   *mockGitOps
		result   *service.Result
		wantBase string
		wantNum  string
	}

	assertPRCreatedInput struct {
		t                      *testing.T
		pullReq                *mockPR
		gitOps                 *mockGitOps
		wantBase               string
		wantDefaultBranchCalls int
	}

	gitRepoRunInput struct {
		t      *testing.T
		gitOps *mockGitOps
		pr     prports.PRClient
		cfg    *config.Config
	}

	refInfo = storedomain.RefInfo
	snap    = storedomain.Snapshot
	pullReq = prdomain.PullRequest
)

const (
	testMainBranch = "main"

	testRepository = "owner/repo"

	testTargetFolder = "taskfiles"

	testDevelopBranch = "develop"

	testReleaseBranch = "release/2026"
)

var (
	_ gitports.Brancher  = (*mockGitOps)(nil)
	_ gitports.Indexer   = (*mockGitOps)(nil)
	_ gitports.Publisher = (*mockGitOps)(nil)

	_ prports.PRClient = (*mockPR)(nil)
)

// TestOrchestratorCreatesPRAgainstTriggerBranch verifies the PR targets the configured base branch.
func TestOrchestratorCreatesPRAgainstTriggerBranch(t *testing.T) {
	t.Parallel()

	pullReq := newMockPR(nil, nil)
	gitOps := newMockGitOps(false, consts.Empty)
	runGitRepoOrchestrator(&gitRepoRunInput{
		t: t, gitOps: gitOps, pr: pullReq, cfg: testConfigWithBaseBranch(testReleaseBranch),
	})

	assertPRCreated(&assertPRCreatedInput{
		t: t, pullReq: pullReq, gitOps: gitOps,
		wantBase: testReleaseBranch, wantDefaultBranchCalls: consts.IndexZero,
	})
}

// TestOrchestratorCreatesPRWithResolvedBase verifies the PR base resolves to the default branch.
func TestOrchestratorCreatesPRWithResolvedBase(t *testing.T) {
	t.Parallel()

	pullReq := newMockPR(nil, nil)
	gitOps := newMockGitOps(false, testDevelopBranch)
	runGitRepoOrchestrator(&gitRepoRunInput{t: t, gitOps: gitOps, pr: pullReq, cfg: nil})

	if pullReq.createdBase != testDevelopBranch {
		t.Fatalf("created PR base = %q, want develop", pullReq.createdBase)
	}
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

	result, err := orchestrator.Run(t.Context(), cfg)
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected unrelated changes error")
	}

	assertPathNotExist(t, filepath.Join(workspace, testTargetFolder, "go", "Taskfile.yml"))
}

// TestOrchestratorUpdatesExistingPR verifies an existing open pull request is updated.
func TestOrchestratorUpdatesExistingPR(t *testing.T) {
	t.Parallel()

	pullReq := newMockPR(
		&prdomain.PullRequest{Number: consts.IndexSeven, URL: "https://example/pr/7"},
		nil,
	)
	gitOps := newMockGitOps(false, testMainBranch)
	result := runGitRepoOrchestrator(&gitRepoRunInput{t: t, gitOps: gitOps, pr: pullReq, cfg: nil})

	assertPRUpdated(&assertPRUpdatedInput{
		t: t, pullReq: pullReq, gitOps: gitOps, result: result,
		wantBase: testMainBranch, wantNum: "7",
	})
}

func assertPRCreated(input *assertPRCreatedInput) {
	input.t.Helper()

	if input.pullReq.createdBase != input.wantBase {
		input.t.Fatalf("created PR base = %q, want %s", input.pullReq.createdBase, input.wantBase)
	}

	if input.gitOps.defaultBranchCalls != input.wantDefaultBranchCalls {
		input.t.Fatalf(
			"DefaultBranch called %d times, want %d",
			input.gitOps.defaultBranchCalls,
			input.wantDefaultBranchCalls,
		)
	}
}

func assertPRUpdated(input *assertPRUpdatedInput) {
	input.t.Helper()

	if input.pullReq.updated != consts.IndexOne {
		input.t.Fatalf("expected PR body update, got %d", input.pullReq.updated)
	}

	if input.pullReq.lastBase != input.wantBase {
		input.t.Fatalf("PR base = %q, want %s", input.pullReq.lastBase, input.wantBase)
	}

	if input.gitOps.defaultBranchCalls == consts.IndexZero {
		input.t.Fatal("expected DefaultBranch before PR")
	}

	if input.result.PullRequestNumber != input.wantNum {
		input.t.Fatalf("got PR number %q", input.result.PullRequestNumber)
	}
}

func assertPathNotExist(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	iox.Discard(info)

	if err == nil {
		t.Fatalf("expected %q not to exist", path)
	}
}

func fixtureRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(
		filepath.Join(
			consts.PathParent, consts.PathParent, consts.PathParent, consts.PathParent,
			"tests", "fixtures", "store",
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	return root
}

func initGitWorkspace(t *testing.T, workspace string) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(workspace, consts.GitDir), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}
}

func newMockGitOps(unrelated bool, defaultBranch string) *mockGitOps {
	return &mockGitOps{
		unrelated:          unrelated,
		defaultBranch:      defaultBranch,
		defaultBranchCalls: consts.IndexZero,
	}
}

func newMockPR(find, create *prdomain.PullRequest) *mockPR {
	return &mockPR{
		find:        find,
		create:      create,
		updated:     consts.IndexZero,
		lastBase:    consts.Empty,
		createdBase: consts.Empty,
	}
}

func newTestOrchestrator(
	t *testing.T,
	gitOps *mockGitOps,
	pullReq prports.PRClient,
) *service.Orchestrator {
	t.Helper()

	return &service.Orchestrator{
		Logger:       nil,
		StoreClient:  &localStore{root: fixtureRoot(t)},
		GitClient:    nil,
		GitBrancher:  gitOps,
		GitIndexer:   gitOps,
		GitPublisher: gitOps,
		PRClient:     pullReq,
	}
}

func runGitRepoOrchestrator(input *gitRepoRunInput) *service.Result {
	input.t.Helper()

	workspace := workspaceWithRootTaskfile(input.t)
	cfg := resolveGitRepoConfig(input.cfg, workspace)

	initGitWorkspace(input.t, workspace)

	return runOrchestrator(input.t, newTestOrchestrator(input.t, input.gitOps, input.pr), cfg)
}

func resolveGitRepoConfig(cfg *config.Config, workspace string) *config.Config {
	if cfg == nil {
		return testConfigWithRepo(workspace, testRepository)
	}

	if cfg.Workspace == consts.Empty {
		cfg.Workspace = workspace
	}

	return cfg
}

func runOrchestrator(
	t *testing.T,
	orchestrator *service.Orchestrator,
	cfg *config.Config,
) *service.Result {
	t.Helper()

	result, err := orchestrator.Run(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	return result
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

func testConfigWithBaseBranch(baseBranch string) *config.Config {
	cfg := testConfigWithRepo(consts.Empty, testRepository)

	cfg.BaseBranch = baseBranch

	return cfg
}

func testConfigWithRepo(workspace, repo string) *config.Config {
	cfg := testConfig(workspace)

	cfg.Repository = repo

	return cfg
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

func (loc *localStore) DownloadSnapshot(_ context.Context, ref *refInfo) (*snap, error) {
	out, err := storesvc.LocalSnapshot(loc.root, ref)
	if err != nil {
		return nil, fmt.Errorf("local snapshot: %w", err)
	}

	return out, nil
}

func (*localStore) ResolveRef(
	_ context.Context,
	requestedVersion string,
) (storedomain.RefInfo, error) {
	return storedomain.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: requestedVersion,
		SourceRef:        "refs/heads/" + testMainBranch,
		ResolvedCommit:   "deadbeef",
		DefaultBranch:    testMainBranch,
	}, nil
}

func (*mockGitOps) BranchExists(context.Context, string) (bool, error) {
	return false, nil
}

func (*mockGitOps) CheckoutBranch(context.Context, string) error      { return nil }
func (*mockGitOps) Commit(context.Context, string) error              { return nil }
func (*mockGitOps) CreateOrResetBranch(context.Context, string) error { return nil }
func (mockGitOps *mockGitOps) DefaultBranch(context.Context) (string, error) {
	mockGitOps.defaultBranchCalls++

	if mockGitOps.defaultBranch != consts.Empty {
		return mockGitOps.defaultBranch, nil
	}

	return testMainBranch, nil
}

func (mockGitOps *mockGitOps) HasUnrelatedChanges(context.Context, pathSet) (bool, error) {
	return mockGitOps.unrelated, nil
}

func (*mockGitOps) LastCommitMessage(context.Context, string) (string, error) {
	return consts.Empty, nil
}

func (*mockGitOps) Push(context.Context, string) error               { return nil }
func (*mockGitOps) PushForceWithLease(context.Context, string) error { return nil }
func (*mockGitOps) Stage(context.Context, []string) error            { return nil }

func (mpr *mockPR) CreatePR(_ context.Context, req *prdomain.CreatePRRequest) (*pullReq, error) {
	mpr.createdBase = req.Base

	if mpr.create != nil {
		return mpr.create, nil
	}

	return &pullReq{Number: consts.Index99, URL: "https://example/pr/99"}, nil
}

func (mpr *mockPR) FindOpenPR(_ context.Context, branch, base string) (*pullReq, error) {
	iox.Discard(branch)

	mpr.lastBase = base

	return mpr.find, nil
}

func (mpr *mockPR) UpdatePRBody(context.Context, int, string) error {
	mpr.updated++

	return nil
}
