// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

type (
	// Snapshot provides module paths and store ref metadata without importing store adapters.
	Snapshot interface {
		ModuleDir(sourceModule string) string
		WorkspaceRoot() string
		SourceRef() string
		ResolvedCommit() string
		DefaultBranch() string
	}

	// GeneratedRootTask describes a TaskOtter-managed root task that fans out to
	// matching tasks in synced module includes.
	GeneratedRootTask = struct {
		Name    string
		Modules []string
	}

	// RootUpdateInput carries data for updating the root Taskfile includes section.
	RootUpdateInput = struct {
		Tasks            []string
		TargetFolder     string
		RootTaskfileDir  string
		DestByTask       map[string]string
		ManagedTasks     []string
		ModuleTaskfiles  map[string][]byte
		GeneratedTasks   []GeneratedRootTask
		ManagedRootTasks []string
	}

	// TaskfileOps rewrites and updates Taskfile YAML without the sync service importing adapters.
	TaskfileOps interface {
		NewRootTemplate() []byte
		RewriteIncludes(
			content []byte,
			sourceToDest map[string]string,
			fromDest string,
		) ([]byte, error)
		UpdateRootTaskfile(content []byte, input *RootUpdateInput) ([]byte, error)
	}
)
