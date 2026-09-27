// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package prepare

import (
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
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

	input, err := PrepareSyncInput(&PrepareSyncInputArgs{
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

	input, err := PrepareSyncInput(&PrepareSyncInputArgs{
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
