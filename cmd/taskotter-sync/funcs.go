// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"context"
	"fmt"

	gitcli "github.com/task-otter/Taskotter/internal/features/git/adapters/cli"
	prgithub "github.com/task-otter/Taskotter/internal/features/pr/adapters/github"
	storegithub "github.com/task-otter/Taskotter/internal/features/store/adapters/github"
	syncrun "github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

func main() { exitFunc(run()) }

func run() int {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)

	defer cancel()

	cfg, result, err := loadRunAndWrite(ctx)
	if err != nil {
		reportError(err, "")

		return exitError
	}

	return reportResult(cfg, result)
}

func loadRunAndWrite(ctx context.Context) (cfg *config.Config, result *syncrun.Result, err error) {
	cfg, err = config.LoadFromEnv()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	result, err = runOrchestrator(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("run sync: %w", err)
	}

	err = writeOutputs(cfg, result)
	if err != nil {
		return nil, nil, fmt.Errorf("write action outputs: %w", err)
	}

	return cfg, result, nil
}

func defaultWireRun(ctx context.Context, cfg *config.Config) (runSyncFn, error) {
	orch, err := WireOrchestrator(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf(errWireOrchFmt, err)
	}

	return orch.Run, nil
}

func runOrchestrator(ctx context.Context, cfg *config.Config) (*syncrun.Result, error) {
	runSync, err := wireRun(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf(errWireOrchFmt, err)
	}

	result, err := runSync(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("run orchestrator: %w", err)
	}

	return result, nil
}

func writeOutputs(cfg *config.Config, result *syncrun.Result) error {
	err := syncrun.WriteActionOutputs(cfg, result)
	if err != nil {
		return fmt.Errorf("write outputs: %w", err)
	}

	return nil
}

func reportResult(cfg *config.Config, result *syncrun.Result) int {
	if result.Changed {
		return handleChanged(cfg, result)
	}

	return handleUnchanged(cfg, result)
}

func reportError(err error, prefix string) {
	iox.FprintfBestEffortf(stderr, "::error::%s%v\n", prefix, err)
}

func handleChanged(cfg *config.Config, result *syncrun.Result) int {
	err := iox.Fprintln(stdout, "TaskOtter produced changes.")
	if err != nil {
		return exitError
	}

	if cfg.FailOnChanges {
		syncrun.ReportSyncRequired(result)

		return exitError
	}

	return exitSuccess
}

func handleUnchanged(cfg *config.Config, result *syncrun.Result) int {
	err := iox.Fprintln(stdout, "TaskOtter completed with no changes.")
	if err != nil {
		return exitError
	}

	if cfg.FailOnChanges {
		syncrun.ReportSyncUpToDate(result)
	}

	return exitSuccess
}

// WireOrchestrator builds an Orchestrator with concrete adapters from configuration.
func WireOrchestrator(ctx context.Context, cfg *config.Config) (*syncrun.Orchestrator, error) {
	client := gitcli.NewClient(cfg.Workspace)
	deps := newWiredDeps(ctx, cfg, client)

	err := wirePRClient(ctx, cfg, deps)
	if err != nil {
		return nil, fmt.Errorf("wire PR client: %w", err)
	}

	return syncrun.NewOrchestrator(deps), nil
}

func newWiredDeps(ctx context.Context, cfg *config.Config, client *gitcli.Client) *syncrun.Deps {
	return &syncrun.Deps{
		Logger:            logging.New(),
		StoreClient:       storegithub.NewClient(ctx, cfg.GitHubToken),
		GitClient:         client,
		GitBrancher:       client,
		GitIndexer:        client,
		GitPublisher:      client,
		PRClient:          nil,
		PrepareSyncInput:  nil,
		BuildPlan:         nil,
		ApplyPlan:         nil,
		ResolveAll:        nil,
		ResolveTransitive: nil,
	}
}

func wirePRClient(ctx context.Context, cfg *config.Config, deps *syncrun.Deps) error {
	if cfg.Repository == consts.Empty {
		return nil
	}

	prClient, err := prgithub.NewClient(ctx, cfg.GitHubToken, cfg.Repository)
	if err != nil {
		return fmt.Errorf(errCreatePRClient, err)
	}

	deps.PRClient = prClient

	return nil
}
