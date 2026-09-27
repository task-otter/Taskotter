// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package apply

import planpkg "github.com/task-otter/Taskotter/internal/features/plan"

// Apply applies the plan using the existing atomic staging and cleanup rules.
func Apply(plan *Plan) error {
	if plan == nil || plan.Input == nil {
		return ErrMissingInput
	}

	return planpkg.ApplyPlan(plan, plan.Input)
}

// Apply applies a plan through the configured applier. Filesystem is reserved
// for the concrete writer boundary used by the next extraction stage.
func (*Applier) Apply(plan *Plan) error {
	return Apply(plan)
}
