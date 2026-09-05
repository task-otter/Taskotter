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
