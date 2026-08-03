// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package app orchestrates the sync, apply, and pull request workflow.
package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/dependency"
	"github.com/task-otter/Taskotter/internal/git"
	"github.com/task-otter/Taskotter/internal/github"
	"github.com/task-otter/Taskotter/internal/logging"
	"github.com/task-otter/Taskotter/internal/resolver"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
)

type (

	// StoreClient resolves store refs and downloads snapshots for the sync pipeline.
	StoreClient interface {
		ResolveRef(ctx context.Context, requestedVersion string) (store.RefInfo, error)
		DownloadSnapshot(ctx context.Context, ref *store.RefInfo) (*store.Snapshot, error)
	}

	// Orchestrator coordinates store, git, and GitHub operations for a sync run.
	Orchestrator struct {
		Logger      *logging.Logger
		StoreClient StoreClient
		GitOps      git.Operations
		PRClient    github.PRClient
	}
)

const (
	fmtArrow          = "%s -> %s"
	fmtTargetFolder   = "Target folder: %s"
	groupSummary      = "Summary"
	errCreatePRClient = "create GitHub PR client: %w"
)

var errUnrelatedChanges = errors.New("unrelated uncommitted changes detected in workspace")

// Run creates an orchestrator from configuration and executes the sync pipeline.
func Run(ctx context.Context, cfg *config.Config) (*Result, error) {
	o, err := NewOrchestrator(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("new orchestrator: %w", err)
	}

	result, err := o.Run(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("run orchestrator: %w", err)
	}

	return result, nil
}

// NewOrchestrator builds an Orchestrator with default clients wired from configuration.
func NewOrchestrator(ctx context.Context, cfg *config.Config) (*Orchestrator, error) {
	orch := &Orchestrator{
		Logger:      logging.New(),
		StoreClient: store.NewClient(ctx, cfg.GitHubToken),
		GitOps:      git.NewClient(cfg.Workspace),
		PRClient:    nil,
	}

	if cfg.Repository != consts.Empty {
		prClient, err := github.NewClient(ctx, cfg.GitHubToken, cfg.Repository)
		if err != nil {
			return nil, fmt.Errorf(errCreatePRClient, err)
		}

		orch.PRClient = prClient
	}

	return orch, nil
}

// Run executes the full sync pipeline.
func (o *Orchestrator) Run(ctx context.Context, cfg *config.Config) (*Result, error) {
	err := o.wireDefaults(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("wire defaults: %w", err)
	}

	result, err := o.run(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("run: %w", err)
	}

	return result, nil
}

func runGroup[T any](logger *logging.Logger, title string, groupFn func() (T, error)) (T, error) {
	var (
		out T
		err error
	)

	logger.Group(title, func() {
		out, err = groupFn()
	})

	if err != nil {
		return out, fmt.Errorf("%s: %w", title, err)
	}

	return out, nil
}

func runGroupNoResult(logger *logging.Logger, title string, groupFn func() error) error {
	var err error

	logger.Group(title, func() {
		err = groupFn()
	})

	if err != nil {
		return fmt.Errorf("%s: %w", title, err)
	}

	return nil
}

func (o *Orchestrator) wireDefaults(ctx context.Context, cfg *config.Config) error {
	o.ensureLogger()
	o.ensureStoreClient(ctx, cfg)
	o.ensureGitOps(cfg)

	err := o.ensurePRClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("ensure PR client: %w", err)
	}

	return nil
}

func (o *Orchestrator) ensureLogger() {
	if o.Logger == nil {
		o.Logger = logging.New()
	}
}

func (o *Orchestrator) ensureStoreClient(ctx context.Context, cfg *config.Config) {
	if o.StoreClient == nil {
		o.StoreClient = store.NewClient(ctx, cfg.GitHubToken)
	}
}

func (o *Orchestrator) ensureGitOps(cfg *config.Config) {
	if o.GitOps == nil {
		o.GitOps = git.NewClient(cfg.Workspace)
	}
}

func (o *Orchestrator) ensurePRClient(ctx context.Context, cfg *config.Config) error {
	if o.PRClient != nil || cfg.Repository == consts.Empty {
		return nil
	}

	prClient, err := github.NewClient(ctx, cfg.GitHubToken, cfg.Repository)
	if err != nil {
		return fmt.Errorf(errCreatePRClient, err)
	}

	o.PRClient = prClient

	return nil
}

