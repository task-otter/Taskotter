// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package root

import (
	"github.com/task-otter/Taskotter/internal/features/root/adapters/taskfile"
	"github.com/task-otter/Taskotter/internal/features/root/ports"
)

type (
	Operations       = ports.TaskfileOps
	TemplateProvider = ports.RootTemplateProvider
	IncludeRewriter  = ports.IncludeRewriter
	RootUpdater      = ports.RootTaskfileUpdater
	RootUpdateInput  = ports.RootUpdateInput
	GeneratedTask    = ports.GeneratedRootTask
	Ops              = taskfile.Ops

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
