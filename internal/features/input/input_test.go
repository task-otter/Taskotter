// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package input

import (
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type testSnapshot struct{}

func (testSnapshot) ModuleDir(string) string { return "" }
func (testSnapshot) WorkspaceRoot() string   { return "" }
func (testSnapshot) SourceRef() string       { return "main" }
func (testSnapshot) ResolvedCommit() string  { return "deadbeef" }
func (testSnapshot) DefaultBranch() string   { return "main" }

func TestBuildMapsResolvedModules(t *testing.T) {
	t.Parallel()

	input, err := Build(&BuildInput{
		Cfg: &config.Config{
			TargetFolder: "taskfiles",
			Tasks:        []string{"lint"},
		},
		Snapshot: testSnapshot{},
		Resolutions: []resolvesvc.Resolution{{
			LogicalTask:  "lint",
			SourceModule: "eslint/node/pnpm",
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	requested, ok := input.Requested["lint"]

	if !ok {
		t.Fatal("Build() did not create requested lint record")
	}

	if requested.SourceModule != "eslint/node/pnpm" {
		t.Fatalf("source module = %q", requested.SourceModule)
	}

	if requested.DestinationModule != "eslint" {
		t.Fatalf("destination module = %q, want eslint", requested.DestinationModule)
	}

	if input.DestByTask["lint"] != "eslint" {
		t.Fatalf("destination by task = %q, want eslint", input.DestByTask["lint"])
	}
}
