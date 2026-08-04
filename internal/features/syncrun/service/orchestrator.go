// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package service orchestrates the sync, apply, and pull request workflow.
//
//nolint:funcorder // methods follow pipeline phases, not strict alphabetical order
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prservice "github.com/task-otter/Taskotter/internal/features/pr/service"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/features/syncrun/ports"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

type (

	// Orchestrator coordinates store, git, and GitHub operations for a sync run.
	Orchestrator struct {
		Logger       *logging.Logger
		StoreClient  ports.StoreClient
		GitClient    gitports.Workspace
		GitBrancher  ports.Brancher
		GitIndexer   ports.Indexer
		GitPublisher ports.Publisher
		PRClient     ports.PRClient
	}

	buildPlanInput struct {
		cfg         *config.Config
		snapshot    *storedomain.Snapshot
		resolutions []resolvesvc.Resolution
		depSources  []string
	}

	changedPlanInput struct {
		cfg       *config.Config
		plan      *syncdomain.Plan
		syncInput *syncdomain.SyncInput
		ref       *storedomain.RefInfo
		result    *Result
	}

	createPRInput struct {
		result        *Result
		branch        string
		defaultBranch string
		body          string
	}

	finishSyncInput struct {
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

	planResult struct {
		syncInput *syncdomain.SyncInput
		plan      *syncdomain.Plan
		result    *Result
	}

	prPhaseInput struct {
		cfg           *config.Config
		plan          *syncdomain.Plan
		ref           *storedomain.RefInfo
		result        *Result
		defaultBranch string
	}

	updatePRInput struct {
		existing *prdomain.PullRequest
		result   *Result
		body     string
	}

	prResolveInput struct {
		phase    *prPhaseInput
		existing *prdomain.PullRequest
		body     string
	}

	plannedSyncInput struct {
		cfg      *config.Config
		ref      *storedomain.RefInfo
		snapshot *storedomain.Snapshot
	}
	branchPlanIn struct {
		inp       *changedPlanInput
		defBranch string
	}

	gitPlanIn struct {
		cfg  *config.Config
		plan *syncPlan
	}

	fetchIn struct {
		cfg *config.Config
	}

	planResultArgs struct {
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
)

const (
	fmtArrow = "%s -> %s"

	fmtGroupErr = "%s: %w"

	fmtTargetFolder = "Target folder: %s"

	groupSummary = "Summary"

	errFmtBuildSyncPlan = "build sync plan: %w"

	errFmtCheckUnrelatedChanges = "check unrelated changes: %w"

	errFmtFindOpenPullRequest = "find open pull request: %w"

	errFmtResolveStoreRef = "resolve store ref: %w"

	errFmtResolveDependencies = "resolve dependencies: %w"

	errFmtResolveRequestedModules = "resolve requested modules: %w"

	errFmtResolveTransitiveDeps = "resolve transitive dependencies: %w"

	fmtRunGroupedErr = "run grouped: %w"
)

var errUnrelatedChanges = errors.New("unrelated uncommitted changes detected in workspace")

func runGitSyncSteps(steps []gitSyncStep) error {
	for i := range steps {
		step := &steps[i]

		err := step.fn()
		if err != nil {
			return fmt.Errorf(fmtGroupErr, step.msg, err)
		}
	}

	return nil
}

func runGrouped(logger *logging.Logger, title string, action func() error) error {
	var err error

	logger.Group(title, func() {
		err = action()
	})

	if err != nil {
		return fmt.Errorf(fmtGroupErr, title, err)
	}

	return nil
}

func assignGrouped(logger *logging.Logger, title string, assign func() error) error {
	err := runGrouped(logger, title, assign)
	if err != nil {
		return fmt.Errorf(fmtRunGroupedErr, err)
	}

	return nil
}

func runGroupNoResult(logger *logging.Logger, title string, groupFn func() error) error {
	err := assignGrouped(logger, title, groupFn)
	if err != nil {
		return fmt.Errorf(fmtRunGroupedErr, err)
	}

	return nil
}

func sourceModulesOf(resolutions []resolvesvc.Resolution) []string {
	requestedSources := make([]string, consts.IndexZero, len(resolutions))

	for i := range resolutions {
		requestedSources = append(requestedSources, resolutions[i].SourceModule)
	}

	return requestedSources
}

// Run executes the full sync pipeline.
func (orch *Orchestrator) Run(ctx context.Context, cfg *config.Config) (*Result, error) {
	orch.wireDefaults()

	result, err := orch.execPipeline(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("run: %w", err)
	}

	return result, nil
}

func (orch *Orchestrator) applyChangedPlan(ctx context.Context, input *changedPlanInput) error {
	defaultBranch, err := orch.maybeDefBranch(ctx, &gitPlanIn{cfg: input.cfg, plan: input.plan})
	if err != nil {
		return fmt.Errorf("resolve default branch: %w", err)
	}

	err = orch.applySyncChanges(ctx, &branchPlanIn{inp: input, defBranch: defaultBranch})
	if err != nil {
		return fmt.Errorf("apply sync changes: %w", err)
	}

	return nil
}

func (orch *Orchestrator) applySyncChanges(ctx context.Context, args *branchPlanIn) error {
	err := orch.copyTaskModules(args.inp.plan, args.inp.syncInput)
	if err != nil {
		return fmt.Errorf("copy task modules: %w", err)
	}

	err = orch.commitAndMaybePR(ctx, args)
	if err != nil {
		return fmt.Errorf("commit and maybe PR: %w", err)
	}

	return nil
}

func (orch *Orchestrator) buildPlanResult(inp *buildPlanInput, ref *refInfo) (planResult, error) {
	syncInput, plan, err := orch.buildSyncPlan(inp)
	if err != nil {
		return planResult{}, fmt.Errorf(errFmtBuildSyncPlan, err)
	}

	result := buildResult(inp.cfg, plan, ref)

	return planResult{syncInput: syncInput, plan: plan, result: result}, nil
}

//nolint:gocritic // single-line sig for whitespace
func (orch *Orchestrator) buildSyncPlan(inp *buildPlanInput) (*syncIn, *syncPlan, error) {
	syncInput, err := syncsvc.PrepareSyncInput(&syncsvc.PrepareSyncInputArgs{
		Cfg:         inp.cfg,
		Snapshot:    inp.snapshot,
		TaskfileOps: synctaskfile.Ops{},
		Resolutions: inp.resolutions,
		DepSources:  inp.depSources,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("prepare sync input: %w", err)
	}

	orch.logDestinationNormalization(&syncInput)

	plan, err := orch.compareManagedFiles(&syncInput)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtBuildSyncPlan, err)
	}

	return &syncInput, plan, nil
}

func (orch *Orchestrator) checkUnrelatedChanges(ctx context.Context, plan *syncPlan) error {
	allowed := gitports.AllowedPathSet(plan.StagePaths)

	unrelated, err := orch.GitIndexer.HasUnrelatedChanges(ctx, allowed)
	if err != nil {
		return fmt.Errorf(errFmtCheckUnrelatedChanges, err)
	}

	if unrelated {
		return errUnrelatedChanges
	}

	return nil
}

func (orch *Orchestrator) commitAndMaybePR(ctx context.Context, args *branchPlanIn) error {
	err := orch.maybeCommitPush(ctx, &gitPlanIn{cfg: args.inp.cfg, plan: args.inp.plan})
	if err != nil {
		return fmt.Errorf("commit and push: %w", err)
	}

	err = orch.runPRPhase(ctx, args)
	if err != nil {
		return fmt.Errorf("run PR phase: %w", err)
	}

	return nil
}

func (orch *Orchestrator) runPRPhase(ctx context.Context, args *branchPlanIn) error {
	err := orch.maybeCreateOrUpdatePR(ctx, &prPhaseInput{
		cfg:           args.inp.cfg,
		plan:          args.inp.plan,
		ref:           args.inp.ref,
		defaultBranch: args.defBranch,
		result:        args.inp.result,
	})
	if err != nil {
		return fmt.Errorf("create or update PR: %w", err)
	}

	return nil
}

func (orch *Orchestrator) closeSnapshotQuietly(snapshot *storedomain.Snapshot) {
	closeErr := snapshot.Close()
	if closeErr != nil {
		orch.Logger.Printf("close store snapshot: %v", closeErr)
	}
}

func logBuiltPlan(logger *logging.Logger, built *syncdomain.Plan) {
	logger.Printf("Changed: %t", built.Changed)
	logger.Printf(
		"Added: %d Updated: %d Removed: %d",
		len(built.Added),
		len(built.Updated),
		len(built.Removed),
	)
}

func (orch *Orchestrator) compareManagedFiles(syncInput *syncIn) (*syncPlan, error) {
	var plan *syncdomain.Plan

	err := assignGrouped(orch.Logger, "Compare managed files", func() error {
		built, planErr := syncsvc.BuildPlan(syncInput)
		if planErr != nil {
			return fmt.Errorf("build plan: %w", planErr)
		}

		logBuiltPlan(orch.Logger, built)

		plan = built

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("compare managed files: %w", err)
	}

	return plan, nil
}

func (orch *Orchestrator) copyTaskModules(plan *syncPlan, syncInput *syncIn) error {
	err := runGroupNoResult(orch.Logger, "Copy task modules", func() error {
		applyErr := syncsvc.ApplyPlan(plan, syncInput)
		if applyErr != nil {
			return fmt.Errorf("apply plan: %w", applyErr)
		}

		orch.Logger.Printf("Copied modules and validated generated YAML")

		return nil
	})
	if err != nil {
		return fmt.Errorf("apply sync plan: %w", err)
	}

	return nil
}

func (orch *Orchestrator) createNewPR(ctx context.Context, input *createPRInput) error {
	pullReq, err := orch.PRClient.CreatePR(ctx, &prdomain.CreatePRRequest{
		Branch: input.branch,
		Base:   input.defaultBranch,
		Body:   input.body,
	})
	if err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}

	input.result.PullRequestNumber = strconv.Itoa(pullReq.Number)
	input.result.PullRequestURL = pullReq.URL
	orch.Logger.Printf("Created pull request #%d", pullReq.Number)

	return nil
}

func (orch *Orchestrator) prBaseBranch(ctx context.Context, cfg *config.Config) (string, error) {
	if cfg.BaseBranch != consts.Empty {
		return cfg.BaseBranch, nil
	}

	defaultBranch, err := orch.GitBrancher.DefaultBranch(ctx)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve pull request base branch: %w", err)
	}

	return defaultBranch, nil
}

