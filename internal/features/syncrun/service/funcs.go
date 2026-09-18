// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prservice "github.com/task-otter/Taskotter/internal/features/pr/service"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

func wireSyncHooks(deps *Deps) {
	if deps.PrepareSyncInput == nil {
		deps.PrepareSyncInput = syncsvc.PrepareSyncInput
	}

	if deps.BuildPlan == nil {
		deps.BuildPlan = syncsvc.BuildPlan
	}

	if deps.ApplyPlan == nil {
		deps.ApplyPlan = syncsvc.ApplyPlan
	}

	wireResolveHooks(deps)
}

func wireResolveHooks(deps *Deps) {
	if deps.ResolveAll == nil {
		deps.ResolveAll = resolvesvc.ResolveAll
	}

	if deps.ResolveTransitive == nil {
		deps.ResolveTransitive = resolvesvc.ResolveTransitive
	}
}

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

// NewOrchestrator builds an Orchestrator that runs the sync pipeline with deps.
func NewOrchestrator(deps *Deps) *Orchestrator {
	return &Orchestrator{run: func(ctx context.Context, cfg *config.Config) (*Result, error) {
		wireDefaults(deps)

		result, err := execPipeline(ctx, deps, cfg)
		if err != nil {
			return nil, fmt.Errorf(errFmtRun, err)
		}

		return result, nil
	}}
}

// Run executes the full sync pipeline.
func (orch *Orchestrator) Run(ctx context.Context, cfg *config.Config) (*Result, error) {
	if orch == nil || orch.run == nil {
		return nil, errOrchestratorNotConfigured
	}

	result, err := orch.run(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf(errFmtRun, err)
	}

	return result, nil
}

func applyChangedPlan(ctx context.Context, deps *Deps, input *changedPlanInput) error {
	defaultBranch, err := maybeDefBranch(ctx, deps, &gitPlanIn{cfg: input.cfg, plan: input.plan})
	if err != nil {
		return fmt.Errorf("resolve default branch: %w", err)
	}

	err = applySyncChanges(ctx, deps, &branchPlanIn{inp: input, defBranch: defaultBranch})
	if err != nil {
		return fmt.Errorf("apply sync changes: %w", err)
	}

	return nil
}

func applySyncChanges(ctx context.Context, deps *Deps, args *branchPlanIn) error {
	err := copyTaskModules(deps, args.inp.plan, args.inp.syncInput)
	if err != nil {
		return fmt.Errorf("copy task modules: %w", err)
	}

	err = commitAndMaybePR(ctx, deps, args)
	if err != nil {
		return fmt.Errorf("commit and maybe PR: %w", err)
	}

	return nil
}

func buildPlanResult(deps *Deps, inp *buildPlanInput, ref *refInfo) (planResult, error) {
	syncInput, plan, err := buildSyncPlan(deps, inp)
	if err != nil {
		return planResult{}, fmt.Errorf(errFmtBuildSyncPlan, err)
	}

	result := buildResult(inp.cfg, plan, ref)

	return planResult{syncInput: syncInput, plan: plan, result: result}, nil
}

func buildSyncPlan(deps *Deps, inp *buildPlanInput) (*syncIn, *syncPlan, error) {
	syncInput, err := deps.PrepareSyncInput(&syncsvc.PrepareSyncInputArgs{
		Cfg:         inp.cfg,
		Snapshot:    syncsvc.SnapshotPort(inp.snapshot),
		TaskfileOps: synctaskfile.NewOps(),
		Resolutions: inp.resolutions,
		DepSources:  inp.depSources,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("prepare sync input: %w", err)
	}

	logDestinationNormalization(deps, &syncInput)

	plan, err := compareManagedFiles(deps, &syncInput)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtBuildSyncPlan, err)
	}

	return &syncInput, plan, nil
}

func checkUnrelatedChanges(ctx context.Context, deps *Deps, plan *syncPlan) error {
	allowed := gitports.AllowedPathSet(plan.StagePaths)

	unrelated, err := deps.GitIndexer.HasUnrelatedChanges(ctx, allowed)
	if err != nil {
		return fmt.Errorf(errFmtCheckUnrelatedChanges, err)
	}

	if unrelated {
		return errUnrelatedChanges
	}

	return nil
}

