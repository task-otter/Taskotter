// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// TestWireOrchestratorInvalidRepository verifies an invalid repository coordinate fails construction.
func TestWireOrchestratorInvalidRepository(t *testing.T) {
	t.Parallel()

	orch, err := WireOrchestrator(t.Context(), invalidRepoOrchestratorConfig())
	iox.Discard(orch)

	if err == nil {
		t.Fatal("expected repository parse error")
	}
}

func invalidRepoOrchestratorConfig() *config.Config {
	return &config.Config{
		Tasks:              nil,
		JSRuntime:          consts.Empty,
		NodePackageManager: consts.Empty,
		NodeVersionManager: consts.Empty,
		IncludesDoc:        false,
		SyncRoot:           false,
		FailOnChanges:      false,
		StoreVersion:       consts.Empty,
		TargetFolder:       consts.Empty,
		RootTaskfile:       consts.Empty,
		GitHubToken:        consts.Empty,
		Workspace:          consts.Empty,
		Repository:         "not-a-valid-repo",
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}
}