func (orch *Orchestrator) groupedSnap(ctx context.Context, ref *refInfo) (*snapInfo, error) {
	snap, downloadErr := orch.StoreClient.DownloadSnapshot(ctx, ref)
	if downloadErr != nil {
		return nil, fmt.Errorf("download snapshot: %w", downloadErr)
	}

	orch.Logger.Printf("Loaded store snapshot from %s", ref.ResolvedCommit)

	return snap, nil
}

func (orch *Orchestrator) downloadSnap(ctx context.Context, ref *refInfo) (*snapInfo, error) {
	var snapshot *storedomain.Snapshot

	err := assignGrouped(orch.Logger, "Download store", func() error {
		snap, downloadErr := orch.groupedSnap(ctx, ref)
		if downloadErr != nil {
			return fmt.Errorf("fetch grouped snapshot: %w", downloadErr)
		}

		snapshot = snap

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("download store: %w", err)
	}

	return snapshot, nil
}

func (orch *Orchestrator) ensureGitReadyForSync(ctx context.Context, cfg *config.Config) error {
	if orch.GitClient != nil {
		orch.GitClient.EnsureSafeDirectory()
	}

	gitports.WriteLocalIdentity()

	err := gitports.EnsureBranchOwned(ctx, orch.GitBrancher, cfg.BranchName)
	if err != nil {
		return fmt.Errorf("ensure branch owned: %w", err)
	}

	return nil
}

func (orch *Orchestrator) ensureLogger() {
	if orch.Logger == nil {
		orch.Logger = logging.New()
	}
}

func (orch *Orchestrator) execPipeline(ctx context.Context, cfg *config.Config) (*Result, error) {
	orch.logValidateInputs(cfg)

	result, err := orch.runPipeline(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("run pipeline: %w", err)
	}

	return result, nil
}

//nolint:gocritic // single-line sig for whitespace
func (orch *Orchestrator) getStore(ctx context.Context, in *fetchIn) (*refInfo, *snapInfo, error) {
	ref, err := orch.storeRef(ctx, in.cfg)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtResolveStoreRef, err)
	}

	snapshot, err := orch.downloadSnap(ctx, &ref)
	if err != nil {
		return nil, nil, fmt.Errorf("download store snapshot: %w", err)
	}

	orch.Logger.Group("Load module catalog", func() {
		orch.Logger.Printf("Catalog modules: %d", len(snapshot.Catalog))
	})

	return &ref, snapshot, nil
}

