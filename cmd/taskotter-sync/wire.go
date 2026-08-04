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
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

const (
	errCreatePRClient = "create GitHub PR client: %w"
)

// WireOrchestrator builds an Orchestrator with concrete adapters from configuration.
func WireOrchestrator(ctx context.Context, cfg *config.Config) (*syncrun.Orchestrator, error) {
	client := gitcli.NewClient(cfg.Workspace)

	orch := &syncrun.Orchestrator{
		Logger:       logging.New(),
		StoreClient:  storegithub.NewClient(ctx, cfg.GitHubToken),
		GitClient:    client,
		GitBrancher:  client,
		GitIndexer:   client,
		GitPublisher: client,
		PRClient:     nil,
	}

	err := wirePRClient(ctx, cfg, orch)
	if err != nil {
		return nil, fmt.Errorf("wire PR client: %w", err)
	}

	return orch, nil
}

func wirePRClient(ctx context.Context, cfg *config.Config, orch *syncrun.Orchestrator) error {
	if cfg.Repository == consts.Empty {
		return nil
	}

	prClient, err := prgithub.NewClient(ctx, cfg.GitHubToken, cfg.Repository)
	if err != nil {
		return fmt.Errorf(errCreatePRClient, err)
	}

	orch.PRClient = prClient

	return nil
}
