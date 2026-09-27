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
	notFoundMsg     = "not found in store"
	closeMatches    = "close matches"
	errorFmt        = "Error() = %q"
	attemptedModule = "go/node/npm"
)

func assertContains(t *testing.T, msg, needle string) {
	t.Helper()

	if !strings.Contains(msg, needle) {
		t.Fatalf(errorFmt, msg)
	}
}

func assertOmits(t *testing.T, msg, needle string) {
	t.Helper()

	if strings.Contains(msg, needle) {
		t.Fatalf(errorFmt, msg)
	}
}

// TestResolveErrorMessageIncludesAllDetails verifies attempted module and close matches appear.
func TestResolveErrorMessageIncludesAllDetails(t *testing.T) {
	t.Parallel()

	resolveErr := &domain.ResolveError{
		LogicalTask:  consts.Go,
		Attempted:    attemptedModule,
		Message:      notFoundMsg,
		CloseMatches: []string{consts.Go, "golang"},
	}

	msg := resolveErr.Error()
	assertContains(t, msg, "attempted source module")
	assertContains(t, msg, closeMatches)

	if resolveErr.Detail() != msg {
		t.Fatalf("Detail() = %q, want %q", resolveErr.Detail(), msg)
	}
}

// TestResolveErrorMessageOmitsOptionalDetails verifies the minimal message form.
func TestResolveErrorMessageOmitsOptionalDetails(t *testing.T) {
	t.Parallel()

	resolveErr := &domain.ResolveError{
		LogicalTask: consts.Go,
		Message:     notFoundMsg,
	}

	msg := resolveErr.Error()
	assertOmits(t, msg, "attempted")
	assertOmits(t, msg, closeMatches)
}

// TestResolveErrorEmptyMessageBranches covers Error paths after Message is empty.
func TestResolveErrorEmptyMessageBranches(t *testing.T) {
	t.Parallel()

	assertContains(
		t,
		(&domain.ResolveError{LogicalTask: consts.Go, Attempted: attemptedModule}).Error(),
		attemptedModule,
	)
	assertContains(
		t,
		(&domain.ResolveError{LogicalTask: consts.Go, CloseMatches: []string{consts.Go}}).Error(),
		closeMatches,
	)
	assertContains(t, (&domain.ResolveError{LogicalTask: consts.Go}).Error(), `task "go"`)
	assertContains(t, (&domain.ResolveError{}).Error(), `task ""`)
}

// TestResolveErrorTask covers Task().
func TestResolveErrorTask(t *testing.T) {
	t.Parallel()

	resolveErr := &domain.ResolveError{LogicalTask: consts.Go}

	if resolveErr.Task() != consts.Go {
		t.Fatalf("Task() = %q", resolveErr.Task())
	}
}
