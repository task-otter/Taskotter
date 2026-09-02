// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"path/filepath"
	"testing"

	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

// TestOrchestratorCreatesPRAgainstTriggerBranch verifies the PR targets the configured base branch.
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

// TestOrchestratorCreatesPRWithResolvedBase verifies the PR base resolves to the default branch.
func TestOrchestratorCreatesPRWithResolvedBase(t *testing.T) {
	t.Parallel()

	pullReq := newMockPR(nil)
	gitOps := newMockGitOps(false, testDevelopBranch)
	runGitRepoOrchestrator(&gitRepoRunInput{t: t, gitOps: gitOps, pr: pullReq, cfg: nil})

	if pullReq.createdBase != testDevelopBranch {
		t.Fatalf("created PR base = %q, want develop", pullReq.createdBase)
	}
}

// TestOrchestratorLogsDependencies verifies transitive dependency modules are resolved.
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

// TestOrchestratorPreparesGitWorkspace verifies credentials are configured when GitClient is set.
func TestOrchestratorPreparesGitWorkspace(t *testing.T) {
	t.Parallel()

	gitWork := newMockWorkspace()
	runPreparedGitWorkspace(t, gitWork)

	if gitWork.safeCalls != consts.IndexOne {
		t.Fatalf("EnsureSafeDirectory calls = %d", gitWork.safeCalls)
	}
}

// TestOrchestratorReportsCleanupFailureQuietly verifies snapshot cleanup errors are logged only.
func TestOrchestratorReportsCleanupFailureQuietly(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	store := newLocalStore(t)

	store.cleanupErr = errTestBoom

	orch := newTestOrchestratorParts(&testOrchInput{
		t: t, store: store, gitOps: nil, gitWork: nil, pullReq: nil,
	})
	runOrchestrator(t, orch, cfg)
}

// TestOrchestratorUnrelatedDirtyTreeFails verifies unrelated dirty changes fail the run.
func TestOrchestratorUnrelatedDirtyTreeFails(t *testing.T) {
	t.Parallel()

	workspace := workspaceWithRootTaskfile(t)
	cfg := testConfig(workspace)
	orchestrator := newTestOrchestrator(t, newMockGitOps(true, consts.Empty), nil)

	initGitWorkspace(t, workspace)

	assertRunFails(t, orchestrator, cfg)
	assertPathNotExist(t, filepath.Join(workspace, testTargetFolder, "go", "Taskfile.yml"))
}

// TestOrchestratorSkipsPRWithoutClient verifies git sync succeeds when PRClient is nil.
func TestOrchestratorSkipsPRWithoutClient(t *testing.T) {
	t.Parallel()

	gitOps := newMockGitOps(false, testMainBranch)
	runGitRepoOrchestrator(&gitRepoRunInput{t: t, gitOps: gitOps, pr: nil, cfg: nil})
}

// TestOrchestratorUpdatesExistingPR verifies an existing open pull request is updated.
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

	orch := newTestOrchestratorParts(&testOrchInput{
		t: t, store: newLocalStore(t), gitOps: gitOps, gitWork: gitWork, pullReq: newMockPR(nil),
	})
	runOrchestrator(t, orch, cfg)
}
