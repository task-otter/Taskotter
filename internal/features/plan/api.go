// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package plan

import (
	"fmt"

	inputpkg "github.com/task-otter/Taskotter/internal/features/input"
	"github.com/task-otter/Taskotter/internal/features/plan/domain"
)

type (
	// Plan is the complete desired synchronization result consumed by apply.
	Plan = domain.Plan
	// Input is the resolved module set consumed by the planner.
	Input = inputpkg.Input
)

// Build computes the desired synchronization plan.
func Build(syncInput *Input) (*Plan, error) {
	plan, err := BuildPlan(syncInput)
	if err != nil {
		return nil, fmt.Errorf("build plan: %w", err)
	}

	return plan, nil
}
