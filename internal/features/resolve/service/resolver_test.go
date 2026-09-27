// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

// TestTransitiveResolverResolveReturnsDependencies verifies the behavior covered by this test.
func TestTransitiveResolverResolveReturnsDependencies(t *testing.T) {
	t.Parallel()

	got, err := (transitiveResolver{}).Resolve([]string{resolveApp}, map[string][]string{
		resolveApp: {resolveLib},
		resolveLib: nil,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(got) != consts.IndexOne || got[consts.IndexZero] != resolveLib {
		t.Fatalf(fmtResolve, got)
	}
}

// TestTransitiveResolverResolveWrapsErrors verifies the behavior covered by this test.
func TestTransitiveResolverResolveWrapsErrors(t *testing.T) {
	t.Parallel()

	got, err := (transitiveResolver{}).Resolve([]string{"missing"}, map[string][]string{})
	if err == nil {
		t.Fatal("expected error")
	}

	if got != nil {
		t.Fatalf(fmtResolve, got)
	}
}

// TestLevenshtein verifies the behavior covered by this test.
func TestLevenshtein(t *testing.T) {
	t.Parallel()

	if got := Levenshtein(resolveTask, resolveTask); got != scoreIdenticalString {
		t.Fatalf("Levenshtein() = %d", got)
	}
}
