// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:revive // Orchestration dependencies are intentionally explicit at the composition boundary.
package service

import (
	"context"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prports "github.com/task-otter/Taskotter/internal/features/pr/ports"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	syncprepare "github.com/task-otter/Taskotter/internal/features/sync/prepare"
	rundomain "github.com/task-otter/Taskotter/internal/features/syncrun/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

type (
	storeClient interface {
		ResolveRef(ctx context.Context, requestedVersion string) (storedomain.RefInfo, error)
		DownloadSnapshot(
			ctx context.Context,
			ref *storedomain.RefInfo,
		) (*storedomain.Snapshot, error)
	}

	// Deps describes the deps.
	Deps = struct {
		Logger            *logging.Logger
		StoreClient       storeClient
		GitClient         gitports.Workspace
		GitBrancher       gitports.Brancher
		GitIndexer        gitports.Indexer
		GitPublisher      gitports.Publisher
		PRClient          prports.PRClient
		PrepareSyncInput  PrepareSyncInputFn
		BuildPlan         BuildPlanFn
		ApplyPlan         ApplyPlanFn
		ResolveAll        ResolveAllFn
		ResolveTransitive ResolveTransitiveFn
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
		result    *rundomain.Result
	}

	createPRInput = struct {
		result        *rundomain.Result
		branch        string
		defaultBranch string
		body          string
	}

	finishSyncInput = struct {
		cfg       *config.Config
		plan      *syncdomain.Plan
		syncInput *syncdomain.SyncInput
		ref       *storedomain.RefInfo
		result    *rundomain.Result
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
		result    *rundomain.Result
	}

	prPhaseInput = struct {
		cfg           *config.Config
		plan          *syncdomain.Plan
		ref           *storedomain.RefInfo
		result        *rundomain.Result
		defaultBranch string
	}

	updatePRInput = struct {
		existing *prdomain.PullRequest
		result   *rundomain.Result
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

	summaryInput = struct {
		Log    *logging.Logger
		Cfg    *config.Config
		Plan   *syncdomain.Plan
		Result *rundomain.Result
		PRURL  string
	}

	// PrepareSyncInputFn defines the function signature used for this orchestration step.
	PrepareSyncInputFn func(*syncprepare.SyncInputArgs) (syncdomain.SyncInput, error)
	// BuildPlanFn defines the function signature used for this orchestration step.
	BuildPlanFn func(*syncdomain.SyncInput) (*syncdomain.Plan, error)
	// ApplyPlanFn defines the function signature used for this orchestration step.
	ApplyPlanFn func(*syncdomain.Plan, *syncdomain.SyncInput) error
	// ResolveAllFn defines the function signature used for this orchestration step.
	ResolveAllFn func(*resolvesvc.ResolveAllInput) ([]resolvesvc.Resolution, error)
	// ResolveTransitiveFn defines the function signature used for this orchestration step.
	ResolveTransitiveFn func([]string, map[string][]string) ([]string, error)

	// RunFn defines the function signature used for this orchestration step.
	RunFn func(context.Context, *config.Config) (*rundomain.Result, error)
	// Orchestrator describes the orchestrator.
	//nolint:reusability // This named function type defines the single orchestration entry point.
	Orchestrator RunFn
)
