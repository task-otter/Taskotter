// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package apply

import (
	"errors"

	"github.com/task-otter/Taskotter/internal/features/input"
	"github.com/task-otter/Taskotter/internal/features/plan"
)

type (
	// Filesystem is the filesystem dependency used during apply.
	Filesystem any
	// Applier applies a synchronization plan.
	Applier struct {
		Filesystem Filesystem
	}
	// Plan describes the changes to apply.
	Plan = plan.Plan
	// Input contains the state needed to apply a plan.
	Input = input.Input
)

// ErrMissingInput indicates that a plan does not contain apply input.
var ErrMissingInput = errors.New("sync plan does not contain apply input")