func (orch *Orchestrator) openPR(ctx context.Context, brn, def string) (*pullReq, error) {
	existing, err := orch.PRClient.FindOpenPR(ctx, brn, def)

	if err != nil && !errors.Is(err, prdomain.ErrPullRequestNotFound) {
		return nil, fmt.Errorf(errFmtFindOpenPullRequest, err)
	}

	return existing, nil
}

func (orch *Orchestrator) finishChangedPlan(ctx context.Context, input *finishSyncInput) error {
	err := orch.applyChangedPlan(ctx, &changedPlanInput{
		cfg:       input.cfg,
		plan:      input.plan,
		syncInput: input.syncInput,
		ref:       input.ref,
		result:    input.result,
	})
	if err != nil {
		return fmt.Errorf("apply changed plan: %w", err)
	}

	orch.logSummary(&summaryInput{
		Log:    orch.Logger,
		Cfg:    input.cfg,
		Plan:   input.plan,
		Result: input.result,
		PRURL:  input.result.PullRequestURL,
	})

	return nil
}

func (orch *Orchestrator) finishSync(ctx context.Context, input *finishSyncInput) (*Result, error) {
	if !input.plan.Changed {
		orch.logSummary(&summaryInput{
			Log:    orch.Logger,
			Cfg:    input.cfg,
			Plan:   input.plan,
			Result: input.result,
			PRURL:  consts.Empty,
		})

		return input.result, nil
	}

	err := orch.finishChangedPlan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("finish changed plan: %w", err)
	}

	return input.result, nil
}

