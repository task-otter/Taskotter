// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
)

type (
	// PrepareSyncInputFn prepares a sync input from store snapshot data.
	PrepareSyncInputFn func(*syncsvc.PrepareSyncInputArgs) (syncdomain.SyncInput, error)

	// BuildPlanFn compares managed files against the store snapshot.
	BuildPlanFn func(*syncdomain.SyncInput) (*syncdomain.Plan, error)

	// ApplyPlanFn copies planned module files into the workspace.
	ApplyPlanFn func(*syncdomain.Plan, *syncdomain.SyncInput) error

	// ResolveAllFn resolves requested logical tasks to store modules.
	ResolveAllFn func(*resolvesvc.ResolveAllInput) ([]resolvesvc.Resolution, error)

	// ResolveTransitiveFn resolves transitive module dependencies.
	ResolveTransitiveFn func([]string, map[string][]string) ([]string, error)
)

func (orch *Orchestrator) wireSyncHooks() {
	if orch.PrepareSyncInput == nil {
		orch.PrepareSyncInput = syncsvc.PrepareSyncInput
	}

	if orch.BuildPlan == nil {
		orch.BuildPlan = syncsvc.BuildPlan
	}

	if orch.ApplyPlan == nil {
		orch.ApplyPlan = syncsvc.ApplyPlan
	}

	orch.wireResolveHooks()
}

func (orch *Orchestrator) wireResolveHooks() {
	if orch.ResolveAll == nil {
		orch.ResolveAll = resolvesvc.ResolveAll
	}

	if orch.ResolveTransitive == nil {
		orch.ResolveTransitive = resolvesvc.ResolveTransitive
	}
}
