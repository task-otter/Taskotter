// Package app orchestrates the TaskOtter sync pipeline from store resolution through git and PR creation.
package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/dependency"
	"github.com/task-otter/Taskotter/internal/git"
	gh "github.com/task-otter/Taskotter/internal/github"
	"github.com/task-otter/Taskotter/internal/logging"
	"github.com/task-otter/Taskotter/internal/resolver"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
)

var errUnrelatedChanges = errors.New("unrelated uncommitted changes detected in workspace")

// StoreClient resolves store refs and downloads snapshots for the sync pipeline.
type StoreClient interface {
	ResolveRef(ctx context.Context, requestedVersion string) (store.RefInfo, error)
	DownloadSnapshot(ctx context.Context, ref store.RefInfo) (*store.Snapshot, error)
}

// Orchestrator coordinates store, git, and GitHub operations for a sync run.
type Orchestrator struct {
	Logger      *logging.Logger
	StoreClient StoreClient
	GitOps      git.Operations
	PRClient    gh.PRClient
}

// Run creates an orchestrator from configuration and executes the sync pipeline.
func Run(ctx context.Context, cfg *config.Config) (*Result, error) {
	o, err := NewOrchestrator(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return o.Run(ctx, cfg)
}

// NewOrchestrator builds an Orchestrator with default clients wired from configuration.
func NewOrchestrator(ctx context.Context, cfg *config.Config) (*Orchestrator, error) {
	orch := &Orchestrator{
		Logger:      logging.New(),
		StoreClient: store.NewClient(ctx, cfg.GitHubToken),
		GitOps:      git.NewClient(cfg.Workspace),
		PRClient:    nil,
	}
	if cfg.Repository != "" {
		prClient, err := gh.NewClient(ctx, cfg.GitHubToken, cfg.Repository)
		if err != nil {
			return nil, fmt.Errorf("create GitHub PR client: %w", err)
		}

		orch.PRClient = prClient
	}

	return orch, nil
}

// Run executes the full sync pipeline.
func (o *Orchestrator) Run(ctx context.Context, cfg *config.Config) (*Result, error) {
	err := o.wireDefaults(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return o.run(ctx, cfg)
}

func runGroup[T any](logger *logging.Logger, title string, groupFn func() (T, error)) (T, error) {
	var (
		out T
		err error
	)

	logger.Group(title, func() {
		out, err = groupFn()
	})

	return out, err
}

func runGroupNoResult(logger *logging.Logger, title string, groupFn func() error) error {
	var err error

	logger.Group(title, func() {
		err = groupFn()
	})

	return err
}

func (o *Orchestrator) wireDefaults(ctx context.Context, cfg *config.Config) error {
	if o.Logger == nil {
		o.Logger = logging.New()
	}

	if o.StoreClient == nil {
		o.StoreClient = store.NewClient(ctx, cfg.GitHubToken)
	}

	if o.GitOps == nil {
		o.GitOps = git.NewClient(cfg.Workspace)
	}

	if o.PRClient == nil && cfg.Repository != "" {
		prClient, err := gh.NewClient(ctx, cfg.GitHubToken, cfg.Repository)
		if err != nil {
			return fmt.Errorf("create GitHub PR client: %w", err)
		}

		o.PRClient = prClient
	}

	return nil
}

func (o *Orchestrator) runGitPreconditions(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
) (string, error) {
	err := o.GitOps.EnsureSafeDirectory(ctx)
	if err != nil {
		return "", fmt.Errorf("ensure safe directory: %w", err)
	}

	err = git.WriteLocalIdentity(ctx, o.GitOps)
	if err != nil {
		return "", fmt.Errorf("write local git identity: %w", err)
	}

	err = git.EnsureBranchOwned(ctx, o.GitOps, cfg.BranchName)
	if err != nil {
		return "", fmt.Errorf("ensure branch owned: %w", err)
	}

	allowed := git.AllowedPathSet(plan.StagePaths)

	unrelated, err := o.GitOps.HasUnrelatedChanges(ctx, allowed)
	if err != nil {
		return "", fmt.Errorf("check unrelated changes: %w", err)
	}

	if unrelated {
		return "", errUnrelatedChanges
	}

	if cfg.BaseBranch != "" {
		return cfg.BaseBranch, nil
	}

	defaultBranch, err := o.GitOps.DefaultBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve pull request base branch: %w", err)
	}

	return defaultBranch, nil
}

func (o *Orchestrator) runGitSync(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
) error {
	err := o.GitOps.CheckoutBranch(ctx, cfg.BranchName, true)
	if err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}

	err = o.GitOps.Stage(ctx, plan.StagePaths)
	if err != nil {
		return fmt.Errorf("stage paths: %w", err)
	}

	err = o.GitOps.Commit(ctx, git.SyncCommitMessage)
	if err != nil {
		return fmt.Errorf("commit changes: %w", err)
	}

	err = o.GitOps.Push(ctx, cfg.BranchName, true)
	if err != nil {
		return fmt.Errorf("push branch: %w", err)
	}

	return nil
}

