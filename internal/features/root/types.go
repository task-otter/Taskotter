// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package root

import (
	"github.com/task-otter/Taskotter/internal/features/root/adapters/taskfile"
	"github.com/task-otter/Taskotter/internal/features/root/ports"
)

type (
	// Operations groups root Taskfile operations.
	Operations = ports.TaskfileOps
	// TemplateProvider creates root Taskfile templates.
	TemplateProvider = ports.RootTemplateProvider
	// IncludeRewriter rewrites module includes.
	IncludeRewriter = ports.IncludeRewriter
	// Updater updates the root Taskfile.
	Updater = ports.RootTaskfileUpdater
	// UpdateInput contains root Taskfile update inputs.
	UpdateInput = ports.RootUpdateInput
	// GeneratedTask describes a generated root task.
	GeneratedTask = ports.GeneratedRootTask
	// Ops groups all root operations.
	Ops = taskfile.Ops

	// Input contains root generation inputs.
	Input struct {
		DestByTask       map[string]string
		ModuleTaskfiles  map[string][]byte
		TargetFolder     string
		RootTaskfileDir  string
		ExistingRoot     []byte
		Tasks            []string
		ManagedTasks     []string
		GeneratedTasks   []GeneratedTask
		ManagedRootTasks []string
		RootExists       bool
	}

	// Result contains generated root content.
	Result struct {
		Content        []byte
		GeneratedTasks []GeneratedTask
		Changed        bool
	}
)

// Generator is the planning boundary for generated root Taskfile content.
type Generator interface {
	Generate(Input) (Result, error)
}

// DefaultGenerator uses the Taskfile adapter at the composition boundary.
type DefaultGenerator struct {
	Ops Operations
}
