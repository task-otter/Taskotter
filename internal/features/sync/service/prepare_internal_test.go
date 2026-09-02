// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:exhaustruct_v5 // test fixtures only set fields exercised by the unit
package service

import (
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// TestCollectRequestedSourcesPreservesOrder verifies resolution sources are collected.
func TestCollectRequestedSourcesPreservesOrder(t *testing.T) {
	t.Parallel()

	got := collectRequestedSources([]resolvesvc.Resolution{
		{SourceModule: consts.Go},
		{SourceModule: "eslint"},
	})

	if len(got) != consts.IndexTwo || got[consts.IndexZero] != consts.Go {
		t.Fatalf(gotFmt, got)
	}
}

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
	assertFails(t, err)
}

// TestPrepareSyncInputWrapsAssembleFailure verifies PrepareSyncInput wraps collisions.
func TestPrepareSyncInputWrapsAssembleFailure(t *testing.T) {
	t.Parallel()

	input, err := PrepareSyncInput(&PrepareSyncInputArgs{
		Snapshot:    nil,
		TaskfileOps: nil,
		DepSources:  nil,
		Cfg:         emptyConfigPtr(),
		Resolutions: []resolvesvc.Resolution{
			{LogicalTask: pathA, SourceModule: eslintNodePNPM},
			{LogicalTask: pathB, SourceModule: eslintBun},
		},
	})

	iox.Discard(input)
	assertFails(t, err)
}
