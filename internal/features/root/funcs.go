// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package root

import (
	"bytes"

	"github.com/task-otter/Taskotter/internal/features/root/adapters/taskfile"
)

// NewOps creates the Taskfile operations used by the planner.
func NewOps() Ops {
	return taskfile.NewOps()
}

// Generate updates root Taskfile content without writing to the workspace.
func Generate(input Input, ops Operations) (Result, error) {
	content := input.ExistingRoot

	if !input.RootExists {
		content = ops.NewRootTemplate()
	}

	updated, err := ops.UpdateRootTaskfile(content, &UpdateInput{
		Tasks:            input.Tasks,
		TargetFolder:     input.TargetFolder,
		RootTaskfileDir:  input.RootTaskfileDir,
		DestByTask:       input.DestByTask,
		ManagedTasks:     input.ManagedTasks,
		ModuleTaskfiles:  input.ModuleTaskfiles,
		GeneratedTasks:   input.GeneratedTasks,
		ManagedRootTasks: input.ManagedRootTasks,
	})
	if err != nil {
		return Result{}, err
	}

	return Result{
		Content:        updated,
		GeneratedTasks: input.GeneratedTasks,
		Changed:        !bytes.Equal(content, updated),
	}, nil
}

// Generate updates root content using the configured operations.
func (g DefaultGenerator) Generate(input Input) (Result, error) {
	ops := g.Ops

	if ops == nil {
		ops = NewOps()
	}

	return Generate(input, ops)
}