func (o *Orchestrator) runGitPreconditions(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
) (string, error) {
	err := o.ensureGitReadyForSync(ctx, cfg)
	if err != nil {
		return consts.Empty, fmt.Errorf("ensure git ready: %w", err)
	}

	err = o.checkUnrelatedChanges(ctx, plan)
	if err != nil {
		return consts.Empty, fmt.Errorf("check unrelated changes: %w", err)
	}

	base, err := o.determinePRBaseBranch(ctx, cfg)
	if err != nil {
		return consts.Empty, fmt.Errorf("determine PR base branch: %w", err)
	}

	return base, nil
}

func (o *Orchestrator) ensureGitReadyForSync(ctx context.Context, cfg *config.Config) error {
	err := o.GitOps.EnsureSafeDirectory(ctx)
	if err != nil {
		return fmt.Errorf("ensure safe directory: %w", err)
	}

	err = git.WriteLocalIdentity(ctx, o.GitOps)
	if err != nil {
		return fmt.Errorf("write local git identity: %w", err)
	}

	err = git.EnsureBranchOwned(ctx, o.GitOps, cfg.BranchName)
	if err != nil {
		return fmt.Errorf("ensure branch owned: %w", err)
	}

	return nil
}

func (o *Orchestrator) checkUnrelatedChanges(ctx context.Context, plan *syncer.Plan) error {
	allowed := git.AllowedPathSet(plan.StagePaths)

	unrelated, err := o.GitOps.HasUnrelatedChanges(ctx, allowed)
	if err != nil {
		return fmt.Errorf("check unrelated changes: %w", err)
	}

	if unrelated {
		return errUnrelatedChanges
	}

	return nil
}

func (o *Orchestrator) determinePRBaseBranch(
	ctx context.Context,
	cfg *config.Config,
) (string, error) {
	if cfg.BaseBranch != consts.Empty {
		return cfg.BaseBranch, nil
	}

	defaultBranch, err := o.GitOps.DefaultBranch(ctx)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve pull request base branch: %w", err)
	}

	return defaultBranch, nil
}

func (o *Orchestrator) runGitSync(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
) error {
	steps := []struct {
		fn  func() error
		msg string
	}{
		{
			fn:  func() error { return o.GitOps.CheckoutBranch(ctx, cfg.BranchName, true) },
			msg: "checkout branch",
		},
		{fn: func() error { return o.GitOps.Stage(ctx, plan.StagePaths) }, msg: "stage paths"},
		{
			fn:  func() error { return o.GitOps.Commit(ctx, git.SyncCommitMessage) },
			msg: "commit changes",
		},
		{fn: func() error { return o.GitOps.Push(ctx, cfg.BranchName, true) }, msg: "push branch"},
	}

	for i := range steps {
		step := &steps[i]

		err := step.fn()
		if err != nil {
			return fmt.Errorf("%s: %w", step.msg, err)
		}
	}

	return nil
}

func (o *Orchestrator) runPR(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
	ref *store.RefInfo,
	defaultBranch string,
	result *Result,
) error {
	body := github.BuildPRBody(cfg, plan, github.StoreRefFrom(ref))

	existing, err := o.PRClient.FindOpenPR(ctx, cfg.BranchName, defaultBranch)

	if err != nil && !errors.Is(err, github.ErrPullRequestNotFound) {
		return fmt.Errorf("find open pull request: %w", err)
	}

	if existing != nil {
		err := o.updateExistingPR(ctx, existing, body, result)
		if err != nil {
			return fmt.Errorf("update existing PR: %w", err)
		}

		return nil
	}

	err = o.createNewPR(ctx, cfg.BranchName, defaultBranch, body, result)
	if err != nil {
		return fmt.Errorf("create new PR: %w", err)
	}

	return nil
}

func (o *Orchestrator) updateExistingPR(
	ctx context.Context,
	existing *github.PullRequest,
	body string,
	result *Result,
) error {
	err := o.PRClient.UpdatePRBody(ctx, existing.Number, body)
	if err != nil {
		return fmt.Errorf("update pull request body: %w", err)
	}

	result.PullRequestNumber = strconv.Itoa(existing.Number)
	result.PullRequestURL = existing.URL
	o.Logger.Printf("Updated pull request #%d", existing.Number)

	return nil
}