func (orch *Orchestrator) gitStepDefs(ctx context.Context, args *gitPlanIn) []gitSyncStep {
	return append(
		[]gitSyncStep{orch.gitCheckoutStep(ctx, args.cfg)},
		orch.gitStepsAfter(ctx, args)...,
	)
}

func (orch *Orchestrator) gitCheckoutStep(ctx context.Context, cfg *config.Config) gitSyncStep {
	return gitSyncStep{
		fn:  func() error { return orch.GitBrancher.CreateOrResetBranch(ctx, cfg.BranchName) },
		msg: "checkout branch",
	}
}

func (orch *Orchestrator) gitStepsAfter(ctx context.Context, args *gitPlanIn) []gitSyncStep {
	return []gitSyncStep{
		{
			fn:  func() error { return orch.GitIndexer.Stage(ctx, args.plan.StagePaths) },
			msg: "stage paths",
		},
		{
			fn:  func() error { return orch.GitIndexer.Commit(ctx, gitports.SyncCommitMessage) },
			msg: "commit changes",
		},
		{
			fn:  func() error { return orch.GitPublisher.PushForceWithLease(ctx, args.cfg.BranchName) },
			msg: "push branch",
		},
	}
}

func (orch *Orchestrator) logDependencies(deps []string) {
	for i := range deps {
		orch.Logger.Printf("dependency: %s", deps[i])
	}
}

