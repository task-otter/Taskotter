// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package input

import (
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	testSnapshot struct{}
)

const (
	testDefaultBranch = "main"
)

func (testSnapshot) DefaultBranch() string   { return testDefaultBranch }
func (testSnapshot) ModuleDir(string) string { return "" }
func (testSnapshot) ResolvedCommit() string  { return "deadbeef" }
func (testSnapshot) SourceRef() string       { return testDefaultBranch }
func (testSnapshot) WorkspaceRoot() string   { return "" }

// TestBuildMapsResolvedModules verifies resolved modules are normalized for planning.
func TestBuildMapsResolvedModules(t *testing.T) {
	t.Parallel()

	input, err := Build(testBuildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertResolvedModule(t, &input)
}

// TestBuildReportsInputAssemblyFailure verifies Build wraps input errors.
func TestBuildReportsInputAssemblyFailure(t *testing.T) {
	t.Parallel()

	input, err := Build(&SyncInputArgs{
		Cfg:      &config.Config{TargetFolder: config.DefaultTargetFolder},
		Snapshot: testSnapshot{},
		Resolutions: []resolvesvc.Resolution{
			{LogicalTask: pathA, SourceModule: eslintNodePNPM},
			{LogicalTask: pathB, SourceModule: eslintBun},
		},
	})

	if input.Config != nil {
		t.Fatal("Build() returned input after destination collision")
	}

	if err == nil {
		t.Fatal("Build() error = nil, want destination collision")
	}
}

func testBuildInput() *SyncInputArgs {
	return &SyncInputArgs{
		Cfg:         &config.Config{TargetFolder: "taskfiles", Tasks: []string{pathA}},
		Snapshot:    testSnapshot{},
		Resolutions: []resolvesvc.Resolution{{LogicalTask: pathA, SourceModule: eslintNodePNPM}},
	}
}

func assertResolvedModule(t *testing.T, input *Input) {
	t.Helper()

	requested := requireRequested(t, input, pathA)

	if requested.SourceModule != eslintNodePNPM || requested.DestinationModule != eslint {
		t.Fatalf("requested = %#v", requested)
	}

	if input.DestByTask[pathA] != eslint {
		t.Fatalf(
			"destination by task = %q, want %s",
			input.DestByTask[pathA],
			eslint,
		)
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
