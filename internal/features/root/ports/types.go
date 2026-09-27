// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"github.com/task-otter/Taskotter/internal/features/plan/domain"
)

type (
	// RootTemplateProvider supplies the default root Taskfile document.
	RootTemplateProvider interface {
		NewRootTemplate() []byte
	}
	// IncludeRewriter rewrites module include paths in Taskfile YAML.
	IncludeRewriter interface {
		RewriteIncludes(
			content []byte,
			destinations map[string]string,
			targetFolder string,
		) ([]byte, error)
	}
	// RootTaskfileUpdater merges managed modules into the root Taskfile.
	RootTaskfileUpdater interface {
		UpdateRootTaskfile(content []byte, input *RootUpdateInput) ([]byte, error)
	}
	// GeneratedRootTask describes a TaskOtter-managed root task that fans out to
	// matching tasks in synced module includes.
	GeneratedRootTask = domain.GeneratedRootTask
	// RootUpdateInput carries data for updating the root Taskfile includes section.
	RootUpdateInput = domain.RootUpdateInput
)
