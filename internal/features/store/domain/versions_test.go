// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/features/store/domain"
)

const (
	testSemanticVersion = "v1.2.3"
)

// TestSemanticVersion verifies tag normalization without accepting arbitrary names.
//
//nolint:funlen,maintidx // Explicit normalization cases keep accepted and rejected tags together.
func TestSemanticVersion(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		testSemanticVersion: testSemanticVersion,
		"1.2.3":             testSemanticVersion,
		"v1.2":              "v1.2.0",
		"1":                 "v1.0.0",
		"v1.2.3+build":      testSemanticVersion,
		"1.2.3-rc.1":        "v1.2.3-rc.1",
		"":                  "",
		"release":           "",
		"v01.2.3":           "",
		"V1.2.3":            "",
	}

	for tag := range cases {
		want := cases[tag]

		t.Run(tag, func(t *testing.T) {
			t.Parallel()

			if got := domain.SemanticVersion(tag); got != want {
				t.Fatalf("SemanticVersion(%q) = %q, want %q", tag, got, want)
			}
		})
	}
}
