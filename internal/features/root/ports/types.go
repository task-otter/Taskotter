// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:revive // Ports intentionally expose each independent composition capability.
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

	// RootTemplateProvider supplies the default root Taskfile document.
	RootTemplateProvider interface {
		NewRootTemplate() []byte
	}

	// IncludeRewriter rewrites module include paths in Taskfile YAML.
	IncludeRewriter interface {
		RewriteIncludes(
			content []byte,
			sourceToDest map[string]string,
			fromDest string,
		) ([]byte, error)
	}

	// RootTaskfileUpdater merges managed modules into the root Taskfile.
	RootTaskfileUpdater interface {
		UpdateRootTaskfile(content []byte, input *RootUpdateInput) ([]byte, error)
	}

	// TaskfileOps groups the narrow Taskfile capabilities at the composition boundary.
	TaskfileOps interface {
		RootTemplateProvider
		IncludeRewriter
		RootTaskfileUpdater
	}
)
