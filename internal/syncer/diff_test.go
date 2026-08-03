// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/resolver"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
	yaml "go.yaml.in/yaml/v3"
)

func writeFileWithDir(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, data, mode)
	if err != nil {
		t.Fatal(err)
	}
}

func writeTaskotterMetadata(t *testing.T, workspace, targetFolder string, meta []byte) {
	t.Helper()

	path := filepath.Join(workspace, targetFolder, testMetadataRelPath)
	writeFileWithDir(t, path, meta, consts.FilePerm644)
}

// writeCorruptLock seeds workspace with metadata pointing at a lock file whose
// contents are not valid YAML.
func writeCorruptLock(t *testing.T, workspace string) {
	t.Helper()

	lockPath := filepath.Join(workspace, config.DefaultTargetFolder, testLockFileName)
	writeFileWithDir(t, lockPath, []byte(testInvalidYAML), consts.FilePerm644)

	meta := []byte(`target_folder: taskfiles
lock_file: taskfiles/.taskotter-lock.yml
configuration_hash: abc
`)

	writeTaskotterMetadata(t, workspace, config.DefaultTargetFolder, meta)
}

func buildSingleModuleSyncInput(
	t *testing.T,
	cfg *config.Config,
	snap *store.Snapshot,
) syncer.SyncInput {
	t.Helper()

	res, err := resolver.Resolve(consts.Go, snap.Catalog, consts.Empty, consts.Empty)
	if err != nil {
		t.Fatal(err)
	}

	return syncer.SyncInput{
		Config:   cfg,
		Snapshot: snap,
		Requested: map[string]syncer.ModuleRecord{
			consts.Go: {
				SourceModule:      res.SourceModule,
				DestinationModule: consts.Go,
				Path:              config.DefaultTargetFolder + "/go",
			},
		},
		Dependencies: nil,
		SourceToDest: map[string]string{res.SourceModule: consts.Go},
		DestByTask:   map[string]string{consts.Go: consts.Go},
	}
}

func buildPlanFrom(
	t *testing.T,
	cfg *config.Config,
	snap *store.Snapshot,
	resolutions []resolver.Resolution,
	depSources []string,
) *syncer.Plan {
	t.Helper()

	syncInput, err := app.PrepareSyncInput(cfg, snap, resolutions, depSources)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := syncer.BuildPlan(&syncInput)
	if err != nil {
		t.Fatal(err)
	}

	return plan
}

// TestBuildPlanCorruptLockFails verifies a corrupt lock file fails plan building.
func TestBuildPlanCorruptLockFails(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	writeCorruptLock(t, workspace)

	cfg := testConfig(workspace, mutateGoWithDocs)
	snap := fixtureStore(t)

	si := buildSingleModuleSyncInput(t, cfg, snap)

	_, err := syncer.BuildPlan(&si)
	if err == nil {
		t.Fatal(errExpectedCorruptLock)
	}
}

// TestBuildPlanCorruptMetadataFails verifies corrupt metadata fails plan building.
func TestBuildPlanCorruptMetadataFails(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	writeTaskotterMetadata(t, workspace, config.DefaultTargetFolder, []byte(testInvalidYAML))

	cfg := testConfig(workspace, mutateGoWithDocs)
	snap := fixtureStore(t)

	si := buildSingleModuleSyncInput(t, cfg, snap)

	_, err := syncer.BuildPlan(&si)
	if err == nil {
		t.Fatal(errExpectedCorruptMetadata)
	}
}

func mutateGoWithDocsHashA(cfg *config.Config) {
	mutateGoWithDocs(cfg)

	cfg.ConfigurationHash = "hash-a"
}

// TestMetadataOnlyChangeMarksChanged verifies a configuration hash-only change marks the plan changed.
func TestMetadataOnlyChangeMarksChanged(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cfg, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocsHashA)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	cfg.ConfigurationHash = "hash-b"

	_, plan2 := preparePlan(t, workspace, cfg)

	if !plan2.Changed {
		t.Fatal("expected metadata-only configuration hash change to mark plan changed")
	}
}

func rebuildPlanWithDifferentSHA(t *testing.T, workspace string) *syncer.Plan {
	t.Helper()

	cfg := testConfig(workspace, mutateGoWithDocs)
	snap := fixtureStore(t)

	snap.Ref.ResolvedCommit = "different-sha-only"

	resolutions, depSources := resolveModulesForTest(t, cfg, snap)

	return buildPlanFrom(t, cfg, snap, resolutions, depSources)
}

// TestSHAOnlyLockChangeNotChanged verifies a resolved-commit-only difference does not mark files changed.
func TestSHAOnlyLockChangeNotChanged(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	plan2 := rebuildPlanWithDifferentSHA(t, workspace)

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
	cfg, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	cfg.IncludesDoc = false

	_, plan2 := preparePlan(t, workspace, cfg)

	if !plan2.Changed {
		t.Fatal("expected changes when includes-doc toggles")
	}
}

func writeLockFile(t *testing.T, workspace, targetFolder string, files []syncer.ManagedFile) {
	t.Helper()

	var lock syncer.LockFile

	lock.Configuration.TargetFolder = targetFolder
	lock.ManagedFiles = files

	data, err := yaml.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(workspace, targetFolder, testLockFileName)
	writeFileWithDir(t, lockPath, data, consts.FilePerm644)
}

func writeMinimalLock(t *testing.T, workspace, targetFolder string, files []syncer.ManagedFile) {
	t.Helper()

	writeLockFile(t, workspace, targetFolder, files)

	meta := []byte(
		"target_folder: " + targetFolder +
			"\nlock_file: " + targetFolder + "/.taskotter-lock.yml\nconfiguration_hash: x\n",
	)

	writeTaskotterMetadata(t, workspace, targetFolder, meta)
}
