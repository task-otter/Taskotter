// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prports "github.com/task-otter/Taskotter/internal/features/pr/ports"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	syncprepare "github.com/task-otter/Taskotter/internal/features/sync/prepare"
	rundomain "github.com/task-otter/Taskotter/internal/features/syncrun/domain"
	"github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	localStore struct {
		resolveErr  error
		downloadErr error
		cleanupErr  error
		root        string
	}

	pathSet = map[string]struct{}

	mockGitOps struct {
		errUnrelated       error
		errDefaultBranch   error
		errBranchExists    error
		errLastCommit      error
		errCheckout        error
		errStage           error
		errCommit          error
		errPush            error
		defaultBranch      string
		lastCommitMsg      string
		defaultBranchCalls int
		unrelated          bool
		branchExists       bool
	}

	mockPR struct {
		find        *prdomain.PullRequest
		findErr     error
		createErr   error
		updateErr   error
		lastBase    string
		createdBase string
		updated     int
	}

	mockWorkspace struct {
		credErr   error
		safeCalls int
	}

	assertPRUpdatedInput struct {
		t        *testing.T
		pullReq  *mockPR
		gitOps   *mockGitOps
		result   *rundomain.Result
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

	testOrchInput struct {
		t       *testing.T
		store   *localStore
		gitOps  *mockGitOps
		gitWork *mockWorkspace
		pullReq prports.PRClient
	}

	failEnv struct {
		store *localStore
		git   *mockGitOps
		work  *mockWorkspace
		pr    *mockPR
		cfg   *config.Config
		deps  *service.Deps
		orch  *service.Orchestrator
	}

	namedFailCase struct {
		setup func(*failEnv)
		name  string
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

	testStoreVersion = "v1.2.3"

	testPRURLSeven = "https://example/pr/7"

	testSourceSHA = "abc123"

	emptyJSONArray = "[]"

	wantErrText = "expected error"
)

var (
	errTestBoom = errors.New("test boom")

	_ gitports.Brancher  = (*mockGitOps)(nil)
	_ gitports.Indexer   = (*mockGitOps)(nil)
	_ gitports.Publisher = (*mockGitOps)(nil)
	_ gitports.Workspace = (*mockWorkspace)(nil)
	_ prports.PRClient   = (*mockPR)(nil)
)

// TestOrchestratorRunReportsFailures verifies the expected behavior.
func TestOrchestratorRunReportsFailures(t *testing.T) {
	t.Parallel()
	runNamedFailCases(t, runFailCases())
}

// TestOrchestratorHookFailures verifies the expected behavior.
func TestOrchestratorHookFailures(t *testing.T) {
	t.Parallel()
	runNamedFailCases(t, hookFailCases())
}

func applyApplyPlanFail(env *failEnv) {
	env.deps.ApplyPlan = func(*syncdomain.Plan, *syncdomain.SyncInput) error {
		return errTestBoom
	}
}

func applyBuildPlanFail(env *failEnv) {
	env.deps.BuildPlan = func(*syncdomain.SyncInput) (*syncdomain.Plan, error) {
		return nil, errTestBoom
	}
}

func applyPrepareFail(env *failEnv) {
	env.deps.PrepareSyncInput = func(*syncprepare.SyncInputArgs) (syncdomain.SyncInput, error) {
		return syncdomain.SyncInput{}, errTestBoom
	}
}

func applyResolveAllFail(env *failEnv) {
	env.deps.ResolveAll = func(*resolvesvc.ResolveAllInput) ([]resolvesvc.Resolution, error) {
		return nil, errTestBoom
	}
}

func applyResolveDepsFail(env *failEnv) {
	env.deps.ResolveTransitive = func([]string, map[string][]string) ([]string, error) {
		return nil, errTestBoom
	}
}

func hookFailCases() []namedFailCase {
	return []namedFailCase{
		{name: "prepare", setup: applyPrepareFail},
		{name: "build_plan", setup: applyBuildPlanFail},
		{name: "apply_plan", setup: applyApplyPlanFail},
		{name: "resolve_all", setup: applyResolveAllFail},
		{name: "resolve_deps", setup: applyResolveDepsFail},
	}
}

func readyFailEnv(t *testing.T) *failEnv {
	t.Helper()

	workspace := workspaceWithRootTaskfile(t)
	initGitWorkspace(t, workspace)

	return bindFailEnv(t, workspace)
}

func bindFailEnv(t *testing.T, workspace string) *failEnv {
	t.Helper()

	gitOps := newMockGitOps(false, testMainBranch)
	store := newLocalStore(t)
	work := newMockWorkspace()
	pullReq := newMockPR(nil)

	deps, orch := newTestOrchestratorParts(&testOrchInput{
		t: t, store: store, gitOps: gitOps, gitWork: work, pullReq: pullReq,
	})

	return &failEnv{
		store: store,
		git:   gitOps,
		work:  work,
		pr:    pullReq,
		cfg:   testConfigWithRepo(workspace),
		deps:  deps,
		orch:  orch,
	}
}

func runFailCases() []namedFailCase {
	return []namedFailCase{
		{name: "resolve_ref", setup: setupResolveRefErr},
		{name: "download", setup: setupDownloadErr},
		{name: "unknown_task", setup: setupUnknownTask},
		{name: "credentials", setup: setupCredErr},
		{name: "unrelated_err", setup: setupUnrelatedErr},
		{name: "default_branch", setup: setupDefaultBranchErr},
		{name: "foreign_branch", setup: setupForeignBranch},
		{name: "checkout", setup: setupCheckoutErr},
		{name: "stage", setup: setupStageErr},
		{name: "commit", setup: setupCommitErr},
		{name: "push", setup: setupPushErr},
		{name: "create_pr", setup: setupCreatePRErr},
		{name: "find_pr", setup: setupFindPRErr},
		{name: "update_pr", setup: setupUpdatePRErr},
	}
}

func runNamedFailCases(t *testing.T, cases []namedFailCase) {
	t.Helper()

	for index := range cases {
		failCase := cases[index]

		t.Run(failCase.name, func(t *testing.T) {
			t.Parallel()
			runNamedFailOnce(t, failCase)
		})
	}
}

func runNamedFailOnce(t *testing.T, failCase namedFailCase) {
	t.Helper()

	env := readyFailEnv(t)
	failCase.setup(env)
	assertRunFails(t, env.orch, env.cfg)
}

func setupResolveRefErr(env *failEnv) { env.store.resolveErr = errTestBoom }

func setupDownloadErr(env *failEnv) { env.store.downloadErr = errTestBoom }

func setupCredErr(env *failEnv) { env.work.credErr = errTestBoom }

func setupUnrelatedErr(env *failEnv) { env.git.errUnrelated = errTestBoom }

func setupCheckoutErr(env *failEnv) { env.git.errCheckout = errTestBoom }

func setupStageErr(env *failEnv) { env.git.errStage = errTestBoom }

func setupCommitErr(env *failEnv) { env.git.errCommit = errTestBoom }

func setupPushErr(env *failEnv) { env.git.errPush = errTestBoom }

func setupCreatePRErr(env *failEnv) { env.pr.createErr = errTestBoom }

func setupFindPRErr(env *failEnv) { env.pr.findErr = errTestBoom }

func setupDefaultBranchErr(env *failEnv) {
	env.git.errDefaultBranch = errTestBoom
}

func setupForeignBranch(env *failEnv) {
	env.git.branchExists = true
	env.git.lastCommitMsg = "foreign commit"
}

func setupUnknownTask(env *failEnv) {
	env.cfg.Tasks = []string{"missing-task"}
}

func setupUpdatePRErr(env *failEnv) {
	env.pr.find = &prdomain.PullRequest{Number: consts.IndexSeven, URL: testPRURLSeven}
	env.pr.updateErr = errTestBoom
}

// TestOrchestratorCreatesPRAgainstTriggerBranch verifies the expected behavior.
func TestOrchestratorCreatesPRAgainstTriggerBranch(t *testing.T) {
	t.Parallel()

	pullReq := newMockPR(nil)
	gitOps := newMockGitOps(false, consts.Empty)
	runGitRepoOrchestrator(&gitRepoRunInput{
		t: t, gitOps: gitOps, pr: pullReq, cfg: testConfigWithBaseBranch(testReleaseBranch),
	})

	assertPRCreated(&assertPRCreatedInput{
		t: t, pullReq: pullReq, gitOps: gitOps,
		wantBase: testReleaseBranch, wantDefaultBranchCalls: consts.IndexZero,
	})
}

// TestOrchestratorCreatesPRWithResolvedBase verifies the expected behavior.
func TestOrchestratorCreatesPRWithResolvedBase(t *testing.T) {
	t.Parallel()

	pullReq := newMockPR(nil)
	gitOps := newMockGitOps(false, testDevelopBranch)
	runGitRepoOrchestrator(&gitRepoRunInput{t: t, gitOps: gitOps, pr: pullReq, cfg: nil})

	if pullReq.createdBase != testDevelopBranch {
		t.Fatalf("created PR base = %q, want develop", pullReq.createdBase)
	}
}

// TestOrchestratorLogsDependencies verifies the expected behavior.
func TestOrchestratorLogsDependencies(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfigEslintPnpm(workspace)
	result := runOrchestrator(t, newTestOrchestrator(t, nil, nil), cfg)

	if result.StoreVersion != testStoreVersion {
		t.Fatalf("StoreVersion = %q, want %s", result.StoreVersion, testStoreVersion)
	}

	if result.ResolvedDependencies == emptyJSONArray {
		t.Fatal("expected non-empty resolved dependencies")
	}
}

// TestOrchestratorNoChangeAfterApply verifies the expected behavior.
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

// TestOrchestratorPreparesGitWorkspace verifies the expected behavior.
func TestOrchestratorPreparesGitWorkspace(t *testing.T) {
	t.Parallel()

	gitWork := newMockWorkspace()
	runPreparedGitWorkspace(t, gitWork)

	if gitWork.safeCalls != consts.IndexOne {
		t.Fatalf("EnsureSafeDirectory calls = %d", gitWork.safeCalls)
	}
}

// TestOrchestratorReportsCleanupFailureQuietly verifies the expected behavior.
func TestOrchestratorReportsCleanupFailureQuietly(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	store := newLocalStore(t)

	store.cleanupErr = errTestBoom

	deps, orch := newTestOrchestratorParts(&testOrchInput{
		t: t, store: store, gitOps: nil, gitWork: nil, pullReq: nil,
	})

	iox.Discard(deps)

	runOrchestrator(t, orch, cfg)
}

// TestOrchestratorUnrelatedDirtyTreeFails verifies the expected behavior.
func TestOrchestratorUnrelatedDirtyTreeFails(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	orchestrator := newTestOrchestrator(t, newMockGitOps(true, consts.Empty), nil)

	initGitWorkspace(t, workspace)

	assertRunFails(t, orchestrator, cfg)
	assertPathNotExist(t, filepath.Join(workspace, testTargetFolder, "go", "Taskfile.yml"))
}

// TestOrchestratorSkipsPRWithoutClient verifies the expected behavior.
func TestOrchestratorSkipsPRWithoutClient(t *testing.T) {
	t.Parallel()

	gitOps := newMockGitOps(false, testMainBranch)
	runGitRepoOrchestrator(&gitRepoRunInput{t: t, gitOps: gitOps, pr: nil, cfg: nil})
}

// TestOrchestratorUpdatesExistingPR verifies the expected behavior.
func TestOrchestratorUpdatesExistingPR(t *testing.T) {
	t.Parallel()

	pullReq := newMockPR(&prdomain.PullRequest{Number: consts.IndexSeven, URL: testPRURLSeven})
	gitOps := newMockGitOps(false, testMainBranch)
	result := runGitRepoOrchestrator(&gitRepoRunInput{t: t, gitOps: gitOps, pr: pullReq, cfg: nil})

	assertPRUpdated(&assertPRUpdatedInput{
		t: t, pullReq: pullReq, gitOps: gitOps, result: result,
		wantBase: testMainBranch, wantNum: "7",
	})
}

func runPreparedGitWorkspace(t *testing.T, gitWork *mockWorkspace) {
	t.Helper()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfigWithRepo(workspace)
	gitOps := newMockGitOps(false, testMainBranch)

	initGitWorkspace(t, workspace)

	deps, orch := newTestOrchestratorParts(&testOrchInput{
		t: t, store: newLocalStore(t), gitOps: gitOps, gitWork: gitWork, pullReq: newMockPR(nil),
	})

	iox.Discard(deps)

	runOrchestrator(t, orch, cfg)
}

// TestOrchestratorRunRequiresConfiguration verifies the expected behavior.
func TestOrchestratorRunRequiresConfiguration(t *testing.T) {
	t.Parallel()

	result, err := new(service.Orchestrator).Run(t.Context(), &config.Config{})
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected configuration error")
	}
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

func assertRunFails(t *testing.T, orch *service.Orchestrator, cfg *config.Config) {
	t.Helper()

	result, err := orch.Run(t.Context(), cfg)
	iox.Discard(result)

	if err == nil {
		t.Fatal(wantErrText)
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

func newLocalStore(t *testing.T) *localStore {
	t.Helper()

	return &localStore{
		root:        fixtureRoot(t),
		resolveErr:  nil,
		downloadErr: nil,
		cleanupErr:  nil,
	}
}

func newMockGitOps(unrelated bool, defaultBranch string) *mockGitOps {
	return &mockGitOps{
		unrelated:          unrelated,
		defaultBranch:      defaultBranch,
		defaultBranchCalls: consts.IndexZero,
		lastCommitMsg:      gitports.SyncCommitMessage,
		branchExists:       false,
		errUnrelated:       nil,
		errDefaultBranch:   nil,
		errBranchExists:    nil,
		errLastCommit:      nil,
		errCheckout:        nil,
		errStage:           nil,
		errCommit:          nil,
		errPush:            nil,
	}
}

func newMockPR(find *prdomain.PullRequest) *mockPR {
	return &mockPR{
		find:        find,
		findErr:     nil,
		createErr:   nil,
		updateErr:   nil,
		updated:     consts.IndexZero,
		lastBase:    consts.Empty,
		createdBase: consts.Empty,
	}
}

func newMockWorkspace() *mockWorkspace {
	return &mockWorkspace{credErr: nil, safeCalls: consts.IndexZero}
}

func newTestOrchestrator(
	t *testing.T,
	gitOps *mockGitOps,
	pullReq prports.PRClient,
) *service.Orchestrator {
	t.Helper()

	deps, orch := newTestOrchestratorParts(&testOrchInput{
		t:       t,
		store:   newLocalStore(t),
		gitOps:  gitOps,
		gitWork: nil,
		pullReq: pullReq,
	})

	iox.Discard(deps)

	return orch
}

func newTestOrchestratorParts(input *testOrchInput) (*service.Deps, *service.Orchestrator) {
	input.t.Helper()

	deps := &service.Deps{
		Logger:       nil,
		StoreClient:  input.store,
		GitClient:    nil,
		GitBrancher:  input.gitOps,
		GitIndexer:   input.gitOps,
		GitPublisher: input.gitOps,
		PRClient:     input.pullReq,
	}

	if input.gitWork != nil {
		deps.GitClient = input.gitWork
	}

	return deps, service.NewOrchestrator(deps)
}

func resolveGitRepoConfig(cfg *config.Config, workspace string) *config.Config {
	if cfg == nil {
		return testConfigWithRepo(workspace)
	}

	if cfg.Workspace == consts.Empty {
		cfg.Workspace = workspace
	}

	return cfg
}

func runGitRepoOrchestrator(input *gitRepoRunInput) *rundomain.Result {
	input.t.Helper()

	workspace := workspaceWithRootTaskfile(input.t)
	cfg := resolveGitRepoConfig(input.cfg, workspace)

	initGitWorkspace(input.t, workspace)

	return runOrchestrator(input.t, newTestOrchestrator(input.t, input.gitOps, input.pr), cfg)
}

func runOrchestrator(
	t *testing.T,
	orchestrator *service.Orchestrator,
	cfg *config.Config,
) *rundomain.Result {
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

func testConfigEslintPnpm(workspace string) *config.Config {
	cfg := testConfig(workspace)

	cfg.Tasks = []string{consts.EslintNode}
	cfg.NodePackageManager = config.PMPnpm
	cfg.StoreVersion = testStoreVersion

	return cfg
}

func testConfigWithBaseBranch(baseBranch string) *config.Config {
	cfg := testConfigWithRepo(consts.Empty)

	cfg.BaseBranch = baseBranch

	return cfg
}

func testConfigWithRepo(workspace string) *config.Config {
	cfg := testConfig(workspace)

	cfg.Repository = testRepository

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
	if loc.downloadErr != nil {
		return nil, loc.downloadErr
	}

	out, err := storesvc.LocalSnapshot(loc.root, ref)
	if err != nil {
		return nil, fmt.Errorf("local snapshot: %w", err)
	}

	if loc.cleanupErr != nil {
		out.Cleanup = func() error { return loc.cleanupErr }
	}

	return out, nil
}

func (loc *localStore) ResolveRef(
	_ context.Context,
	requestedVersion string,
) (storedomain.RefInfo, error) {
	if loc.resolveErr != nil {
		return storedomain.RefInfo{}, loc.resolveErr
	}

	return storedomain.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: requestedVersion,
		SourceRef:        "refs/heads/" + testMainBranch,
		ResolvedCommit:   "deadbeef",
		DefaultBranch:    testMainBranch,
	}, nil
}

func (ops *mockGitOps) BranchExists(context.Context, string) (bool, error) {
	if ops.errBranchExists != nil {
		return false, ops.errBranchExists
	}

	return ops.branchExists, nil
}

func (*mockGitOps) CheckoutBranch(context.Context, string) error { return nil }

func (ops *mockGitOps) Commit(context.Context, string) error {
	return ops.errCommit
}

func (ops *mockGitOps) CreateOrResetBranch(context.Context, string) error {
	return ops.errCheckout
}

func (ops *mockGitOps) DefaultBranch(context.Context) (string, error) {
	ops.defaultBranchCalls++

	if ops.errDefaultBranch != nil {
		return consts.Empty, ops.errDefaultBranch
	}

	if ops.defaultBranch != consts.Empty {
		return ops.defaultBranch, nil
	}

	return testMainBranch, nil
}

func (ops *mockGitOps) HasUnrelatedChanges(context.Context, pathSet) (bool, error) {
	if ops.errUnrelated != nil {
		return false, ops.errUnrelated
	}

	return ops.unrelated, nil
}

func (ops *mockGitOps) LastCommitMessage(context.Context, string) (string, error) {
	if ops.errLastCommit != nil {
		return consts.Empty, ops.errLastCommit
	}

	return ops.lastCommitMsg, nil
}

func (*mockGitOps) Push(context.Context, string) error { return nil }

func (ops *mockGitOps) PushForceWithLease(context.Context, string) error {
	return ops.errPush
}

func (ops *mockGitOps) Stage(context.Context, []string) error {
	return ops.errStage
}

func (mpr *mockPR) CreatePR(_ context.Context, req *prdomain.CreatePRRequest) (*pullReq, error) {
	mpr.createdBase = req.Base

	if mpr.createErr != nil {
		return nil, mpr.createErr
	}

	return &pullReq{Number: consts.Index99, URL: "https://example/pr/99"}, nil
}

func (mpr *mockPR) FindOpenPR(_ context.Context, branch, base string) (*pullReq, error) {
	iox.Discard(branch)

	mpr.lastBase = base

	if mpr.findErr != nil {
		return nil, mpr.findErr
	}

	return mpr.find, nil
}

func (mpr *mockPR) UpdatePRBody(context.Context, int, string) error {
	if mpr.updateErr != nil {
		return mpr.updateErr
	}

	mpr.updated++

	return nil
}

func (work *mockWorkspace) ConfigureCredentials(context.Context, string, string) error {
	return work.credErr
}

func (work *mockWorkspace) EnsureSafeDirectory() {
	work.safeCalls++
}