func (orch *Orchestrator) logDestinationNormalization(syncInput *syncIn) {
	orch.Logger.Group("Normalize destination names", func() {
		for source := range syncInput.SourceToDest {
			orch.Logger.Printf(fmtArrow, source, syncInput.SourceToDest[source])
		}
	})
}

func (orch *Orchestrator) logResolutions(resolved []resItem) {
	for i := range resolved {
		res := &resolved[i]
		orch.Logger.Printf(fmtArrow, res.LogicalTask, res.SourceModule)
	}
}

func (orch *Orchestrator) logValidateInputs(cfg *config.Config) {
	orch.Logger.Group("Validate inputs", func() {
		orch.Logger.Printf("Validated %d task(s)", len(cfg.Tasks))
		orch.Logger.Printf(fmtTargetFolder, cfg.TargetFolder)
	})
}

func (orch *Orchestrator) logSummary(in *summaryInput) {
	orch.Logger.Group(groupSummary, func() {
		printSummary(in)
	})
}

func (orch *Orchestrator) maybeCommitPush(ctx context.Context, args *gitPlanIn) error {
	if !gitports.IsGitRepo(args.cfg.Workspace) {
		return nil
	}

	err := runGroupNoResult(orch.Logger, "Create synchronization commit", func() error {
		runErr := orch.runGitSync(ctx, args)
		if runErr != nil {
			return fmt.Errorf("run git sync: %w", runErr)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("create synchronization commit: %w", err)
	}

	return nil
}

func (orch *Orchestrator) maybeCreateOrUpdatePR(ctx context.Context, input *prPhaseInput) error {
	if !gitports.IsGitRepo(input.cfg.Workspace) || orch.PRClient == nil {
		return nil
	}

	err := runGroupNoResult(orch.Logger, "Create or update pull request", func() error {
		runErr := orch.runPR(ctx, input)
		if runErr != nil {
			return fmt.Errorf("run PR: %w", runErr)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("create or update pull request: %w", err)
	}

	return nil
}

func (orch *Orchestrator) maybeDefBranch(ctx context.Context, args *gitPlanIn) (string, error) {
	if !gitports.IsGitRepo(args.cfg.Workspace) {
		return consts.Empty, nil
	}

	base, err := orch.runGitPre(ctx, args)
	if err != nil {
		return consts.Empty, fmt.Errorf("run git preconditions: %w", err)
	}

	return base, nil
}

func (orch *Orchestrator) planFinish(ctx context.Context, inp *planIn) (*Result, error) {
	planned, err := orch.planResult(&planResultArgs{cfg: inp.cfg, snap: inp.snapshot, ref: inp.ref})
	if err != nil {
		return nil, fmt.Errorf("plan and build result: %w", err)
	}

	result, err := orch.finishSync(ctx, &finishSyncInput{
		cfg:       inp.cfg,
		plan:      planned.plan,
		syncInput: planned.syncInput,
		ref:       inp.ref,
		result:    planned.result,
	})
	if err != nil {
		return nil, fmt.Errorf("finish sync: %w", err)
	}

	return result, nil
}

func (orch *Orchestrator) planResult(args *planResultArgs) (planResult, error) {
	resolutions, depSources, err := orch.modDeps(args.cfg, args.snap)
	if err != nil {
		return planResult{}, fmt.Errorf("resolve modules and deps: %w", err)
	}

	planned, err := orch.buildPlanResult(&buildPlanInput{
		cfg:         args.cfg,
		snapshot:    args.snap,
		resolutions: resolutions,
		depSources:  depSources,
	}, args.ref)
	if err != nil {
		return planResult{}, fmt.Errorf("build plan result: %w", err)
	}

	return planned, nil
}

func (orch *Orchestrator) resolveAllModules(cfg *config.Config, snap *snapInfo) ([]resItem, error) {
	resolved, err := resolvesvc.ResolveAll(&resolvesvc.ResolveAllInput{
		Tasks:          cfg.Tasks,
		Catalog:        snap.Catalog,
		PackageManager: cfg.NodePackageManager,
		VersionManager: cfg.NodeVersionManager,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve modules: %w", err)
	}

	orch.logResolutions(resolved)

	return resolved, nil
}

func (orch *Orchestrator) resolveDepSources(res []resItem, snap *snapInfo) ([]string, error) {
	var deps []string

	err := assignGrouped(orch.Logger, "Resolve dependencies", func() error {
		resolved, depErr := orch.resolveTransitiveDeps(res, snap)
		if depErr != nil {
			return fmt.Errorf(errFmtResolveTransitiveDeps, depErr)
		}

		deps = resolved

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf(errFmtResolveDependencies, err)
	}

	return deps, nil
}

func (orch *Orchestrator) resolveTransitiveDeps(res []resItem, snap *snapInfo) ([]string, error) {
	requestedSources := sourceModulesOf(res)

	resolved, err := resolvesvc.ResolveTransitive(requestedSources, snap.Deps)
	if err != nil {
		return nil, fmt.Errorf("resolve transitive dependencies: %w", err)
	}

	orch.logDependencies(resolved)

	return resolved, nil
}

//nolint:gocritic // single-line sig for whitespace
func (orch *Orchestrator) modDeps(cfg *config.Config, snap *snapInfo) ([]resItem, []string, error) {
	resolutions, err := orch.resolveReqMods(cfg, snap)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtResolveRequestedModules, err)
	}

	depSources, err := orch.resolveDepSources(resolutions, snap)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtResolveDependencies, err)
	}

	return resolutions, depSources, nil
}

//nolint:nestif // create vs update PR paths require branching on existing PR
func (orch *Orchestrator) resolveOrCreatePR(ctx context.Context, input *prResolveInput) error {
	if input.existing == nil {
		err := orch.createResolvedPR(ctx, input)
		if err != nil {
			return fmt.Errorf("create resolved PR: %w", err)
		}

		return nil
	}

	err := orch.updateResolvedPR(ctx, input)
	if err != nil {
		return fmt.Errorf("update resolved PR: %w", err)
	}

	return nil
}

func (orch *Orchestrator) createResolvedPR(ctx context.Context, input *prResolveInput) error {
	err := orch.createNewPR(ctx, &createPRInput{
		result:        input.phase.result,
		branch:        input.phase.cfg.BranchName,
		defaultBranch: input.phase.defaultBranch,
		body:          input.body,
	})
	if err != nil {
		return fmt.Errorf("create new PR: %w", err)
	}

	return nil
}

func (orch *Orchestrator) updateResolvedPR(ctx context.Context, input *prResolveInput) error {
	err := orch.updateExistingPR(ctx, &updatePRInput{
		existing: input.existing,
		body:     input.body,
		result:   input.phase.result,
	})
	if err != nil {
		return fmt.Errorf("update existing PR: %w", err)
	}

	return nil
}

func (orch *Orchestrator) resolveReqMods(cfg *config.Config, snap *snapInfo) ([]resItem, error) {
	var resolved []resolvesvc.Resolution

	err := assignGrouped(orch.Logger, "Resolve requested modules", func() error {
		modules, resolveErr := orch.resolveAllModules(cfg, snap)
		if resolveErr != nil {
			return fmt.Errorf("resolve all modules: %w", resolveErr)
		}

		resolved = modules

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf(errFmtResolveRequestedModules, err)
	}

	return resolved, nil
}

func (orch *Orchestrator) storeRef(ctx context.Context, cfg *config.Config) (refInfo, error) {
	var ref refInfo

	err := assignGrouped(orch.Logger, "Resolve source version", func() error {
		resolved, resolveErr := orch.storeRefGrp(ctx, cfg)
		if resolveErr != nil {
			return fmt.Errorf("resolve store ref in group: %w", resolveErr)
		}

		ref = resolved

		return nil
	})
	if err != nil {
		return refInfo{}, fmt.Errorf("resolve source version: %w", err)
	}

	return ref, nil
}

func (orch *Orchestrator) storeRefGrp(ctx context.Context, cfg *config.Config) (refInfo, error) {
	resolved, err := orch.StoreClient.ResolveRef(ctx, cfg.StoreVersion)
	if err != nil {
		return refInfo{}, fmt.Errorf(errFmtResolveStoreRef, err)
	}

	orch.Logger.Printf("Source ref: %s", resolved.SourceRef)
	orch.Logger.Printf("Resolved commit: %s", resolved.ResolvedCommit)

	return resolved, nil
}

func (orch *Orchestrator) runGitPre(ctx context.Context, args *gitPlanIn) (string, error) {
	err := orch.ensureGitReadyForSync(ctx, args.cfg)
	if err != nil {
		return consts.Empty, fmt.Errorf("ensure git ready: %w", err)
	}

	err = orch.checkUnrelatedChanges(ctx, args.plan)
	if err != nil {
		return consts.Empty, fmt.Errorf(errFmtCheckUnrelatedChanges, err)
	}

	base, err := orch.prBaseBranch(ctx, args.cfg)
	if err != nil {
		return consts.Empty, fmt.Errorf("determine PR base branch: %w", err)
	}

	return base, nil
}

func (orch *Orchestrator) runGitSync(ctx context.Context, args *gitPlanIn) error {
	err := runGitSyncSteps(orch.gitStepDefs(ctx, args))
	if err != nil {
		return fmt.Errorf("run git sync steps: %w", err)
	}

	return nil
}

func (orch *Orchestrator) runPR(ctx context.Context, input *prPhaseInput) error {
	body := prservice.BuildPRBody(input.cfg, input.plan, prservice.StoreRefFrom(input.ref))

	existing, err := orch.openPR(ctx, input.cfg.BranchName, input.defaultBranch)
	if err != nil {
		return fmt.Errorf(errFmtFindOpenPullRequest, err)
	}

	err = orch.resolveOrCreatePR(ctx, &prResolveInput{
		phase:    input,
		body:     body,
		existing: existing,
	})
	if err != nil {
		return fmt.Errorf("resolve or create PR: %w", err)
	}

	return nil
}

func (orch *Orchestrator) runPipeline(ctx context.Context, cfg *config.Config) (*Result, error) {
	ref, snapshot, err := orch.getStore(ctx, &fetchIn{cfg: cfg})
	if err != nil {
		return nil, fmt.Errorf("fetch store data: %w", err)
	}

	defer orch.closeSnapshotQuietly(snapshot)

	planned, err := orch.planFinish(ctx, &plannedSyncInput{
		cfg:      cfg,
		ref:      ref,
		snapshot: snapshot,
	})
	if err != nil {
		return nil, fmt.Errorf("plan and finish sync: %w", err)
	}

	return planned, nil
}

func (orch *Orchestrator) updateExistingPR(ctx context.Context, input *updatePRInput) error {
	err := orch.PRClient.UpdatePRBody(ctx, input.existing.Number, input.body)
	if err != nil {
		return fmt.Errorf("update pull request body: %w", err)
	}

	input.result.PullRequestNumber = strconv.Itoa(input.existing.Number)
	input.result.PullRequestURL = input.existing.URL
	orch.Logger.Printf("Updated pull request #%d", input.existing.Number)

	return nil
}

func (orch *Orchestrator) wireDefaults() {
	orch.ensureLogger()
}
