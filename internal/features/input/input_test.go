// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package input

import (
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
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

	requested := requireRequested(t, &input, "lint")

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

func requireRequested(t *testing.T, input *Input, task string) lockmodel.ModuleRecord {
	t.Helper()

	requested, ok := input.Requested[task]

	if !ok {
		t.Fatalf("Build() did not create requested %s record", task)
	}

	return requested
}
