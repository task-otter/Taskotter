// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"context"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prports "github.com/task-otter/Taskotter/internal/features/pr/ports"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

type (
	prepareSyncInputFn func(*syncsvc.PrepareSyncInputArgs) (syncdomain.SyncInput, error)

	buildPlanFn func(*syncdomain.SyncInput) (*syncdomain.Plan, error)

	applyPlanFn func(*syncdomain.Plan, *syncdomain.SyncInput) error

	resolveAllFn func(*resolvesvc.ResolveAllInput) ([]resolvesvc.Resolution, error)

	resolveTransitiveFn func([]string, map[string][]string) ([]string, error)

	// storeClient resolves store refs and downloads snapshots for the sync pipeline.
	storeClient interface {
		ResolveRef(ctx context.Context, requestedVersion string) (storedomain.RefInfo, error)
		DownloadSnapshot(
			ctx context.Context,
			ref *storedomain.RefInfo,
		) (*storedomain.Snapshot, error)
	}

	// Deps holds collaborators for one sync run pipeline.
	Deps = struct {
		Logger            *logging.Logger
		StoreClient       storeClient
		GitClient         gitports.Workspace
		GitBrancher       gitports.Brancher
		GitIndexer        gitports.Indexer
		GitPublisher      gitports.Publisher
		PRClient          prports.PRClient
		PrepareSyncInput  prepareSyncInputFn
		BuildPlan         buildPlanFn
		ApplyPlan         applyPlanFn
		ResolveAll        resolveAllFn
		ResolveTransitive resolveTransitiveFn
	}

	// Orchestrator coordinates store, git, and GitHub operations for a sync run.
	Orchestrator struct {
		run func(context.Context, *config.Config) (*Result, error)
	}

	buildPlanInput = struct {
		cfg         *config.Config
		snapshot    *storedomain.Snapshot
		resolutions []resolvesvc.Resolution
		depSources  []string
	}

	changedPlanInput = struct {
		cfg       *config.Config
		plan      *syncdomain.Plan
		syncInput *syncdomain.SyncInput
		ref       *storedomain.RefInfo
		result    *Result
	}

	createPRInput = struct {
		result        *Result
		branch        string
		defaultBranch string
		body          string
	}

	finishSyncInput = struct {
		cfg       *config.Config
		plan      *syncdomain.Plan
		syncInput *syncdomain.SyncInput
		ref       *storedomain.RefInfo
		result    *Result
	}

	gitSyncStep struct {
		fn  func() error
		msg string
	}

	branchPair = struct {
		name          string
		defaultBranch string
	}

	planResult = struct {
		syncInput *syncdomain.SyncInput
		plan      *syncdomain.Plan
		result    *Result
	}

	prPhaseInput = struct {
		cfg           *config.Config
		plan          *syncdomain.Plan
		ref           *storedomain.RefInfo
		result        *Result
		defaultBranch string
	}

	updatePRInput = struct {
		existing *prdomain.PullRequest
		result   *Result
		body     string
	}

	prResolveInput = struct {
		phase    *prPhaseInput
		existing *prdomain.PullRequest
		body     string
	}

	plannedSyncInput = struct {
		cfg      *config.Config
		ref      *storedomain.RefInfo
		snapshot *storedomain.Snapshot
	}
	branchPlanIn = struct {
		inp       *changedPlanInput
		defBranch string
	}

	gitPlanIn = struct {
		cfg  *config.Config
		plan *syncPlan
	}

	fetchIn = struct {
		cfg *config.Config
	}

	planResultArgs = struct {
		cfg  *config.Config
		snap *snapInfo
		ref  *refInfo
	}

	refInfo  = storedomain.RefInfo
	snapInfo = storedomain.Snapshot
	pullReq  = prdomain.PullRequest
	resItem  = resolvesvc.Resolution
	syncIn   = syncdomain.SyncInput
	syncPlan = syncdomain.Plan
	planIn   = plannedSyncInput

	// Result captures sync outcomes for logging, GitHub Actions output, and PR metadata.
	Result = struct {
		Plan                 *syncdomain.Plan
		Ref                  storedomain.RefInfo
		StoreVersion         string
		SourceRef            string
		SourceSHA            string
		TargetFolder         string
		ResolvedTasksJSON    string
		ResolvedDependencies string
		PullRequestNumber    string
		PullRequestURL       string
		Changed              bool
	}

	// ResolvedTask is the JSON representation of a resolved task module mapping.
	ResolvedTask struct {
		SourceModule      string
		DestinationModule string
		Path              string
	}

	summaryInput = struct {
		Log    *logging.Logger
		Cfg    *config.Config
		Plan   *syncdomain.Plan
		Result *Result
		PRURL  string
	}
)
