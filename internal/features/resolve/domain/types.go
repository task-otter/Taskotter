// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// Resolution records the resolved source module for a logical task.
	Resolution = struct {
		LogicalTask  string
		SourceModule string
	}

	// TaskError is a task resolution failure.
	TaskError interface {
		error
		Task() string
	}

	// ResolveError reports task resolution failures with optional close matches.
	ResolveError struct {
		// LogicalTask is the requested task name.
		LogicalTask string
		// Attempted is the source module that was tried, if any.
		Attempted string
		// Message is the failure reason.
		Message string
		// CloseMatches lists similar catalog entries.
		CloseMatches []string
	}
)