func (o *Orchestrator) createNewPR(
	ctx context.Context,
	branch, defaultBranch, body string,
	result *Result,
) error {
	pullReq, err := o.PRClient.CreatePR(ctx, branch, defaultBranch, body)
	if err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}

	result.PullRequestNumber = strconv.Itoa(pullReq.Number)
	result.PullRequestURL = pullReq.URL
	o.Logger.Printf("Created pull request #%d", pullReq.Number)

	return nil
}

func (o *Orchestrator) resolveStoreRef(
	ctx context.Context,
	cfg *config.Config,
) (store.RefInfo, error) {
	ref, err := runGroup(o.Logger, "Resolve source version", func() (store.RefInfo, error) {
		resolved, resolveErr := o.StoreClient.ResolveRef(ctx, cfg.StoreVersion)
		if resolveErr != nil {
			return store.RefInfo{}, fmt.Errorf("resolve store ref: %w", resolveErr)
		}

		o.Logger.Printf("Source ref: %s", resolved.SourceRef)
		o.Logger.Printf("Resolved commit: %s", resolved.ResolvedCommit)

		return resolved, nil
	})
	if err != nil {
		return store.RefInfo{}, fmt.Errorf("resolve source version: %w", err)
	}

	return ref, nil
}

func (o *Orchestrator) downloadStoreSnapshot(
	ctx context.Context,
	ref *store.RefInfo,
) (*store.Snapshot, error) {
	snapshot, err := runGroup(o.Logger, "Download store", func() (*store.Snapshot, error) {
		snap, downloadErr := o.StoreClient.DownloadSnapshot(ctx, ref)
		if downloadErr != nil {
			return nil, fmt.Errorf("download snapshot: %w", downloadErr)
		}

		o.Logger.Printf("Loaded store snapshot from %s", ref.ResolvedCommit)

		return snap, nil
	})
	if err != nil {
		return nil, fmt.Errorf("download store: %w", err)
	}

	return snapshot, nil
}

func (o *Orchestrator) resolveModulesAndDeps(
	cfg *config.Config,
	snapshot *store.Snapshot,
) ([]resolver.Resolution, []string, error) {
	resolutions, err := o.resolveRequestedModules(cfg, snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve requested modules: %w", err)
	}

	depSources, err := o.resolveDependencySources(resolutions, snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve dependencies: %w", err)
	}

	return resolutions, depSources, nil
}

func (o *Orchestrator) resolveRequestedModules(
	cfg *config.Config,
	snapshot *store.Snapshot,
) ([]resolver.Resolution, error) {
	//nolint:wrapcheck // runGroup wraps the error with the group title
	return runGroup(
		o.Logger,
		"Resolve requested modules",
		func() ([]resolver.Resolution, error) {
			resolved, resolveErr := resolver.ResolveAll(
				cfg.Tasks,
				snapshot.Catalog,
				cfg.NodePackageManager,
				cfg.NodeVersionManager,
			)
			if resolveErr != nil {
				return nil, fmt.Errorf("resolve modules: %w", resolveErr)
			}

			for i := range resolved {
				res := &resolved[i]
				o.Logger.Printf(fmtArrow, res.LogicalTask, res.SourceModule)
			}

			return resolved, nil
		},
	)
}

func (o *Orchestrator) resolveDependencySources(
	resolutions []resolver.Resolution,
	snapshot *store.Snapshot,
) ([]string, error) {
	//nolint:wrapcheck // runGroup wraps the error with the group title
	return runGroup(o.Logger, "Resolve dependencies", func() ([]string, error) {
		requestedSources := sourceModulesOf(resolutions)

		deps, depErr := dependency.ResolveTransitive(requestedSources, snapshot.Deps)
		if depErr != nil {
			return nil, fmt.Errorf("resolve transitive dependencies: %w", depErr)
		}

		o.logDependencies(deps)

		return deps, nil
	})
}

func sourceModulesOf(resolutions []resolver.Resolution) []string {
	requestedSources := make([]string, 0, len(resolutions))

	for i := range resolutions {
		requestedSources = append(requestedSources, resolutions[i].SourceModule)
	}

	return requestedSources
}

