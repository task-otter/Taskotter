package repo_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/repo"
)

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

func TestParseRejectsInvalidCoordinates(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"owner",
		"owner/",
		"/repo",
		"owner/repo/extra",
	}

	for _, input := range cases {
		_, _, err := repo.Parse(input)
		if err == nil {
			t.Fatalf("Parse(%q) expected error", input)
		}
	}
}
