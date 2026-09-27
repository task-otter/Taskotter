// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package input

import (
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	prepareRecordAssertion struct {
		got, want string
		label     string
	}
)

const (
	pathA          = "lint"
	pathB          = "format"
	eslintNodePNPM = "eslint/node/pnpm"
	eslintBun      = "eslint/bun"
	eslint         = "eslint"
)

// TestPrepareSyncInputReportsDestinationCollision verifies colliding destinations fail.
func TestPrepareSyncInputReportsDestinationCollision(t *testing.T) {
	t.Parallel()
	assertDestinationCollision(t)
}

// TestCollectRequestedSourcesPreservesOrder verifies the expected behavior.
func TestCollectRequestedSourcesPreservesOrder(t *testing.T) {
	t.Parallel()

	got := collectRequestedSources([]resolvesvc.Resolution{
		{SourceModule: consts.Go},
		{SourceModule: eslint},
	})

	if len(got) != consts.IndexTwo || got[consts.IndexZero] != consts.Go {
		t.Fatalf("sources = %v", got)
	}
}

// TestPrepareSyncInputBuildsRecords verifies the behavior covered by this test.
func TestPrepareSyncInputBuildsRecords(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{TargetFolder: config.DefaultTargetFolder}
	resolutions := []resolvesvc.Resolution{
		{LogicalTask: pathA, SourceModule: consts.Go},
	}

	input, err := SyncInput(&SyncInputArgs{
		Cfg: cfg, Resolutions: resolutions, DepSources: []string{"shellcheck"},
	})
	if err != nil {
		t.Fatalf("SyncInput() error = %v", err)
	}

	assertPrepareRecords(t, &input)
}

func assertDestinationCollision(t *testing.T) {
	t.Helper()

	input, err := SyncInput(&SyncInputArgs{
		Cfg: &config.Config{TargetFolder: config.DefaultTargetFolder},
		Resolutions: []resolvesvc.Resolution{
			{LogicalTask: pathA, SourceModule: eslintNodePNPM},
			{LogicalTask: pathB, SourceModule: eslintBun},
		},
	})

	if input.Config != nil {
		t.Fatal("collision unexpectedly produced input")
	}

	if err == nil {
		t.Fatal("expected destination collision")
	}
}

func assertPrepareRecords(t *testing.T, input *Input) {
	t.Helper()

	requested := input.Requested[pathA]

	assertions := []prepareRecordAssertion{
		{requested.SourceModule, consts.Go, "requested source"},
		{requested.Path, "taskfiles/go", "requested path"},
		{input.DestByTask[pathA], consts.Go, "dest by task"},
		{input.Dependencies[consts.IndexZero].Path, "taskfiles/shellcheck", "dependency path"},
	}

	for idx := range assertions {
		assertPrepareRecord(t, &assertions[idx])
	}
}

func assertPrepareRecord(t *testing.T, assertion *prepareRecordAssertion) {
	t.Helper()

	if assertion.got != assertion.want {
		t.Fatalf("%s = %q, want %q", assertion.label, assertion.got, assertion.want)
	}
}