func (o *Orchestrator) logDependencies(deps []string) {
	for i := range deps {
		o.Logger.Printf("dependency: %s", deps[i])
	}
}

func (o *Orchestrator) buildSyncPlan(
	cfg *config.Config,
	snapshot *store.Snapshot,
	resolutions []resolver.Resolution,
	depSources []string,
) (*syncer.SyncInput, *syncer.Plan, error) {
	syncInput, err := PrepareSyncInput(cfg, snapshot, resolutions, depSources)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare sync input: %w", err)
	}

	o.logDestinationNormalization(&syncInput)

	plan, err := o.compareManagedFiles(&syncInput)
	if err != nil {
		return nil, nil, fmt.Errorf("build sync plan: %w", err)
	}

	return &syncInput, plan, nil
}

func (o *Orchestrator) logDestinationNormalization(syncInput *syncer.SyncInput) {
	o.Logger.Group("Normalize destination names", func() {
		for source := range syncInput.SourceToDest {
			o.Logger.Printf(fmtArrow, source, syncInput.SourceToDest[source])
		}
	})
}

func (o *Orchestrator) compareManagedFiles(syncInput *syncer.SyncInput) (*syncer.Plan, error) {
	//nolint:wrapcheck // runGroup wraps the error with the group title
	return runGroup(o.Logger, "Compare managed files", func() (*syncer.Plan, error) {
		built, planErr := syncer.BuildPlan(syncInput)
		if planErr != nil {
			return nil, fmt.Errorf("build plan: %w", planErr)
		}

		o.Logger.Printf("Changed: %t", built.Changed)
		o.Logger.Printf(
			"Added: %d Updated: %d Removed: %d",
			len(built.Added),
			len(built.Updated),
			len(built.Removed),
		)

		return built, nil
	})
}

func (o *Orchestrator) applyChangedPlan(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
	syncInput *syncer.SyncInput,
	ref *store.RefInfo,
	result *Result,
) error {
	defaultBranch, err := o.maybeResolveDefaultBranch(ctx, cfg, plan)
	if err != nil {
		return fmt.Errorf("resolve default branch: %w", err)
	}

	err = o.copyTaskModules(plan, syncInput)
	if err != nil {
		return fmt.Errorf("copy task modules: %w", err)
	}

	err = o.maybeCommitAndPush(ctx, cfg, plan)
	if err != nil {
		return fmt.Errorf("commit and push: %w", err)
	}

	err = o.maybeCreateOrUpdatePR(ctx, cfg, plan, ref, defaultBranch, result)
	if err != nil {
		return fmt.Errorf("create or update PR: %w", err)
	}

	return nil
}

func (o *Orchestrator) maybeResolveDefaultBranch(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
) (string, error) {
	if !git.IsGitRepo(cfg.Workspace) {
		return consts.Empty, nil
	}

	base, err := o.runGitPreconditions(ctx, cfg, plan)
	if err != nil {
		return consts.Empty, fmt.Errorf("run git preconditions: %w", err)
	}

	return base, nil
}

func (o *Orchestrator) copyTaskModules(plan *syncer.Plan, syncInput *syncer.SyncInput) error {
	err := runGroupNoResult(o.Logger, "Copy task modules", func() error {
		applyErr := syncer.ApplyPlan(plan, syncInput)
		if applyErr != nil {
			return fmt.Errorf("apply plan: %w", applyErr)
		}

		o.Logger.Printf("Copied modules and validated generated YAML")

		return nil
	})
	if err != nil {
		return fmt.Errorf("apply sync plan: %w", err)
	}

	return nil
}

func (o *Orchestrator) maybeCommitAndPush(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
) error {
	if !git.IsGitRepo(cfg.Workspace) {
		return nil
	}

	//nolint:wrapcheck // runGroupNoResult wraps the error with the group title
	return runGroupNoResult(o.Logger, "Create synchronization commit", func() error {
		err := o.runGitSync(ctx, cfg, plan)
		if err != nil {
			return fmt.Errorf("run git sync: %w", err)
		}

		return nil
	})
}

