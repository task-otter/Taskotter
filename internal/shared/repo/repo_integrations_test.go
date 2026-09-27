// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package repo_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/repo"
)

// TestParse verifies a valid owner/name repository coordinate parses correctly.
func TestParse(t *testing.T) {
	t.Parallel()

	owner, name, err := repo.Parse("task-otter/Taskotter")
	if err != nil {
		t.Fatal(err)
	}

	if owner != "task-otter" || name != "Taskotter" {
		t.Fatalf("Parse() = %q, %q; want task-otter, Taskotter", owner, name)
	}
}

// TestParseRejectsInvalidCoordinates verifies malformed repository coordinates return errors.
func TestParseRejectsInvalidCoordinates(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"owner",
		"owner/",
		"/repo",
		"owner/repo/extra",
	}

	for i := range cases {
		assertParseInvalid(t, cases[i])
	}
}

func assertParseInvalid(t *testing.T, full string) {
	t.Helper()

	owner, name, err := repo.Parse(full)
	iox.Discard2(owner, name)

	if err == nil {
		t.Fatalf("Parse(%q) expected error", full)
	}
}