func commitAndMaybePR(ctx context.Context, deps *Deps, args *branchPlanIn) error {
	err := maybeCommitPush(ctx, deps, &gitPlanIn{cfg: args.inp.cfg, plan: args.inp.plan})
	if err != nil {
		return fmt.Errorf("commit and push: %w", err)
	}

	err = runPRPhase(ctx, deps, args)
	if err != nil {
		return fmt.Errorf("run PR phase: %w", err)
	}

	return nil
}

func runPRPhase(ctx context.Context, deps *Deps, args *branchPlanIn) error {
	err := maybeCreateOrUpdatePR(ctx, deps, &prPhaseInput{
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

func closeSnapshotQuietly(deps *Deps, snapshot *storedomain.Snapshot) {
	closeErr := storedomain.Close(snapshot)
	if closeErr != nil {
		deps.Logger.Printf("close store snapshot: %v", closeErr)
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

func compareManagedFiles(deps *Deps, syncInput *syncIn) (*syncPlan, error) {
	var plan *syncdomain.Plan

	err := assignGrouped(deps.Logger, "Compare managed files", func() error {
		built, planErr := deps.BuildPlan(syncInput)
		if planErr != nil {
			return fmt.Errorf("build plan: %w", planErr)
		}

		logBuiltPlan(deps.Logger, built)

		plan = built

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("compare managed files: %w", err)
	}

	return plan, nil
}

func copyTaskModules(deps *Deps, plan *syncPlan, syncInput *syncIn) error {
	err := runGroupNoResult(deps.Logger, "Copy task modules", func() error {
		applyErr := deps.ApplyPlan(plan, syncInput)
		if applyErr != nil {
			return fmt.Errorf("apply plan: %w", applyErr)
		}

		deps.Logger.Printf("Copied modules and validated generated YAML")

		return nil
	})
	if err != nil {
		return fmt.Errorf("apply sync plan: %w", err)
	}

	return nil
}

func createNewPR(ctx context.Context, deps *Deps, input *createPRInput) error {
	pullReq, err := deps.PRClient.CreatePR(ctx, &prdomain.CreatePRRequest{
		Branch: input.branch,
		Base:   input.defaultBranch,
		Body:   input.body,
	})
	if err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}

	input.result.PullRequestNumber = strconv.Itoa(pullReq.Number)
	input.result.PullRequestURL = pullReq.URL
	deps.Logger.Printf("Created pull request #%d", pullReq.Number)

	return nil
}

func prBaseBranch(ctx context.Context, deps *Deps, cfg *config.Config) (string, error) {
	if cfg.BaseBranch != consts.Empty {
		return cfg.BaseBranch, nil
	}

	defaultBranch, err := deps.GitBrancher.DefaultBranch(ctx)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve pull request base branch: %w", err)
	}

	return defaultBranch, nil
}

func groupedSnap(ctx context.Context, deps *Deps, ref *refInfo) (*snapInfo, error) {
	snap, downloadErr := deps.StoreClient.DownloadSnapshot(ctx, ref)
	if downloadErr != nil {
		return nil, fmt.Errorf("download snapshot: %w", downloadErr)
	}

	deps.Logger.Printf("Loaded store snapshot from %s", ref.ResolvedCommit)

	return snap, nil
}

func downloadSnap(ctx context.Context, deps *Deps, ref *refInfo) (*snapInfo, error) {
	var snapshot *storedomain.Snapshot

	err := assignGrouped(deps.Logger, "Download store", func() error {
		snap, downloadErr := groupedSnap(ctx, deps, ref)
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

func ensureGitReadyForSync(ctx context.Context, deps *Deps, cfg *config.Config) error {
	err := prepareGitWorkspace(ctx, deps, cfg)
	if err != nil {
		return fmt.Errorf("prepare git workspace: %w", err)
	}

	gitports.WriteLocalIdentity()

	err = gitports.EnsureBranchOwned(ctx, deps.GitBrancher, cfg.BranchName)
	if err != nil {
		return fmt.Errorf("ensure branch owned: %w", err)
	}

	return nil
}

func prepareGitWorkspace(ctx context.Context, deps *Deps, cfg *config.Config) error {
	if deps.GitClient == nil {
		return nil
	}

	deps.GitClient.EnsureSafeDirectory()

	err := deps.GitClient.ConfigureCredentials(ctx, cfg.GitHubToken, cfg.Repository)
	if err != nil {
		return fmt.Errorf("configure credentials: %w", err)
	}

	return nil
}

func ensureLogger(deps *Deps) {
	if deps.Logger == nil {
		deps.Logger = logging.New()
	}
}

func execPipeline(ctx context.Context, deps *Deps, cfg *config.Config) (*Result, error) {
	logValidateInputs(deps, cfg)

	result, err := runPipeline(ctx, deps, cfg)
	if err != nil {
		return nil, fmt.Errorf("run pipeline: %w", err)
	}

	return result, nil
}

func getStore(ctx context.Context, deps *Deps, in *fetchIn) (*refInfo, *snapInfo, error) {
	ref, err := storeRef(ctx, deps, in.cfg)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtResolveStoreRef, err)
	}

	snapshot, err := downloadSnap(ctx, deps, &ref)
	if err != nil {
		return nil, nil, fmt.Errorf("download store snapshot: %w", err)
	}

	deps.Logger.Group("Load module catalog", func() {
		deps.Logger.Printf("Catalog modules: %d", len(snapshot.Catalog))
	})

	return &ref, snapshot, nil
}

func openPR(ctx context.Context, deps *Deps, branches *branchPair) (*pullReq, error) {
	existing, err := deps.PRClient.FindOpenPR(ctx, branches.name, branches.defaultBranch)

	if err != nil && !errors.Is(err, prdomain.ErrPullRequestNotFound) {
		return nil, fmt.Errorf(errFmtFindOpenPullRequest, err)
	}

	return existing, nil
}

func finishChangedPlan(ctx context.Context, deps *Deps, input *finishSyncInput) error {
	err := applyChangedPlan(ctx, deps, &changedPlanInput{
		cfg:       input.cfg,
		plan:      input.plan,
		syncInput: input.syncInput,
		ref:       input.ref,
		result:    input.result,
	})
	if err != nil {
		return fmt.Errorf("apply changed plan: %w", err)
	}

	logSummary(deps, &summaryInput{
		Log:    deps.Logger,
		Cfg:    input.cfg,
		Plan:   input.plan,
		Result: input.result,
		PRURL:  input.result.PullRequestURL,
	})

	return nil
}

func finishSync(ctx context.Context, deps *Deps, input *finishSyncInput) (*Result, error) {
	if !input.plan.Changed {
		logSummary(deps, &summaryInput{
			Log:    deps.Logger,
			Cfg:    input.cfg,
			Plan:   input.plan,
			Result: input.result,
			PRURL:  consts.Empty,
		})

		return input.result, nil
	}

	err := finishChangedPlan(ctx, deps, input)
	if err != nil {
		return nil, fmt.Errorf("finish changed plan: %w", err)
	}

	return input.result, nil
}

func gitStepDefs(ctx context.Context, deps *Deps, args *gitPlanIn) []gitSyncStep {
	return append(
		[]gitSyncStep{gitCheckoutStep(ctx, deps, args.cfg)},
		gitStepsAfter(ctx, deps, args)...,
	)
}

func gitCheckoutStep(ctx context.Context, deps *Deps, cfg *config.Config) gitSyncStep {
	return gitSyncStep{
		fn:  func() error { return deps.GitBrancher.CreateOrResetBranch(ctx, cfg.BranchName) },
		msg: "checkout branch",
	}
}

func gitStepsAfter(ctx context.Context, deps *Deps, args *gitPlanIn) []gitSyncStep {
	return []gitSyncStep{
		{
			fn:  func() error { return deps.GitIndexer.Stage(ctx, args.plan.StagePaths) },
			msg: "stage paths",
		},
		{
			fn:  func() error { return deps.GitIndexer.Commit(ctx, gitports.SyncCommitMessage) },
			msg: "commit changes",
		},
		{
			fn:  func() error { return deps.GitPublisher.PushForceWithLease(ctx, args.cfg.BranchName) },
			msg: "push branch",
		},
	}
}

func logDependencies(deps *Deps, modules []string) {
	for i := range modules {
		deps.Logger.Printf("dependency: %s", modules[i])
	}
}

func logDestinationNormalization(deps *Deps, syncInput *syncIn) {
	deps.Logger.Group("Normalize destination names", func() {
		for source := range syncInput.SourceToDest {
			deps.Logger.Printf(fmtArrow, source, syncInput.SourceToDest[source])
		}
	})
}

func logResolutions(deps *Deps, resolved []resItem) {
	for i := range resolved {
		res := &resolved[i]
		deps.Logger.Printf(fmtArrow, res.LogicalTask, res.SourceModule)
	}
}

func logValidateInputs(deps *Deps, cfg *config.Config) {
	deps.Logger.Group("Validate inputs", func() {
		deps.Logger.Printf("Validated %d task(s)", len(cfg.Tasks))
		deps.Logger.Printf(fmtTargetFolder, cfg.TargetFolder)
	})
}

func logSummary(deps *Deps, in *summaryInput) {
	deps.Logger.Group(groupSummary, func() {
		printSummary(in)
	})
}

func maybeCommitPush(ctx context.Context, deps *Deps, args *gitPlanIn) error {
	if !gitports.IsGitRepo(args.cfg.Workspace) {
		return nil
	}

	err := runGroupNoResult(deps.Logger, "Create synchronization commit", func() error {
		runErr := runGitSync(ctx, deps, args)
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

func maybeCreateOrUpdatePR(ctx context.Context, deps *Deps, input *prPhaseInput) error {
	if !gitports.IsGitRepo(input.cfg.Workspace) || deps.PRClient == nil {
		return nil
	}

	err := runGroupNoResult(deps.Logger, "Create or update pull request", func() error {
		runErr := runPR(ctx, deps, input)
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

func maybeDefBranch(ctx context.Context, deps *Deps, args *gitPlanIn) (string, error) {
	if !gitports.IsGitRepo(args.cfg.Workspace) {
		return consts.Empty, nil
	}

	base, err := runGitPre(ctx, deps, args)
	if err != nil {
		return consts.Empty, fmt.Errorf("run git preconditions: %w", err)
	}

	return base, nil
}

func planFinish(ctx context.Context, deps *Deps, inp *planIn) (*Result, error) {
	planned, err := computePlanResult(
		deps,
		&planResultArgs{cfg: inp.cfg, snap: inp.snapshot, ref: inp.ref},
	)
	if err != nil {
		return nil, fmt.Errorf("plan and build result: %w", err)
	}

	result, err := finishSync(ctx, deps, &finishSyncInput{
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

func computePlanResult(deps *Deps, args *planResultArgs) (planResult, error) {
	resolutions, depSources, err := modDeps(deps, args.cfg, args.snap)
	if err != nil {
		return planResult{}, fmt.Errorf("resolve modules and deps: %w", err)
	}

	planned, err := buildPlanResult(deps, &buildPlanInput{
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

func resolveAllModules(deps *Deps, cfg *config.Config, snap *snapInfo) ([]resItem, error) {
	resolved, err := deps.ResolveAll(&resolvesvc.ResolveAllInput{
		Tasks:          cfg.Tasks,
		Catalog:        snap.Catalog,
		PackageManager: cfg.NodePackageManager,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve modules: %w", err)
	}

	logResolutions(deps, resolved)

	return resolved, nil
}

func resolveDepSources(deps *Deps, res []resItem, snap *snapInfo) ([]string, error) {
	var depSources []string

	err := assignGrouped(deps.Logger, "Resolve dependencies", func() error {
		resolved, depErr := resolveTransitiveDeps(deps, res, snap)
		if depErr != nil {
			return fmt.Errorf(errFmtResolveTransitiveDeps, depErr)
		}

		depSources = resolved

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf(errFmtResolveDependencies, err)
	}

	return depSources, nil
}

func resolveTransitiveDeps(deps *Deps, res []resItem, snap *snapInfo) ([]string, error) {
	requestedSources := sourceModulesOf(res)

	resolved, err := deps.ResolveTransitive(requestedSources, snap.Deps)
	if err != nil {
		return nil, fmt.Errorf("resolve transitive dependencies: %w", err)
	}

	logDependencies(deps, resolved)

	return resolved, nil
}

func modDeps(deps *Deps, cfg *config.Config, snap *snapInfo) ([]resItem, []string, error) {
	resolutions, err := resolveReqMods(deps, cfg, snap)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtResolveRequestedModules, err)
	}

	depSources, err := resolveDepSources(deps, resolutions, snap)
	if err != nil {
		return nil, nil, fmt.Errorf(errFmtResolveDependencies, err)
	}

	return resolutions, depSources, nil
}

func resolveOrCreatePR(ctx context.Context, deps *Deps, input *prResolveInput) error {
	if input.existing == nil {
		err := createResolvedPR(ctx, deps, input)
		if err != nil {
			return fmt.Errorf("create resolved PR: %w", err)
		}

		return nil
	}

	err := updateResolvedPR(ctx, deps, input)
	if err != nil {
		return fmt.Errorf("update resolved PR: %w", err)
	}

	return nil
}

func createResolvedPR(ctx context.Context, deps *Deps, input *prResolveInput) error {
	err := createNewPR(ctx, deps, &createPRInput{
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

func updateResolvedPR(ctx context.Context, deps *Deps, input *prResolveInput) error {
	err := updateExistingPR(ctx, deps, &updatePRInput{
		existing: input.existing,
		body:     input.body,
		result:   input.phase.result,
	})
	if err != nil {
		return fmt.Errorf("update existing PR: %w", err)
	}

	return nil
}

func resolveReqMods(deps *Deps, cfg *config.Config, snap *snapInfo) ([]resItem, error) {
	var resolved []resolvesvc.Resolution

	err := assignGrouped(deps.Logger, "Resolve requested modules", func() error {
		modules, resolveErr := resolveAllModules(deps, cfg, snap)
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

func storeRef(ctx context.Context, deps *Deps, cfg *config.Config) (refInfo, error) {
	var ref refInfo

	err := assignGrouped(deps.Logger, "Resolve source version", func() error {
		resolved, resolveErr := storeRefGrp(ctx, deps, cfg)
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

func storeRefGrp(ctx context.Context, deps *Deps, cfg *config.Config) (refInfo, error) {
	resolved, err := deps.StoreClient.ResolveRef(ctx, cfg.StoreVersion)
	if err != nil {
		return refInfo{}, fmt.Errorf(errFmtResolveStoreRef, err)
	}

	deps.Logger.Printf("Source ref: %s", resolved.SourceRef)
	deps.Logger.Printf("Resolved commit: %s", resolved.ResolvedCommit)

	return resolved, nil
}

func runGitPre(ctx context.Context, deps *Deps, args *gitPlanIn) (string, error) {
	err := ensureGitReadyForSync(ctx, deps, args.cfg)
	if err != nil {
		return consts.Empty, fmt.Errorf("ensure git ready: %w", err)
	}

	err = checkUnrelatedChanges(ctx, deps, args.plan)
	if err != nil {
		return consts.Empty, fmt.Errorf(errFmtCheckUnrelatedChanges, err)
	}

	base, err := prBaseBranch(ctx, deps, args.cfg)
	if err != nil {
		return consts.Empty, fmt.Errorf("determine PR base branch: %w", err)
	}

	return base, nil
}

func runGitSync(ctx context.Context, deps *Deps, args *gitPlanIn) error {
	err := runGitSyncSteps(gitStepDefs(ctx, deps, args))
	if err != nil {
		return fmt.Errorf("run git sync steps: %w", err)
	}

	return nil
}

func runPR(ctx context.Context, deps *Deps, input *prPhaseInput) error {
	body := prservice.BuildPRBody(input.cfg, input.plan, prservice.StoreRefFrom(input.ref))

	existing, err := openPR(ctx, deps, &branchPair{
		name:          input.cfg.BranchName,
		defaultBranch: input.defaultBranch,
	})
	if err != nil {
		return fmt.Errorf(errFmtFindOpenPullRequest, err)
	}

	err = resolveOrCreatePR(ctx, deps, &prResolveInput{
		phase:    input,
		body:     body,
		existing: existing,
	})
	if err != nil {
		return fmt.Errorf("resolve or create PR: %w", err)
	}

	return nil
}

func runPipeline(ctx context.Context, deps *Deps, cfg *config.Config) (*Result, error) {
	ref, snapshot, err := getStore(ctx, deps, &fetchIn{cfg: cfg})
	if err != nil {
		return nil, fmt.Errorf("fetch store data: %w", err)
	}

	defer closeSnapshotQuietly(deps, snapshot)

	planned, err := planFinish(ctx, deps, &plannedSyncInput{
		cfg:      cfg,
		ref:      ref,
		snapshot: snapshot,
	})
	if err != nil {
		return nil, fmt.Errorf("plan and finish sync: %w", err)
	}

	return planned, nil
}

func updateExistingPR(ctx context.Context, deps *Deps, input *updatePRInput) error {
	err := deps.PRClient.UpdatePRBody(ctx, input.existing.Number, input.body)
	if err != nil {
		return fmt.Errorf("update pull request body: %w", err)
	}

	input.result.PullRequestNumber = strconv.Itoa(input.existing.Number)
	input.result.PullRequestURL = input.existing.URL
	deps.Logger.Printf("Updated pull request #%d", input.existing.Number)

	return nil
}

func wireDefaults(deps *Deps) {
	ensureLogger(deps)
	wireSyncHooks(deps)
}

// ReportSyncRequired writes GitHub Actions annotations when a sync pull request must be merged.
func ReportSyncRequired(result *Result) {
	ReportSyncRequiredTo(os.Stderr, result)
}

// ReportSyncRequiredTo writes sync-required GitHub Actions annotations to writer.
func ReportSyncRequiredTo(writer io.Writer, result *Result) {
	writeSyncRequiredAnnotations(writer, syncRequiredSummary(result))
}

// ReportSyncUpToDate writes GitHub Actions notices when managed files already match the store.
func ReportSyncUpToDate(result *Result) {
	iox.FprintBestEffort(os.Stdout, syncUpToDateNotice)
	iox.FprintfBestEffortf(os.Stdout, "Store source SHA: %s\n", result.SourceSHA)
}

// SyncRequired reports whether the sync run changed managed files.
func SyncRequired(result *Result) bool {
	return result.Changed
}

// WriteActionOutputs writes sync result fields to GitHub Actions output or stdout.
func WriteActionOutputs(cfg *config.Config, result *Result) error {
	values := buildOutputValues(result)

	if cfg.GitHubOutput == consts.Empty {
		printOutputsToStdout(values)

		return nil
	}

	err := iox.WriteGitHubOutputs(cfg.GitHubOutput, values)
	if err != nil {
		return fmt.Errorf("write GitHub Actions outputs: %w", err)
	}

	return nil
}

func buildOutputValues(result *Result) map[string]string {
	return map[string]string{
		"changed":               strconv.FormatBool(result.Changed),
		"store-version":         result.StoreVersion,
		"source-ref":            result.SourceRef,
		"source-sha":            result.SourceSHA,
		"target-folder":         result.TargetFolder,
		"resolved-tasks":        result.ResolvedTasksJSON,
		"resolved-dependencies": result.ResolvedDependencies,
		"pull-request-number":   result.PullRequestNumber,
		"pull-request-url":      result.PullRequestURL,
	}
}

func buildResolvedDependenciesJSON(deps []lockmodel.ModuleRecord) string {
	out := make([]ResolvedTask, consts.IndexZero, len(deps))

	for i := range deps {
		dep := &deps[i]

		out = append(out, ResolvedTask{
			SourceModule:      dep.SourceModule,
			DestinationModule: dep.DestinationModule,
			Path:              dep.Path,
		})
	}

	data, err := json.MarshalIndent(out, consts.Empty, jsonIndent)
	iox.Discard(err)

	return string(data)
}

func buildResolvedTasksJSON(requested map[string]lockmodel.ModuleRecord) string {
	out := make(map[string]ResolvedTask, len(requested))

	for task := range requested {
		rec := requested[task]

		out[task] = ResolvedTask{
			SourceModule:      rec.SourceModule,
			DestinationModule: rec.DestinationModule,
			Path:              rec.Path,
		}
	}

	data, err := json.MarshalIndent(out, consts.Empty, jsonIndent)
	iox.Discard(err)

	return string(data)
}

func buildResult(cfg *config.Config, plan *syncdomain.Plan, ref *storedomain.RefInfo) *Result {
	result := newResultShell(cfg, plan, ref)
	fillResolvedJSON(result, plan)

	return result
}

func empty(v string) string {
	if v == consts.Empty {
		return "(latest default branch)"
	}

	return v
}

func fillResolvedJSON(result *Result, plan *syncdomain.Plan) {
	result.ResolvedTasksJSON = buildResolvedTasksJSON(plan.Requested)
	result.ResolvedDependencies = buildResolvedDependenciesJSON(plan.Dependencies)
}

func logDependencyModules(log *logging.Logger, plan *syncdomain.Plan) {
	for i := range plan.Dependencies {
		dep := &plan.Dependencies[i]
		log.Printf("Dependency %s -> %s", dep.SourceModule, dep.Path)
	}
}

func logFileCounts(log *logging.Logger, plan *syncdomain.Plan) {
	log.Printf("Files added: %d", len(plan.Added))
	log.Printf("Files updated: %d", len(plan.Updated))
	log.Printf("Files removed: %d", len(plan.Removed))
}

func logPullRequestOutcome(log *logging.Logger, prURL string) {
	if prURL != consts.Empty {
		log.Printf("Pull request: %s", prURL)

		return
	}

	log.Print("Pull request result: none")
}

func logRequestedTaskModules(log *logging.Logger, cfg *config.Config, plan *syncdomain.Plan) {
	log.Printf("Requested tasks: %v", cfg.Tasks)

	tasks := cfg.Tasks

	for i := range tasks {
		task := tasks[i]
		rec := plan.Requested[task]
		log.Printf("Source module %s -> %s", rec.SourceModule, rec.Path)
	}
}

func logResultMetadata(log *logging.Logger, result *Result) {
	log.Printf("Store version: %s", empty(result.StoreVersion))
	log.Printf("Source SHA: %s", result.SourceSHA)
	log.Printf(fmtTargetFolder, result.TargetFolder)
}

func newResultShell(cfg *config.Config, plan *syncdomain.Plan, ref *storedomain.RefInfo) *Result {
	return &Result{
		Changed:              plan.Changed,
		StoreVersion:         cfg.StoreVersion,
		SourceRef:            ref.SourceRef,
		SourceSHA:            ref.ResolvedCommit,
		TargetFolder:         cfg.TargetFolder,
		ResolvedTasksJSON:    consts.Empty,
		ResolvedDependencies: consts.Empty,
		PullRequestNumber:    consts.Empty,
		PullRequestURL:       consts.Empty,
		Plan:                 plan,
		Ref:                  *ref,
	}
}

func printOutputsToStdout(values map[string]string) {
	keys := make([]string, consts.IndexZero, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for i := range keys {
		key := keys[i]

		iox.FprintfBestEffortf(
			os.Stdout,
			"%s=%s\n",
			key,
			values[key],
		)
	}
}

func printSummary(in *summaryInput) {
	logRequestedTaskModules(in.Log, in.Cfg, in.Plan)
	logDependencyModules(in.Log, in.Plan)
	logResultMetadata(in.Log, in.Result)
	logFileCounts(in.Log, in.Plan)
	logPullRequestOutcome(in.Log, in.PRURL)
}

func syncRequiredSummary(result *Result) string {
	if result.PullRequestURL == consts.Empty {
		return "TaskOtter synced taskfile changes but did not return a pull request URL."
	}

	prNumber := result.PullRequestNumber

	if prNumber == consts.Empty {
		prNumber = "unknown"
	}

	return fmt.Sprintf("TaskOtter opened sync PR #%s: %s", prNumber, result.PullRequestURL)
}

func writeSyncRequiredAnnotations(writer io.Writer, summary string) {
	iox.FprintfBestEffortf(
		writer,
		"::error title=TaskOtter sync required::%s"+syncRequiredErrorSuffix,
		summary,
	)
	iox.FprintBestEffort(writer, syncRequiredNotice)
}

// MarshalJSON encodes a resolved task using the GitHub Actions output keys.
func (task *ResolvedTask) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(map[string]string{
		"source_module":      task.SourceModule,
		"destination_module": task.DestinationModule,
		"path":               task.Path,
	})

	iox.Discard(err)

	return data, nil
}
