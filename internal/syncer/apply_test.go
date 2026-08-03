// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/resolver"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
)

var errSimulatedPromoteFailure = errors.New("simulated promote failure")

func preparePlan(t *testing.T, _ string, cfg *config.Config) (syncer.SyncInput, *syncer.Plan) {
	t.Helper()

	snap := fixtureStore(t)

	resolutions, depSources := resolveModulesForTest(t, cfg, snap)

	syncInput, err := app.PrepareSyncInput(cfg, snap, resolutions, depSources)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := syncer.BuildPlan(&syncInput)
	if err != nil {
		t.Fatal(err)
	}

	return syncInput, plan
}

func resolveModulesForTest(
	t *testing.T,
	cfg *config.Config,
	snap *store.Snapshot,
) ([]resolver.Resolution, []string) {
	t.Helper()

	resolutions, err := resolver.ResolveAll(
		cfg.Tasks,
		snap.Catalog,
		cfg.NodePackageManager,
		cfg.NodeVersionManager,
	)
	if err != nil {
		t.Fatal(err)
	}

	depSources, err := dependencySources(t, sourceModulesOfResolutions(resolutions), snap)
	if err != nil {
		t.Fatal(err)
	}

	return resolutions, depSources
}

func sourceModulesOfResolutions(resolutions []resolver.Resolution) []string {
	sources := make([]string, consts.IndexZero, len(resolutions))

	for _, res := range resolutions {
		sources = append(sources, res.SourceModule)
	}

	return sources
}

func writeFileEntry(path string, entry syncer.FileEntry) error {
	err := os.MkdirAll(filepath.Dir(path), consts.FilePerm755)
	if err != nil {
		return fmt.Errorf("create directory for %q: %w", path, err)
	}

	err = os.WriteFile(path, entry.Data, entry.Mode)
	if err != nil {
		return fmt.Errorf("write file %q: %w", path, err)
	}

	return nil
}

func mutateGoWithDocs(cfg *config.Config) {
	cfg.Tasks = []string{consts.Go}
	cfg.IncludesDoc = true
}

func mutateGoWithDocsNoSyncRoot(cfg *config.Config) {
	mutateGoWithDocs(cfg)

	cfg.SyncRoot = false
}

func setupPlan(
	t *testing.T,
	workspace string,
	mutate func(*config.Config),
) (*config.Config, syncer.SyncInput, *syncer.Plan) {
	t.Helper()

	writeRootTaskfile(t, workspace)

	cfg := testConfig(workspace, mutate)
	syncInput, plan := preparePlan(t, workspace, cfg)

	return cfg, syncInput, plan
}

func setupPlanWithRootContent(
	t *testing.T,
	workspace, rootContent string,
	mutate func(*config.Config),
) (syncer.SyncInput, *syncer.Plan) {
	t.Helper()

	err := os.WriteFile(
		filepath.Join(workspace, testTaskfileName),
		[]byte(rootContent),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(workspace, mutate)
	syncInput, plan := preparePlan(t, workspace, cfg)

	return syncInput, plan
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
}

// TestApplyPlanWritesFiles verifies applying a plan writes module and metadata files.
func TestApplyPlanWritesFiles(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cfg, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join(workspace, config.DefaultTargetFolder, testGoTaskfilePath))
	assertFileExists(t, filepath.Join(workspace, cfg.MetadataPath()))
}

func writeLegacyMetadataFixture(t *testing.T, workspace string) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(workspace, testLegacyMetaDir), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(workspace, config.LegacyMetadataPath),
		[]byte("target_folder: taskfiles\nlock_file: taskfiles/.taskotter-lock.yml\n"),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func assertMetadataMigrated(t *testing.T, workspace string, cfg *config.Config) {
	t.Helper()

	assertFileExists(t, filepath.Join(workspace, cfg.MetadataPath()))

	_, err := os.Stat(filepath.Join(workspace, config.LegacyMetadataPath))

	if !os.IsNotExist(err) {
		t.Fatalf("legacy metadata should be removed, stat returned: %v", err)
	}
}

// TestApplyPlanMigratesLegacyMetadataPath verifies legacy metadata is migrated and the old file removed.
func TestApplyPlanMigratesLegacyMetadataPath(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeLegacyMetadataFixture(t, workspace)

	cfg, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertMetadataMigrated(t, workspace, cfg)
}

func assertRootTaskfileNotDiffed(t *testing.T, plan *syncer.Plan) {
	t.Helper()

	if containsRootTaskfile(plan.Added) || containsRootTaskfile(plan.Updated) {
		t.Fatalf(
			"root Taskfile.yml should not be in the diff: added=%v updated=%v",
			plan.Added,
			plan.Updated,
		)
	}

	if containsRootTaskfile(plan.StagePaths) {
		t.Fatalf("root Taskfile.yml should not be staged: %v", plan.StagePaths)
	}

	if plan.Lock.Configuration.SyncRoot {
		t.Fatal("lock SyncRoot = true, want false")
	}
}

func assertRootTaskfileSkipped(t *testing.T, workspace, rootPath, rootContent string) {
	t.Helper()

	data, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != rootContent {
		t.Fatalf("root Taskfile.yml changed to %q, want %q", data, rootContent)
	}

	assertFileExists(t, filepath.Join(workspace, config.DefaultTargetFolder, "go", "Taskfile.yml"))
}

