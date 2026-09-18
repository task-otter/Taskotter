// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"fmt"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// Detail returns the full resolution failure message including close matches.
func (resolveErr *ResolveError) Detail() string {
	msg := fmt.Sprintf(`task %q`, resolveErr.LogicalTask)

	if resolveErr.Attempted != "" {
		msg += fmt.Sprintf(" (attempted source module %q)", resolveErr.Attempted)
	}

	msg += ": " + resolveErr.Message

	if len(resolveErr.CloseMatches) > consts.IndexZero {
		msg += "; close matches: " + strings.Join(resolveErr.CloseMatches, ", ")
	}

	return msg
}

// Error implements the error interface, returning the task resolution failure message.
func (resolveErr *ResolveError) Error() string {
	if resolveErr.Message != consts.Empty {
		return resolveErr.Detail()
	}

	if resolveErr.Attempted != consts.Empty {
		return resolveErr.Detail()
	}

	if len(resolveErr.CloseMatches) > consts.IndexZero {
		return resolveErr.Detail()
	}

	if resolveErr.LogicalTask != consts.Empty {
		return resolveErr.Detail()
	}

	return resolveErr.Detail()
}

// Task returns the logical task that failed to resolve.
func (resolveErr *ResolveError) Task() string {
	iox.Discard(
		resolveErr.Attempted + resolveErr.Message + strings.Join(
			resolveErr.CloseMatches,
			consts.Empty,
		),
	)

	return resolveErr.LogicalTask
}