func (o *Orchestrator) maybeCreateOrUpdatePR(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
	ref *store.RefInfo,
	defaultBranch string,
	result *Result,
) error {
	if !git.IsGitRepo(cfg.Workspace) || o.PRClient == nil {
		return nil
	}

	//nolint:wrapcheck // runGroupNoResult wraps the error with the group title
	return runGroupNoResult(o.Logger, "Create or update pull request", func() error {
		err := o.runPR(ctx, cfg, plan, ref, defaultBranch, result)
		if err != nil {
			return fmt.Errorf("run PR: %w", err)
		}

		return nil
	})
}

func (o *Orchestrator) run(ctx context.Context, cfg *config.Config) (*Result, error) {
	o.logValidateInputs(cfg)

	ref, snapshot, err := o.fetchStoreData(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("fetch store data: %w", err)
	}

	defer o.closeSnapshotQuietly(snapshot)

	syncInput, plan, result, err := o.planAndResult(cfg, snapshot, ref)
	if err != nil {
		return nil, fmt.Errorf("plan and build result: %w", err)
	}

	final, err := o.finishSync(ctx, cfg, plan, syncInput, ref, result)
	if err != nil {
		return nil, fmt.Errorf("finish sync: %w", err)
	}

	return final, nil
}

func (o *Orchestrator) finishSync(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
	syncInput *syncer.SyncInput,
	ref *store.RefInfo,
	result *Result,
) (*Result, error) {
	if !plan.Changed {
		o.reportUnchanged(cfg, plan, result)

		return result, nil
	}

	err := o.finishChangedPlan(ctx, cfg, plan, syncInput, ref, result)
	if err != nil {
		return nil, fmt.Errorf("finish changed plan: %w", err)
	}

	return result, nil
}

func (o *Orchestrator) closeSnapshotQuietly(snapshot *store.Snapshot) {
	closeErr := snapshot.Close()
	if closeErr != nil {
		o.Logger.Printf("close store snapshot: %v", closeErr)
	}
}

func (o *Orchestrator) reportUnchanged(cfg *config.Config, plan *syncer.Plan, result *Result) {
	o.Logger.Group(groupSummary, func() {
		printSummary(o.Logger, cfg, plan, result, consts.Empty)
	})
}

func (o *Orchestrator) finishChangedPlan(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
	syncInput *syncer.SyncInput,
	ref *store.RefInfo,
	result *Result,
) error {
	err := o.applyChangedPlan(ctx, cfg, plan, syncInput, ref, result)
	if err != nil {
		return fmt.Errorf("apply changed plan: %w", err)
	}

	o.Logger.Group(groupSummary, func() {
		printSummary(o.Logger, cfg, plan, result, result.PullRequestURL)
	})

	return nil
}

func (o *Orchestrator) logValidateInputs(cfg *config.Config) {
	o.Logger.Group("Validate inputs", func() {
		o.Logger.Printf("Validated %d task(s)", len(cfg.Tasks))
		o.Logger.Printf(fmtTargetFolder, cfg.TargetFolder)
	})
}

func (o *Orchestrator) fetchStoreData(
	ctx context.Context,
	cfg *config.Config,
) (*store.RefInfo, *store.Snapshot, error) {
	ref, err := o.resolveStoreRef(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve store ref: %w", err)
	}

	snapshot, err := o.downloadStoreSnapshot(ctx, &ref)
	if err != nil {
		return nil, nil, fmt.Errorf("download store snapshot: %w", err)
	}

	o.Logger.Group("Load module catalog", func() {
		o.Logger.Printf("Catalog modules: %d", len(snapshot.Catalog))
	})

	return &ref, snapshot, nil
}

func (o *Orchestrator) planAndResult(
	cfg *config.Config,
	snapshot *store.Snapshot,
	ref *store.RefInfo,
) (*syncer.SyncInput, *syncer.Plan, *Result, error) {
	resolutions, depSources, err := o.resolveModulesAndDeps(cfg, snapshot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve modules and deps: %w", err)
	}

	syncInput, plan, err := o.buildSyncPlan(cfg, snapshot, resolutions, depSources)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build sync plan: %w", err)
	}

	result, err := buildResult(cfg, plan, ref)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build result: %w", err)
	}

	return syncInput, plan, result, nil
}
