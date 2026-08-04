// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package domain holds resolve feature types and pure error helpers.
package domain

import (
	"fmt"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	// Resolution records the resolved source module for a logical task.
	Resolution struct {
		LogicalTask  string
		SourceModule string
	}

	// ResolveError reports task resolution failures with optional close matches.
	ResolveError struct {
		LogicalTask  string
		Attempted    string
		Message      string
		CloseMatches []string
	}
)

// Error implements the error interface, returning the task resolution failure message.
func (resolveErr *ResolveError) Error() string {
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
