// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"path/filepath"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	buildPlanFromInput struct {
		t           *testing.T
		cfg         *config.Config
		snap        *storedomain.Snapshot
		resolutions []resolvesvc.Resolution
		depSources  []string
	}
)

func buildPlanFrom(input *buildPlanFromInput) *syncdomain.Plan {
	input.t.Helper()

	syncInput, err := syncsvc.PrepareSyncInput(&syncsvc.PrepareSyncInputArgs{
		Cfg: input.cfg, Snapshot: input.snap, TaskfileOps: synctaskfile.Ops{},
		Resolutions: input.resolutions, DepSources: input.depSources,
	})
	if err != nil {
		input.t.Fatal(err)
	}

	plan, err := syncsvc.BuildPlan(&syncInput)
	if err != nil {
		input.t.Fatal(err)
	}

	return plan
}

// writeCorruptLock seeds workspace with metadata pointing at a lock file whose
// contents are not valid YAML.
func writeCorruptLock(t *testing.T, workspace string) {
	t.Helper()

	lockPath := filepath.Join(workspace, config.DefaultTargetFolder, testLockFileName)
	writeFileWithDir(t, lockPath, []byte(testInvalidYAML))

	meta := []byte(`target_folder: taskfiles
lock_file: taskfiles/.taskotter-lock.yml
configuration_hash: abc
`)

	writeTaskotterMetadata(&metadataWriteInput{
		t: t, workspace: workspace, targetFolder: config.DefaultTargetFolder, meta: meta,
	})
}

// TestBuildPlanCorruptLockFails verifies a corrupt lock file fails plan building.
func TestBuildPlanCorruptLockFails(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	writeCorruptLock(t, workspace)

	cfg := testConfig(workspace, mutateGoWithDocs)
	snap := fixtureStore(t)

	si := buildSingleSyncIn(&moduleTestInput{t: t, cfg: cfg, snap: snap})

	expectBuildPlanError(t, &si, errExpectedCorruptLock)
}

// TestBuildPlanCorruptMetadataFails verifies corrupt metadata fails plan building.
func TestBuildPlanCorruptMetadataFails(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	writeTaskotterMetadata(&metadataWriteInput{
		t:            t,
		workspace:    workspace,
		targetFolder: config.DefaultTargetFolder,
		meta:         []byte(testInvalidYAML),
	})

	cfg := testConfig(workspace, mutateGoWithDocs)
	snap := fixtureStore(t)

	si := buildSingleSyncIn(&moduleTestInput{t: t, cfg: cfg, snap: snap})

	expectBuildPlanError(t, &si, errExpectedCorruptMetadata)
}

func mutateGoWithDocsHashA(cfg *config.Config) {
	mutateGoWithDocs(cfg)

	cfg.ConfigurationHash = "hash-a"
}

// TestMetadataOnlyChangeMarksChanged verifies a configuration hash-only change marks the plan changed.
func TestMetadataOnlyChangeMarksChanged(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocsHashA},
	)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	cfg.ConfigurationHash = "hash-b"

	plan2 := replan(t, workspace, cfg)

	if !plan2.Changed {
		t.Fatal("expected metadata-only configuration hash change to mark plan changed")
	}
}

func rebuildPlanWithDifferentSHA(t *testing.T, workspace string) *syncdomain.Plan {
	t.Helper()

	cfg := testConfig(workspace, mutateGoWithDocs)
	snap := fixtureStore(t)

	snap.Ref.ResolvedCommit = "different-sha-only"

	resolutions, depSources := resolveModsForTest(&moduleTestInput{t: t, cfg: cfg, snap: snap})

	return buildPlanFrom(&buildPlanFromInput{
		t: t, cfg: cfg, snap: snap, resolutions: resolutions, depSources: depSources,
	})
}

func applyPlanAndRebuildSHA(t *testing.T, workspace string) *syncdomain.Plan {
	t.Helper()

	syncInput, plan := setupPlanInput(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	return rebuildPlanWithDifferentSHA(t, workspace)
}

// TestSHAOnlyLockChangeNotChanged verifies a resolved-commit-only difference does not mark files changed.
func TestSHAOnlyLockChangeNotChanged(t *testing.T) {
	t.Parallel()

	plan2 := applyPlanAndRebuildSHA(t, t.TempDir())

	if plan2.Changed {
		t.Fatalf(
			"expected no file changes when only resolved commit differs: added=%v updated=%v removed=%v",
			plan2.Added,
			plan2.Updated,
			plan2.Removed,
		)
	}
}

// TestConfigurationChangeMarksUpdated verifies a config field change marks the plan updated.
func TestConfigurationChangeMarksUpdated(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	cfg.IncludesDoc = false

	plan2 := replan(t, workspace, cfg)

	if !plan2.Changed {
		t.Fatal("expected changes when includes-doc toggles")
	}
}