// TestApplyPlanSkipsRootTaskfileWhenDisabled verifies the root Taskfile is left untouched when sync-root is off.
func TestApplyPlanSkipsRootTaskfileWhenDisabled(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	rootPath := filepath.Join(workspace, testTaskfileName)
	rootContent := "this is intentionally not valid Taskfile YAML: ["

	syncInput, plan := setupPlanWithRootContent(
		t,
		workspace,
		rootContent,
		mutateGoWithDocsNoSyncRoot,
	)

	assertRootTaskfileNotDiffed(t, plan)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertRootTaskfileSkipped(t, workspace, rootPath, rootContent)
}

func makeSetupScriptExecutableForTest(t *testing.T) {
	t.Helper()

	setupPath := filepath.Join(
		consts.PathParent,
		consts.PathParent,
		dirTests,
		dirFixtures,
		dirStore,
		"taskfiles",
		consts.Go,
		"setup.sh",
	)

	info, err := os.Stat(setupPath)
	if err != nil {
		t.Fatal(err)
	}

	origMode := info.Mode().Perm()

	err = os.Chmod(setupPath, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(
		func() { _ = os.Chmod(setupPath, origMode) },
	)
}

func assertExecutableBit(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm()&consts.FilePerm111 == consts.IndexZero {
		t.Fatalf("expected executable bit, got %o", info.Mode().Perm())
	}
}

// TestApplyPlanPreservesExecutableMode verifies executable bits are preserved when writing files.
func TestApplyPlanPreservesExecutableMode(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	makeSetupScriptExecutableForTest(t)

	_, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertExecutableBit(t, filepath.Join(workspace, config.DefaultTargetFolder, "go/setup.sh"))
}

func writeObsoleteManagedFile(t *testing.T, workspace string) string {
	t.Helper()

	obsolete := filepath.Join(workspace, config.DefaultTargetFolder, "go/obsolete.txt")

	err := os.MkdirAll(filepath.Dir(obsolete), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(obsolete, []byte("old"), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	writeMinimalLock(t, workspace, config.DefaultTargetFolder, []syncer.ManagedFile{
		{
			SourceModule:      consts.Empty,
			DestinationModule: consts.Go,
			SourcePath:        consts.Empty,
			Path:              config.DefaultTargetFolder + "/go/obsolete.txt",
			SHA256:            consts.Empty,
		},
	})

	return obsolete
}

func failFirstPromoteHook(promoted *int) func(string, syncer.FileEntry) error {
	return func(path string, entry syncer.FileEntry) error {
		if strings.Contains(path, filepath.Join(testLegacyMetaDir, testStagingDir)) {
			return writeFileEntry(path, entry)
		}

		*promoted++

		if *promoted == consts.IndexOne {
			return errSimulatedPromoteFailure
		}

		return writeFileEntry(path, entry)
	}
}

func assertPromoteFails(t *testing.T, plan *syncer.Plan, syncInput syncer.SyncInput) func() {
	return func() {
		err := syncer.ApplyPlan(plan, &syncInput)
		if err == nil {
			t.Fatal("expected promote failure")
		}
	}
}

// TestApplyPlanPromoteBeforeDelete verifies obsolete files remain if a promote step fails before delete.
func TestApplyPlanPromoteBeforeDelete(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	obsolete := writeObsoleteManagedFile(t, workspace)

	_, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	var promoted int

	withCopyFileHook(
		t,
		plan,
		failFirstPromoteHook(&promoted),
		assertPromoteFails(t, plan, syncInput),
	)

	_, err := os.Stat(obsolete)
	if err != nil {
		t.Fatal("obsolete file should remain when promote fails before delete")
	}
}

func recordingCopyHook(
	workspace, stagingMarker string,
	order *[]string,
) func(string, syncer.FileEntry) error {
	return func(path string, entry syncer.FileEntry) error {
		if strings.Contains(path, stagingMarker) {
			return writeFileEntry(path, entry)
		}

		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}

		*order = append(*order, filepath.ToSlash(rel))

		return writeFileEntry(path, entry)
	}
}

func applyPlanOK(t *testing.T, plan *syncer.Plan, syncInput syncer.SyncInput) func() {
	return func() {
		err := syncer.ApplyPlan(plan, &syncInput)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func recordWriteOrder(
	t *testing.T,
	workspace string,
	plan *syncer.Plan,
	syncInput syncer.SyncInput,
) []string {
	t.Helper()

	var order []string

	stagingMarker := filepath.Join(config.DefaultTargetFolder, testLegacyMetaDir, testStagingDir)

	withCopyFileHook(
		t,
		plan,
		recordingCopyHook(workspace, stagingMarker, &order),
		applyPlanOK(t, plan, syncInput),
	)

	return order
}

// TestApplyPlanWriteOrder verifies modules are written before the lock and before metadata.
func TestApplyPlanWriteOrder(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	order := recordWriteOrder(t, workspace, plan, syncInput)

	lockIdx := indexOfSuffix(order, testLockFileName)
	metaIdx := indexOfSuffix(order, testMetadataFileName)
	moduleIdx := indexOfContains(order, config.DefaultTargetFolder+"/go/")

	if lockIdx < moduleIdx || metaIdx < lockIdx {
		t.Fatalf("expected modules before lock before metadata, got %v", order)
	}
}

func indexOfSuffix(paths []string, suffix string) int {
	for i, p := range paths {
		if strings.HasSuffix(p, suffix) {
			return i
		}
	}

	return notFoundIndex
}

func indexOfContains(paths []string, part string) int {
	for i, p := range paths {
		if strings.Contains(p, part) {
			return i
		}
	}

	return notFoundIndex
}
