// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"testing"

	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

// TestOrchestratorRunReportsFailures verifies pipeline error branches are wrapped.
func TestOrchestratorRunReportsFailures(t *testing.T) {
	t.Parallel()
	runNamedFailCases(t, runFailCases())
}

// TestOrchestratorHookFailures verifies sync and resolve seams wrap injected errors.
func TestOrchestratorHookFailures(t *testing.T) {
	t.Parallel()
	runNamedFailCases(t, hookFailCases())
}

func applyApplyPlanFail(env *failEnv) {
	env.orch.ApplyPlan = func(*syncdomain.Plan, *syncdomain.SyncInput) error {
		return errTestBoom
	}
}

func applyBuildPlanFail(env *failEnv) {
	env.orch.BuildPlan = func(*syncdomain.SyncInput) (*syncdomain.Plan, error) {
		return nil, errTestBoom
	}
}

func applyPrepareFail(env *failEnv) {
	env.orch.PrepareSyncInput = func(*syncsvc.PrepareSyncInputArgs) (syncdomain.SyncInput, error) {
		return syncdomain.SyncInput{}, errTestBoom
	}
}

func applyResolveAllFail(env *failEnv) {
	env.orch.ResolveAll = func(*resolvesvc.ResolveAllInput) ([]resolvesvc.Resolution, error) {
		return nil, errTestBoom
	}
}

func applyResolveDepsFail(env *failEnv) {
	env.orch.ResolveTransitive = func([]string, map[string][]string) ([]string, error) {
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

	return &failEnv{
		store: store,
		git:   gitOps,
		work:  work,
		pr:    pullReq,
		cfg:   testConfigWithRepo(workspace),
		orch: newTestOrchestratorParts(&testOrchInput{
			t: t, store: store, gitOps: gitOps, gitWork: work, pullReq: pullReq,
		}),
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
func setupDownloadErr(env *failEnv)   { env.store.downloadErr = errTestBoom }
func setupCredErr(env *failEnv)       { env.work.credErr = errTestBoom }
func setupUnrelatedErr(env *failEnv)  { env.git.errUnrelated = errTestBoom }
func setupCheckoutErr(env *failEnv)   { env.git.errCheckout = errTestBoom }
func setupStageErr(env *failEnv)      { env.git.errStage = errTestBoom }
func setupCommitErr(env *failEnv)     { env.git.errCommit = errTestBoom }
func setupPushErr(env *failEnv)       { env.git.errPush = errTestBoom }
func setupCreatePRErr(env *failEnv)   { env.pr.createErr = errTestBoom }
func setupFindPRErr(env *failEnv)     { env.pr.findErr = errTestBoom }

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
