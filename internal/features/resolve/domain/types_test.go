// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain_test

import (
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/resolve/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	notFoundMsg  = "not found in store"
	closeMatches = "close matches"
	errorFmt     = "Error() = %q"
)

// TestResolveErrorMessageIncludesAllDetails verifies attempted module and close matches appear.
func TestResolveErrorMessageIncludesAllDetails(t *testing.T) {
	t.Parallel()

	resolveErr := &domain.ResolveError{
		LogicalTask:  consts.Go,
		Attempted:    "go/node/npm",
		Message:      notFoundMsg,
		CloseMatches: []string{"go", "golang"},
	}

	msg := resolveErr.Error()

	if !strings.Contains(msg, "attempted source module") || !strings.Contains(msg, closeMatches) {
		t.Fatalf(errorFmt, msg)
	}
}

// TestResolveErrorMessageOmitsOptionalDetails verifies the minimal message form.
func TestResolveErrorMessageOmitsOptionalDetails(t *testing.T) {
	t.Parallel()

	resolveErr := &domain.ResolveError{
		LogicalTask:  consts.Go,
		Attempted:    consts.Empty,
		Message:      notFoundMsg,
		CloseMatches: nil,
	}

	msg := resolveErr.Error()

	if strings.Contains(msg, "attempted") || strings.Contains(msg, closeMatches) {
		t.Fatalf(errorFmt, msg)
	}
}
