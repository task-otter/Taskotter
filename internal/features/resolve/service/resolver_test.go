// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import "testing"

// TestTransitiveResolverResolveReturnsDependencies verifies the behavior covered by this test.
func TestTransitiveResolverResolveReturnsDependencies(t *testing.T) {
	t.Parallel()

	got, err := (transitiveResolver{}).Resolve([]string{"app"}, map[string][]string{
		"app": {"lib"},
		"lib": nil,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(got) != 1 || got[0] != "lib" {
		t.Fatalf("Resolve() = %#v", got)
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
		t.Fatalf("Resolve() = %#v", got)
	}
}

// TestLevenshtein verifies the behavior covered by this test.
func TestLevenshtein(t *testing.T) {
	t.Parallel()

	if got := Levenshtein("task", "task"); got != scoreIdenticalString {
		t.Fatalf("Levenshtein() = %d", got)
	}
}
