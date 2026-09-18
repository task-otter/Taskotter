// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"errors"
	"testing"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prports "github.com/task-otter/Taskotter/internal/features/pr/ports"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
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

	refInfo          = storedomain.RefInfo
	snap             = storedomain.Snapshot
	pullReq          = prdomain.PullRequest
	writeOutputsArgs struct {
		Cfg        *config.Config
		Result     *service.Result
		OutputPath string
	}
)

const (
	testMainBranch     = "main"
	testRepository     = "owner/repo"
	testTargetFolder   = "taskfiles"
	testDevelopBranch  = "develop"
	testReleaseBranch  = "release/2026"
	testStoreVersion   = "v1.2.3"
	testPRURLSeven     = "https://example/pr/7"
	testSourceSHA      = "abc123"
	emptyJSONArray     = "[]"
	wantErrText        = "expected error"
	testPullRequestURL = "https://example.com/pull/42"
	testPRNumber42     = "42"
)

var (
	errTestBoom = errors.New("test boom")

	_ gitports.Brancher  = (*mockGitOps)(nil)
	_ gitports.Indexer   = (*mockGitOps)(nil)
	_ gitports.Publisher = (*mockGitOps)(nil)
	_ gitports.Workspace = (*mockWorkspace)(nil)
	_ prports.PRClient   = (*mockPR)(nil)
)
