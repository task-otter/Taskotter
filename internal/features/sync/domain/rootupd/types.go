// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package rootupd holds root Taskfile update domain types.
package rootupd

type (
	// GeneratedRootTask describes a TaskOtter-managed root task that fans out to
	// matching tasks in synced module includes.
	GeneratedRootTask struct {
		Name    string
		Modules []string
	}

	// RootUpdateInput carries data for updating the root Taskfile includes section.
	RootUpdateInput struct {
		Tasks            []string
		TargetFolder     string
		RootTaskfileDir  string
		DestByTask       map[string]string
		ManagedTasks     []string
		ModuleTaskfiles  map[string][]byte
		GeneratedTasks   []GeneratedRootTask
		ManagedRootTasks []string
	}
)
