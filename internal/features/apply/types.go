// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package apply

import (
	"errors"

	"github.com/task-otter/Taskotter/internal/features/input"
	"github.com/task-otter/Taskotter/internal/features/plan"
)

type (
	Filesystem any
	Applier    struct {
		Filesystem Filesystem
	}
	Plan  = plan.Plan
	Input = input.Input
)

var ErrMissingInput = errors.New("sync plan does not contain apply input")
