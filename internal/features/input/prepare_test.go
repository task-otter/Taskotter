// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package input

import (
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
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
)

// TestPrepareSyncInputReportsDestinationCollision verifies colliding destinations fail.
func TestPrepareSyncInputReportsDestinationCollision(t *testing.T) {
	t.Parallel()

	input, err := SyncInput(&SyncInputArgs{
		Cfg: &config.Config{
			TargetFolder: config.DefaultTargetFolder,
		},
		Resolutions: []resolvesvc.Resolution{
			{LogicalTask: pathA, SourceModule: eslintNodePNPM},
			{LogicalTask: pathB, SourceModule: eslintBun},
		},
		DepSources: nil,
	})

	iox.Discard(input)

	if err == nil {
		t.Fatal("expected destination collision")
	}
}

// TestPrepareSyncInputWrapsAssembleFailure verifies PrepareSyncInput wraps collisions.
func TestPrepareSyncInputWrapsAssembleFailure(t *testing.T) {
	t.Parallel()

	input, err := SyncInput(&SyncInputArgs{
		Snapshot:    nil,
		TaskfileOps: nil,
		DepSources:  nil,
		Cfg:         &config.Config{TargetFolder: config.DefaultTargetFolder},
		Resolutions: []resolvesvc.Resolution{
			{LogicalTask: pathA, SourceModule: eslintNodePNPM},
			{LogicalTask: pathB, SourceModule: eslintBun},
		},
	})

	iox.Discard(input)

	if err == nil {
		t.Fatal("expected wrapped destination collision")
	}
}

// TestCollectRequestedSourcesPreservesOrder verifies the expected behavior.
func TestCollectRequestedSourcesPreservesOrder(t *testing.T) {
	t.Parallel()

	got := collectRequestedSources([]resolvesvc.Resolution{
		{SourceModule: consts.Go},
		{SourceModule: "eslint"},
	})

	if len(got) != consts.IndexTwo || got[consts.IndexZero] != consts.Go {
		t.Fatalf("sources = %v", got)
	}
}

// TestPrepareSyncInputBuildsRecords verifies the behavior covered by this test.
//
//nolint:funlen,maintidx // The explicit requested/dependency record assertions document the contract.
func TestPrepareSyncInputBuildsRecords(t *testing.T) {
	t.Parallel()

	input, err := SyncInput(&SyncInputArgs{
		Cfg: &config.Config{TargetFolder: config.DefaultTargetFolder},
		Resolutions: []resolvesvc.Resolution{
			{LogicalTask: pathA, SourceModule: consts.Go},
		},
		DepSources: []string{"shellcheck"},
	})
	if err != nil {
		t.Fatalf("SyncInput() error = %v", err)
	}

	requested := input.Requested[pathA]
	assertPrepareRecord(
		t,
		&prepareRecordAssertion{requested.SourceModule, consts.Go, "requested source"},
	)
	assertPrepareRecord(
		t,
		&prepareRecordAssertion{requested.Path, "taskfiles/go", "requested path"},
	)
	assertPrepareRecord(
		t,
		&prepareRecordAssertion{input.DestByTask[pathA], consts.Go, "dest by task"},
	)
	assertPrepareRecord(t, &prepareRecordAssertion{
		input.Dependencies[consts.IndexZero].Path, "taskfiles/shellcheck", "dependency path",
	})
}

func assertPrepareRecord(t *testing.T, assertion *prepareRecordAssertion) {
	t.Helper()

	if assertion.got != assertion.want {
		t.Fatalf("%s = %q, want %q", assertion.label, assertion.got, assertion.want)
	}
}