func (o *Orchestrator) runPR(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
	ref store.RefInfo,
	defaultBranch string,
	result *Result,
) error {
	body := gh.BuildPRBody(cfg, plan, gh.StoreRefFrom(ref))

	existing, err := o.PRClient.FindOpenPR(ctx, cfg.BranchName, defaultBranch)
	if err != nil && !errors.Is(err, gh.ErrPullRequestNotFound) {
		return fmt.Errorf("find open pull request: %w", err)
	}

	if existing != nil {
		err = o.PRClient.UpdatePRBody(ctx, existing.Number, body)
		if err != nil {
			return fmt.Errorf("update pull request body: %w", err)
		}

		result.PullRequestNumber = strconv.Itoa(existing.Number)
		result.PullRequestURL = existing.URL
		o.Logger.Printf("Updated pull request #%d", existing.Number)

		return nil
	}

	pullReq, err := o.PRClient.CreatePR(ctx, cfg.BranchName, defaultBranch, body)
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
	ref store.RefInfo,
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
	resolutions, err := runGroup(
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

			for _, res := range resolved {
				o.Logger.Printf("%s -> %s", res.LogicalTask, res.SourceModule)
			}

			return resolved, nil
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve requested modules: %w", err)
	}

	depSources, err := runGroup(o.Logger, "Resolve dependencies", func() ([]string, error) {
		requestedSources := make([]string, 0, len(resolutions))
		for _, res := range resolutions {
			requestedSources = append(requestedSources, res.SourceModule)
		}

		deps, depErr := dependency.ResolveTransitive(requestedSources, snapshot.Deps)
		if depErr != nil {
			return nil, fmt.Errorf("resolve transitive dependencies: %w", depErr)
		}

		for _, dep := range deps {
			o.Logger.Printf("dependency: %s", dep)
		}

		return deps, nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("resolve dependencies: %w", err)
	}

	return resolutions, depSources, nil
}

func (o *Orchestrator) buildSyncPlan(
	cfg *config.Config,
	snapshot *store.Snapshot,
	resolutions []resolver.Resolution,
	depSources []string,
) (syncer.SyncInput, *syncer.Plan, error) {
	syncInput, err := PrepareSyncInput(cfg, snapshot, resolutions, depSources)
	if err != nil {
		return syncer.SyncInput{}, nil, fmt.Errorf("prepare sync input: %w", err)
	}

	o.Logger.Group("Normalize destination names", func() {
		for source, dest := range syncInput.SourceToDest {
			o.Logger.Printf("%s -> %s", source, dest)
		}
	})

	plan, err := runGroup(o.Logger, "Compare managed files", func() (*syncer.Plan, error) {
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
	if err != nil {
		return syncer.SyncInput{}, nil, fmt.Errorf("build sync plan: %w", err)
	}

	return syncInput, plan, nil
}

func (o *Orchestrator) applyChangedPlan(
	ctx context.Context,
	cfg *config.Config,
	plan *syncer.Plan,
	syncInput syncer.SyncInput,
	ref store.RefInfo,
	result *Result,
) error {
	var defaultBranch string

	if git.IsGitRepo(cfg.Workspace) {
		var err error

		defaultBranch, err = o.runGitPreconditions(ctx, cfg, plan)
		if err != nil {
			return err
		}
	}

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

	if git.IsGitRepo(cfg.Workspace) {
		err = runGroupNoResult(o.Logger, "Create synchronization commit", func() error {
			return o.runGitSync(ctx, cfg, plan)
		})
		if err != nil {
			return err
		}
	}

	if git.IsGitRepo(cfg.Workspace) && o.PRClient != nil {
		err = runGroupNoResult(o.Logger, "Create or update pull request", func() error {
			return o.runPR(ctx, cfg, plan, ref, defaultBranch, result)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *Orchestrator) run(ctx context.Context, cfg *config.Config) (*Result, error) {
	o.Logger.Group("Validate inputs", func() {
		o.Logger.Printf("Validated %d task(s)", len(cfg.Tasks))
		o.Logger.Printf("Target folder: %s", cfg.TargetFolder)
	})

	ref, err := o.resolveStoreRef(ctx, cfg)
	if err != nil {
		return nil, err
	}

	snapshot, err := o.downloadStoreSnapshot(ctx, ref)
	if err != nil {
		return nil, err
	}

	defer func() {
		closeErr := snapshot.Close()
		if closeErr != nil {
			o.Logger.Printf("close store snapshot: %v", closeErr)
		}
	}()

	o.Logger.Group("Load module catalog", func() {
		o.Logger.Printf("Catalog modules: %d", len(snapshot.Catalog))
	})

	resolutions, depSources, err := o.resolveModulesAndDeps(cfg, snapshot)
	if err != nil {
		return nil, err
	}

	syncInput, plan, err := o.buildSyncPlan(cfg, snapshot, resolutions, depSources)
	if err != nil {
		return nil, err
	}

	result, err := buildResult(cfg, plan, ref)
	if err != nil {
		return nil, err
	}

	if !plan.Changed {
		o.Logger.Group("Summary", func() {
			printSummary(o.Logger, cfg, plan, result, "")
		})

		return result, nil
	}

	err = o.applyChangedPlan(ctx, cfg, plan, syncInput, ref, result)
	if err != nil {
		return nil, err
	}

	o.Logger.Group("Summary", func() {
		printSummary(o.Logger, cfg, plan, result, result.PullRequestURL)
	})

	return result, nil
}
