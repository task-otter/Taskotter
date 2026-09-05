// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package domain holds resolve feature types and pure error helpers.
package domain

import (
	"fmt"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

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
