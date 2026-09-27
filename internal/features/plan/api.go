// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package plan

import (
	"github.com/task-otter/Taskotter/internal/features/input"
	"github.com/task-otter/Taskotter/internal/features/plan/domain"
	"github.com/task-otter/Taskotter/internal/features/root"
)

type (
	// Plan is the complete desired synchronization result consumed by apply.
	Plan = domain.Plan
	// Input is the resolved module set consumed by the planner.
	Input = input.Input

	// Builder owns plan construction. Collector and Root are retained as
	// composition slots while their concrete operations are extracted.
	Builder struct {
		Collector any
		Root      root.Generator
	}
)

// Build computes the desired synchronization plan.
func Build(input *Input) (*Plan, error) {
	return BuildPlan(input)
}

// Build computes a synchronization plan through the feature boundary.
func (b *Builder) Build(input *Input) (*Plan, error) {
	return Build(input)
}
