// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	"github.com/task-otter/Taskotter/internal/testsupport"
)

func lockSeams(t *testing.T) {
	t.Helper()
	t.Cleanup(testsupport.Lock())
}

func buildPlanFromSyncInput(t *testing.T, syncInput *syncdomain.SyncInput) *syncdomain.Plan {
	t.Helper()

	plan, err := syncsvc.BuildPlan(syncInput)
	if err != nil {
		t.Fatal(err)
	}

	return plan
}

func preparePlan(
	t *testing.T,
	_ string,
	cfg *config.Config,
) (syncInput syncdomain.SyncInput, plan *syncdomain.Plan) {
	t.Helper()

	syncInput = prepareSyncInputForTest(t, cfg)
	plan = buildPlanFromSyncInput(t, &syncInput)

	return syncInput, plan
}

func prepareSyncInputForTest(t *testing.T, cfg *config.Config) syncdomain.SyncInput {
	t.Helper()

	snap := fixtureStore(t)
	resolutions, depSources := resolveModsForTest(&moduleTestInput{t: t, cfg: cfg, snap: snap})

	syncInput, err := syncsvc.PrepareSyncInput(&syncsvc.PrepareSyncInputArgs{
		Cfg: cfg, Snapshot: syncsvc.SnapshotPort(snap), TaskfileOps: synctaskfile.NewOps(),
		Resolutions: resolutions, DepSources: depSources,
	})
	if err != nil {
		t.Fatal(err)
	}

	return syncInput
}

func resolveModsForTest(input *moduleTestInput) (res []resolvesvc.Resolution, dep []string) {
	input.t.Helper()

	res = resolveAllModules(input)

	dep, err := dependencySources(input.t, sourceModulesOfResolutions(res), input.snap)
	if err != nil {
		input.t.Fatal(err)
	}

	return res, dep
}

func sourceModulesOfResolutions(resolutions []resolvesvc.Resolution) []string {
	sources := make([]string, consts.IndexZero, len(resolutions))

	for i := range resolutions {
		sources = append(sources, resolutions[i].SourceModule)
	}

	return sources
}

func writeFileEntry(path string, entry *syncdomain.FileEntry) error {
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

func setupPlanWithRootContent(
	args *setupPlanRootArgs,
) (syncInput syncdomain.SyncInput, plan *syncdomain.Plan) {
	args.t.Helper()

	writeFileWithDir(
		args.t,
		filepath.Join(args.workspace, testTaskfileName),
		[]byte(args.rootContent),
	)

	cfg := testConfig(args.workspace, args.mutate)

	return preparePlan(args.t, args.workspace, cfg)
}

func writeLegacyMetadataFixture(t *testing.T, workspace string) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(workspace, testLegacyMetaDir), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	writeFileWithDir(
		t,
		filepath.Join(workspace, config.LegacyMetadataPath),
		[]byte("target_folder: taskfiles\nlock_file: taskfiles/.taskotter-lock.yml\n"),
	)
}

func assertMetadataMigrated(t *testing.T, workspace string, cfg *config.Config) {
	t.Helper()

	assertFileExists(t, filepath.Join(workspace, config.MetadataPath(cfg)))

	stat, err := os.Stat(filepath.Join(workspace, config.LegacyMetadataPath))
	iox.Discard(stat)

	if !os.IsNotExist(err) {
		t.Fatalf("legacy metadata should be removed, stat returned: %v", err)
	}
}

// TestApplyPlanWritesFiles verifies applying a plan writes module and metadata files.
func TestApplyPlanWritesFiles(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join(workspace, config.DefaultTargetFolder, testGoTaskfilePath))
	assertFileExists(t, filepath.Join(workspace, config.MetadataPath(cfg)))
}

// TestApplyPlanMigratesLegacyMetadataPath verifies legacy metadata is migrated and the old file removed.
func TestApplyPlanMigratesLegacyMetadataPath(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	writeLegacyMetadataFixture(t, workspace)

	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertMetadataMigrated(t, workspace, cfg)
}

func assertRootTaskfileNotDiffed(t *testing.T, plan *syncdomain.Plan) {
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

func assertRootTaskfileSkipped(input *assertRootSkippedInput) {
	input.t.Helper()

	data, err := os.ReadFile(input.rootPath)
	if err != nil {
		input.t.Fatal(err)
	}

	if string(data) != input.rootContent {
		input.t.Fatalf("root Taskfile.yml changed to %q, want %q", data, input.rootContent)
	}

	assertFileExists(
		input.t,
		filepath.Join(input.workspace, config.DefaultTargetFolder, consts.Go, testTaskfileName),
	)
}

func runApplyPlanSkipsRootTest(t *testing.T, workspace, rootContent string) {
	t.Helper()

	rootPath := filepath.Join(workspace, testTaskfileName)
	syncInput, plan := setupPlanWithRootContent(&setupPlanRootArgs{
		t: t, workspace: workspace, rootContent: rootContent, mutate: mutateGoWithDocsNoSyncRoot,
	})

	assertRootTaskfileNotDiffed(t, plan)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertRootTaskfileSkipped(&assertRootSkippedInput{
		t: t, workspace: workspace, rootPath: rootPath, rootContent: rootContent,
	})
}

// TestApplyPlanSkipsRootTaskfileWhenDisabled verifies the root Taskfile is left untouched when sync-root is off.
func TestApplyPlanSkipsRootTaskfileWhenDisabled(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	runApplyPlanSkipsRootTest(t, t.TempDir(), "this is intentionally not valid Taskfile YAML: [")
}

func storeSetupScriptPath() string {
	return filepath.Join(
		consts.PathParent,
		consts.PathParent,
		consts.PathParent,
		consts.PathParent,
		dirTests,
		dirFixtures,
		dirStore,
		"taskfiles",
		consts.Go,
		testRelGoSetupSh,
	)
}

func restoreSetupScriptMode(t *testing.T, setupPath string, origMode os.FileMode) {
	t.Helper()

	err := os.Chmod(setupPath, origMode)
	if err != nil {
		t.Fatal(err)
	}
}

func makeSetupScriptExecutableForTest(t *testing.T) {
	t.Helper()

	setupPath := storeSetupScriptPath()

	info, err := os.Stat(setupPath)
	if err != nil {
		t.Fatal(err)
	}

	origMode := info.Mode().Perm()

	err = os.Chmod(setupPath, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { restoreSetupScriptMode(t, setupPath, origMode) })
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
	lockSeams(t)

	workspace := t.TempDir()
	makeSetupScriptExecutableForTest(t)

	syncInput, plan := setupPlanInput(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertExecutableBit(
		t,
		filepath.Join(workspace, config.DefaultTargetFolder, consts.Go, testRelGoSetupSh),
	)
}

func writeObsoleteFile(t *testing.T, workspace string) string {
	t.Helper()

	obsolete := filepath.Join(
		workspace,
		config.DefaultTargetFolder,
		consts.Go,
		testRelGoObsoleteTxt,
	)

	writeFileWithDir(t, obsolete, []byte("old"))

	return obsolete
}

func writeObsoleteManagedFile(t *testing.T, workspace string) string {
	t.Helper()

	obsolete := writeObsoleteFile(t, workspace)

	writeMinimalLock(&lockWriteInput{
		t: t, workspace: workspace, targetFolder: config.DefaultTargetFolder,
		files: []managed.File{
			{
				SourceModule:      consts.Empty,
				DestinationModule: consts.Go,
				SourcePath:        consts.Empty,
				Path:              config.DefaultTargetFolder + "/" + consts.Go + "/" + testRelGoObsoleteTxt,
				SHA256:            consts.Empty,
			},
		},
	})

	return obsolete
}

func failFirstPromoteHook(promoted *int) func(string, *syncdomain.FileEntry) error {
	return func(path string, entry *syncdomain.FileEntry) error {
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

func assertPromoteFails(
	t *testing.T,
	plan *syncdomain.Plan,
	syncInput *syncdomain.SyncInput,
) func() {
	t.Helper()

	return func() {
		err := syncsvc.ApplyPlan(plan, syncInput)
		if err == nil {
			t.Fatal("expected promote failure")
		}
	}
}

// TestApplyPlanPromoteBeforeDelete verifies obsolete files remain if a promote step fails before delete.
func TestApplyPlanPromoteBeforeDelete(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	obsolete := writeObsoleteManagedFile(t, workspace)

	syncInput, plan := setupPlanInput(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	var promoted int

	withCopyFileHook(&copyHookInput{
		t:    t,
		plan: plan,
		hook: failFirstPromoteHook(&promoted),
		run:  assertPromoteFails(t, plan, &syncInput),
	})

	assertFileExists(t, obsolete)
}

func recordingCopyHook(input *copyRecordInput) func(string, *syncdomain.FileEntry) error {
	return func(path string, entry *syncdomain.FileEntry) error {
		if strings.Contains(path, input.marker) {
			return writeFileEntry(path, entry)
		}

		rel, err := filepath.Rel(input.workspace, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}

		*input.order = append(*input.order, filepath.ToSlash(rel))

		return writeFileEntry(path, entry)
	}
}

func applyPlanOK(t *testing.T, plan *syncdomain.Plan, syncInput *syncdomain.SyncInput) func() {
	t.Helper()

	return func() {
		err := syncsvc.ApplyPlan(plan, syncInput)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func recordWriteOrder(input *writeOrderInput) []string {
	input.t.Helper()

	var order []string

	stagingMarker := filepath.Join(config.DefaultTargetFolder, testLegacyMetaDir, testStagingDir)

	withCopyFileHook(&copyHookInput{
		t:    input.t,
		plan: input.plan,
		hook: recordingCopyHook(&copyRecordInput{
			workspace: input.workspace, marker: stagingMarker, order: &order,
		}),
		run: applyPlanOK(input.t, input.plan, input.syncInput),
	})

	return order
}

// TestApplyPlanWriteOrder verifies modules are written before the lock and before metadata.
func TestApplyPlanWriteOrder(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	syncInput, plan := setupPlanInput(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	order := recordWriteOrder(&writeOrderInput{
		t: t, workspace: workspace, plan: plan, syncInput: &syncInput,
	})

	lockIdx := indexOfSuffix(order, testLockFileName)
	metaIdx := indexOfSuffix(order, testMetadataFileName)
	moduleIdx := indexOfContains(order, config.DefaultTargetFolder+"/"+consts.Go+"/")

	if lockIdx < moduleIdx || metaIdx < lockIdx {
		t.Fatalf("expected modules before lock before metadata, got %v", order)
	}
}

func indexOfSuffix(paths []string, suffix string) int {
	for i := range paths {
		if strings.HasSuffix(paths[i], suffix) {
			return i
		}
	}

	return notFoundIndex
}

func indexOfContains(paths []string, part string) int {
	for i := range paths {
		if strings.Contains(paths[i], part) {
			return i
		}
	}

	return notFoundIndex
}

func buildPlanFrom(input *buildPlanFromInput) *syncdomain.Plan {
	input.t.Helper()

	syncInput, err := syncsvc.PrepareSyncInput(&syncsvc.PrepareSyncInputArgs{
		Cfg:         input.cfg,
		Snapshot:    syncsvc.SnapshotPort(input.snap),
		TaskfileOps: synctaskfile.NewOps(),
		Resolutions: input.resolutions,
		DepSources:  input.depSources,
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
	lockSeams(t)

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
	lockSeams(t)

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

	if !metadataOnlyChangePlan(t).Changed {
		t.Fatal("expected metadata-only configuration hash change to mark plan changed")
	}
}

func metadataOnlyChangePlan(t *testing.T) *syncdomain.Plan {
	t.Helper()
	lockSeams(t)

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

	return plan2
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
	lockSeams(t)

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

func assertRemovedContains(t *testing.T, removed []string, rel string) {
	t.Helper()

	want := goManagedPath(rel)

	if !slices.Contains(removed, want) {
		t.Fatalf("Removed missing %q: %v", want, removed)
	}
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()

	stat, err := os.Stat(path)
	iox.Discard(stat)

	if !os.IsNotExist(err) {
		t.Fatalf("path %q should be absent, stat returned: %v", path, err)
	}
}

func assertDocsRemovedFromWorkspace(t *testing.T, workspace string) {
	t.Helper()

	moduleRoot := filepath.Join(workspace, config.DefaultTargetFolder, consts.Go)

	assertPathAbsent(t, filepath.Join(moduleRoot, testReadmeName))
	assertPathAbsent(t, filepath.Join(moduleRoot, filepath.FromSlash(docGuideMD)))
	assertPathAbsent(t, filepath.Join(moduleRoot, filepath.FromSlash(docNestedNoteMD)))
	assertPathAbsent(t, filepath.Join(moduleRoot, docsDirName))
	assertFileExists(t, filepath.Join(moduleRoot, testTaskfileName))
}

func assertRemovedDocPaths(t *testing.T, removed []string) {
	t.Helper()

	assertRemovedContains(t, removed, testReadmeName)
	assertRemovedContains(t, removed, docGuideMD)
	assertRemovedContains(t, removed, docNestedNoteMD)
}

func applyGoWithDocsPlan(t *testing.T) (workspace string, cfg *config.Config) {
	t.Helper()

	workspace = t.TempDir()
	cfg = applyGoWithDocsToWorkspace(t, workspace)

	return workspace, cfg
}

func applyGoWithDocsToWorkspace(t *testing.T, workspace string) *config.Config {
	t.Helper()

	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertFileExists(
		t,
		filepath.Join(workspace, config.DefaultTargetFolder, consts.Go, docsDirName, "guide.md"),
	)

	return cfg
}

func assertIncludesDocToggleRemovesDocs(t *testing.T, workspace string, cfg *config.Config) {
	t.Helper()

	cfg.IncludesDoc = false

	syncInput2, plan2 := preparePlan(t, workspace, cfg)

	if !plan2.Changed {
		t.Fatal("expected changes when includes-doc toggles")
	}

	assertRemovedDocPaths(t, plan2.Removed)

	err := runApplyPlan(t, plan2, &syncInput2)
	if err != nil {
		t.Fatal(err)
	}

	assertDocsRemovedFromWorkspace(t, workspace)
}

// TestConfigurationChangeMarksUpdated verifies a config field change marks the plan updated.
func TestConfigurationChangeMarksUpdated(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace, cfg := applyGoWithDocsPlan(t)
	assertIncludesDocToggleRemovesDocs(t, workspace, cfg)
}

// TestLoadMetadataCorruptFails verifies malformed metadata YAML returns an error.
func TestLoadMetadataCorruptFails(t *testing.T) {
	t.Parallel()
	assertCorruptMetadata(t)
}

func assertCorruptMetadata(t *testing.T) {
	t.Helper()
	lockSeams(t)

	root, rel := writeCorruptFixture(t, testMetadataFileName)

	meta, err := syncsvc.LoadMetadata(root, rel)
	iox.Discard(meta)

	if err == nil {
		t.Fatal(errExpectedCorruptMetadata)
	}
}

// TestLoadLockCorruptFails verifies malformed lock YAML returns an error.
func TestLoadLockCorruptFails(t *testing.T) {
	t.Parallel()
	assertCorruptLock(t)
}

func assertCorruptLock(t *testing.T) {
	t.Helper()
	lockSeams(t)

	root, rel := writeCorruptFixture(t, "lock.yml")

	lock, err := syncsvc.LoadLock(root, rel)
	iox.Discard(lock)

	if err == nil {
		t.Fatal(errExpectedCorruptLock)
	}
}

func writeCorruptFixture(t *testing.T, rel string) (string, string) {
	t.Helper()

	root := t.TempDir()

	err := os.WriteFile(
		filepath.Join(root, rel),
		[]byte(testBadYAML),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	return root, rel
}

// TestPackageManagerSwitchSameDestination verifies different package manager variants normalize to eslint.
func TestPackageManagerSwitchSameDestination(t *testing.T) {
	t.Parallel()

	mods := []string{"eslint/node/pnpm", "eslint/node/npm", "eslint/bun"}

	for i := range mods {
		t.Run(mods[i], func(t *testing.T) {
			t.Parallel()

			assertNormalizesToEslint(t, mods[i])
		})
	}
}

// TestPrefixSafetyPreservesUnrelatedPaths verifies paths sharing a prefix with the target folder are untouched.
func TestPrefixSafetyPreservesUnrelatedPaths(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	extra := filepath.Join(workspace, "taskfiles-extra", "foo.txt")
	writeFileWithDir(t, extra, []byte("stay"))
	writeTaskGoManagedLock(t, workspace)

	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)
	discardCfg(cfg)

	assertApplyPlanPreservesPath(&preservePathInput{
		t: t, plan: plan, syncInput: &syncInput, path: extra,
	})
}

// TestTargetFolderMigration verifies files migrate to the new target folder while unmanaged files stay.
func TestTargetFolderMigration(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	fixture := writeOldTargetFixture(t, workspace)

	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)
	discardCfg(cfg)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertMigrationResult(&migrationAssertInput{
		t: t, workspace: workspace, oldManaged: fixture.oldManaged, oldUser: fixture.oldUser,
	})
}

func assertMigrationResult(input *migrationAssertInput) {
	input.t.Helper()

	assertFileExists(
		input.t,
		filepath.Join(input.workspace, config.DefaultTargetFolder, testGoTaskfilePath),
	)

	stat, err := os.Stat(input.oldManaged)
	iox.Discard(stat)

	if err == nil {
		input.t.Fatal("old managed file under previous target folder should be removed")
	}

	stat, err = os.Stat(input.oldUser)
	iox.Discard(stat)

	if err != nil {
		input.t.Fatal("unknown files outside managed set should be preserved")
	}
}

func assertNormalizesToEslint(t *testing.T, mod string) {
	t.Helper()

	sourceToDest, err := resolvesvc.BuildDestinationMap([]string{mod})
	if err != nil {
		t.Fatal(err)
	}

	if sourceToDest[mod] != testModuleEslint {
		t.Fatalf("%s should normalize to eslint, got %q", mod, sourceToDest[mod])
	}
}

func writeOldTargetFixture(t *testing.T, workspace string) oldTargetFixture {
	t.Helper()

	oldManaged := filepath.Join(workspace, taskGoTaskfilePath)
	writeFileWithDir(t, oldManaged, []byte("version: '3'\n"))

	oldUser := filepath.Join(workspace, targetFolderTask, consts.Go, testFileUserTxt)
	writeFileWithDir(t, oldUser, []byte(contentKeep))

	writeTaskGoManagedLock(t, workspace)

	return oldTargetFixture{oldManaged: oldManaged, oldUser: oldUser}
}

func writeTaskGoManagedLock(t *testing.T, workspace string) {
	t.Helper()

	writeMinimalLock(&lockWriteInput{
		t: t, workspace: workspace, targetFolder: targetFolderTask,
		files: []managed.File{{
			SourceModule:      consts.Empty,
			DestinationModule: consts.Go,
			SourcePath:        consts.Empty,
			Path:              taskGoTaskfilePath,
			SHA256:            consts.Empty,
		}},
	})
}

func writeModuleFixtureFiles(t *testing.T, dir string) {
	t.Helper()

	writeModuleFile(
		&moduleFileInput{t: t, dir: dir, rel: testTaskfileName, content: "version: \"3\"\n"},
	)
	writeModuleFile(&moduleFileInput{t: t, dir: dir, rel: testReadmeName, content: "docs\n"})
	writeModuleFile(&moduleFileInput{t: t, dir: dir, rel: docGuideMD, content: "guide\n"})
	writeModuleFile(
		&moduleFileInput{t: t, dir: dir, rel: fileGoTestGo, content: "package go_test\n"},
	)
	writeModuleFile(
		&moduleFileInput{t: t, dir: dir, rel: testMetadataFileName, content: testModuleGoMetadata},
	)
	writeModuleFile(
		&moduleFileInput{t: t, dir: dir, rel: docMetadataYML, content: testModuleGoMetadata},
	)
}

func collectOptions(dir string, policy syncsvc.DocPolicy) *syncsvc.CollectOptions {
	return &syncsvc.CollectOptions{
		TaskfileOps:  nil,
		SourceDir:    dir,
		DocPolicy:    policy,
		SourceToDest: nil,
		FromDest:     "",
	}
}

func assertCollectedWithoutDocs(t *testing.T, dir string) {
	t.Helper()

	withoutDocs, err := syncsvc.CollectModuleFiles(collectOptions(dir, syncsvc.DocPolicySkip))
	if err != nil {
		t.Fatal(err)
	}

	assertCollected(t, withoutDocs, map[string]bool{
		testTaskfileName: true,
		testReadmeName:   false,
		docGuideMD:       false,
		docMetadataYML:   false,
	})
}

func assertCollectedWithDocs(t *testing.T, dir string) {
	t.Helper()

	withDocs, err := syncsvc.CollectModuleFiles(collectOptions(dir, syncsvc.DocPolicyInclude))
	if err != nil {
		t.Fatal(err)
	}

	assertCollected(t, withDocs, map[string]bool{
		testTaskfileName:     true,
		testReadmeName:       true,
		docGuideMD:           true,
		docMetadataYML:       true,
		fileGoTestGo:         false,
		testMetadataFileName: false,
	})
}

// TestCollectModuleFilesSkipsTestsAndDocs verifies test and doc files are excluded unless requested.
func TestCollectModuleFilesSkipsTestsAndDocs(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	dir := t.TempDir()
	writeModuleFixtureFiles(t, dir)
	assertCollectedWithDocs(t, dir)
	assertCollectedWithoutDocs(t, dir)
}

func assertCollected(t *testing.T, contents map[string]syncdomain.FileEntry, want map[string]bool) {
	t.Helper()

	for path := range want {
		_, ok := contents[path]

		if ok != want[path] {
			t.Fatalf("collected %q = %t, want %t", path, ok, want[path])
		}
	}
}

func writeNestedDocsStore(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFileWithDir(t, filepath.Join(root, ".deps.yml"), []byte(parentDocLeaf+": []\n"))

	writeNestedDocsToolDir(t, root)

	writeNestedDocsTaskfile(t, root, parentDocNode)
	writeNestedDocsTaskfile(t, root, parentDocLeaf)

	return root
}

func writeNestedDocsTaskfile(t *testing.T, root, module string) {
	t.Helper()

	writeModuleFile(&moduleFileInput{
		t:       t,
		dir:     filepath.Join(root, config.DefaultTargetFolder, module),
		rel:     testTaskfileName,
		content: testEmptyTaskfileYAML,
	})
}

func writeNestedDocsToolDir(t *testing.T, root string) {
	t.Helper()

	writeNestedDocsTaskfile(t, root, parentDocTool)
	writeModuleFile(&moduleFileInput{
		t:       t,
		dir:     filepath.Join(root, config.DefaultTargetFolder, parentDocTool),
		rel:     testReadmeName,
		content: parentDocRootReadme,
	})
}

func writeNestedDocsStoreWithLeafReadme(t *testing.T) string {
	t.Helper()

	root := writeNestedDocsStore(t)
	leafDir := filepath.Join(root, config.DefaultTargetFolder, parentDocLeaf)
	writeModuleFile(&moduleFileInput{
		t: t, dir: leafDir, rel: testReadmeName, content: parentDocLeafReadme,
	})

	return root
}

func nestedDocsSnapshot(t *testing.T, storeRoot string) *storedomain.Snapshot {
	t.Helper()

	snap, err := storesvc.LocalSnapshot(storeRoot, testStoreRefInfo())
	if err != nil {
		t.Fatal(err)
	}

	return snap
}

func nestedDocsSyncInput(
	cfg *config.Config,
	snap *storedomain.Snapshot,
) syncdomain.SyncInput {
	return variantModuleSyncInput(&variantModuleSyncArgs{
		cfg: cfg, snap: snap, task: parentDocTool, source: parentDocLeaf,
	})
}

func mutateNestedDocs(includesDoc bool) func(*config.Config) {
	return func(cfg *config.Config) {
		cfg.Tasks = []string{parentDocTool}
		cfg.IncludesDoc = includesDoc
	}
}

func findManagedFile(plan *syncdomain.Plan, path string) *managed.File {
	for i := range plan.ManagedFiles {
		if plan.ManagedFiles[i].Path == path {
			return &plan.ManagedFiles[i]
		}
	}

	return nil
}

func assertParentReadmePath(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	wantPath := pathutil.JoinRelative(config.DefaultTargetFolder, parentDocTool, testReadmeName)
	wantSource := pathutil.JoinRelative(config.DefaultTargetFolder, parentDocTool, testReadmeName)

	file := findManagedFile(plan, wantPath)

	if file == nil {
		t.Fatalf(errFmtExpectedManagedPath, wantPath)
	}

	if file.SourcePath != wantSource {
		t.Fatalf("SourcePath = %q, want %q", file.SourcePath, wantSource)
	}
}

func assertParentReadmeContent(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	entry, ok := plan.ModuleContents[parentDocLeaf][testReadmeName]

	if !ok {
		t.Fatal("expected README in module contents")
	}

	if string(entry.Data) != parentDocRootReadme {
		t.Fatalf("README content = %q, want %q", entry.Data, parentDocRootReadme)
	}
}

func assertParentReadmeCollected(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	assertParentReadmePath(t, plan)
	assertParentReadmeContent(t, plan)
}

func finishNestedDocsPlan(
	t *testing.T,
	mutate func(*config.Config),
	storeRoot string,
) *syncdomain.Plan {
	t.Helper()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	snap := nestedDocsSnapshot(t, storeRoot)
	cfg := testConfig(workspace, mutate)
	si := nestedDocsSyncInput(cfg, snap)

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	return plan
}

func buildNestedDocsPlan(t *testing.T, includesDoc bool) *syncdomain.Plan {
	t.Helper()

	return finishNestedDocsPlan(t, mutateNestedDocs(includesDoc), writeNestedDocsStore(t))
}

func buildNestedDocsPlanWithLeafReadme(t *testing.T) *syncdomain.Plan {
	t.Helper()

	return finishNestedDocsPlan(t, mutateNestedDocs(true), writeNestedDocsStoreWithLeafReadme(t))
}

// TestLogicalRootDocsMergedFromParent verifies parent README is collected when includes-doc is
// true and skipped when false, with logical-root docs winning over a leaf README.
func TestLogicalRootDocsMergedFromParent(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	withDocs := buildNestedDocsPlan(t, true)
	assertParentReadmeCollected(t, withDocs)

	withoutDocs := buildNestedDocsPlan(t, false)
	assertNoReadmeManaged(t, withoutDocs)

	collision := buildNestedDocsPlanWithLeafReadme(t)
	assertParentReadmeCollected(t, collision)
}

func buildPlanWithLeafMetadata(t *testing.T, metadata string) error {
	t.Helper()

	storeRoot := writeNestedDocsStore(t)
	leafDir := filepath.Join(storeRoot, config.DefaultTargetFolder, parentDocLeaf)
	writeModuleFile(&moduleFileInput{
		t: t, dir: leafDir, rel: testMetadataFileName, content: metadata,
	})

	err := buildNestedDocsMetadataPlan(t, storeRoot)
	if err != nil {
		return fmt.Errorf("leaf metadata plan: %w", err)
	}

	return nil
}

func buildNestedDocsMetadataPlan(t *testing.T, storeRoot string) error {
	t.Helper()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	si := nestedDocsSyncInput(
		testConfig(workspace, mutateNestedDocs(true)),
		nestedDocsSnapshot(t, storeRoot),
	)

	plan, err := syncsvc.BuildPlan(&si)
	iox.Discard(plan)

	if err != nil {
		return fmt.Errorf("build plan: %w", err)
	}

	return nil
}

// TestStoreMetadataAcceptsCurrentSchema verifies the pinned metadata.yml schema is accepted.
func TestStoreMetadataAcceptsCurrentSchema(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := buildPlanWithLeafMetadata(t, currentSchemaMetadata)
	if err != nil {
		t.Fatalf("BuildPlan with current schema: %v", err)
	}
}

// TestStoreMetadataRejectsUnknownSchema verifies an unrecognized metadata.yml schema
// fails the sync rather than being silently ignored.
func TestStoreMetadataRejectsUnknownSchema(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := buildPlanWithLeafMetadata(t, legacySchemaMetadata)
	if err == nil {
		t.Fatal("expected unsupported metadata schema error")
	}

	if !strings.Contains(err.Error(), "unsupported metadata.yml schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func fixtureStore(t *testing.T) *storedomain.Snapshot {
	t.Helper()

	root := filepath.Join(
		consts.PathParent, consts.PathParent, consts.PathParent, consts.PathParent,
		dirTests, dirFixtures, dirStore,
	)

	snap, err := storesvc.LocalSnapshot(root, testStoreRefInfo())
	if err != nil {
		t.Fatal(err)
	}

	return snap
}

func writeRootTaskfile(t *testing.T, workspace string) {
	t.Helper()

	content := []byte(`version: "3"
includes: {}
tasks:
  hello:
    cmds:
      - echo hello
`)

	writeFileWithDir(t, filepath.Join(workspace, testTaskfileName), content)
}

func dependencySources(
	t *testing.T,
	sources []string,
	snap *storedomain.Snapshot,
) ([]string, error) {
	t.Helper()

	deps, err := resolvesvc.ResolveTransitive(sources, snap.Deps)
	if err != nil {
		return nil, fmt.Errorf("resolve transitive deps: %w", err)
	}

	return deps, nil
}

func mutateEslintGoPnpm(cfg *config.Config) {
	cfg.Tasks = []string{testModuleEslint, consts.Go}
	cfg.NodePackageManager = config.PMPnpm
	cfg.IncludesDoc = true
}

func assertGeneratedRootTaskNames(t *testing.T, rootText string) {
	t.Helper()

	wants := []string{
		"task: go:lint",
		"task: eslint:lint",
		"task: go:lint:fix",
		"task: eslint:lint:fix",
	}

	for i := range wants {
		if !strings.Contains(rootText, wants[i]) {
			t.Fatalf("expected %q in root Taskfile:\n%s", wants[i], rootText)
		}
	}
}

func assertGeneratedRootTasks(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	rootText := string(plan.RootTaskfile)
	assertGeneratedRootTaskNames(t, rootText)

	if strings.Contains(rootText, "task: pnpm:lint") {
		t.Fatalf("dependency-only module should not contribute root tasks:\n%s", rootText)
	}

	wantGenerated := []string{"install", "lint", "lint:fix", "version"}

	if !slices.Equal(plan.Lock.GeneratedRootTasks, wantGenerated) {
		t.Fatalf(
			"generated root tasks = %#v, want %#v",
			plan.Lock.GeneratedRootTasks,
			wantGenerated,
		)
	}
}

// TestBuildPlanInitialSync verifies an initial sync marks the plan changed and generates root tasks.
func TestBuildPlanInitialSync(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	cfg := testConfig(workspace, mutateEslintGoPnpm)
	plan := replan(t, workspace, cfg)

	if !plan.Changed {
		t.Fatal(errExpectedChangesInitial)
	}

	assertGeneratedRootTasks(t, plan)
}

// TestBuildPlanCreatesRootTaskfile verifies the root Taskfile.yml is added on initial sync.
func TestBuildPlanCreatesRootTaskfile(t *testing.T) {
	t.Parallel()

	plan := buildRootTaskfilePlan(t)

	if !plan.Changed {
		t.Fatal(errExpectedChangesInitial)
	}

	if !containsRootTaskfile(plan.Added) {
		t.Fatalf("expected root Taskfile.yml in added files, got added=%v", plan.Added)
	}
}

func buildRootTaskfilePlan(t *testing.T) *syncdomain.Plan {
	t.Helper()
	lockSeams(t)

	workspace := t.TempDir()
	snap := fixtureStore(t)
	cfg := testConfig(workspace, mutateGoWithDocs)

	resolutions, depSources := resolveModsForTest(&moduleTestInput{t: t, cfg: cfg, snap: snap})
	plan := buildPlanFrom(&buildPlanFromInput{
		t: t, cfg: cfg, snap: snap, resolutions: resolutions, depSources: depSources,
	})

	return plan
}

func containsRootTaskfile(list []string) bool {
	return slices.Contains(list, testTaskfileName)
}

func writeUnmanagedFile(t *testing.T, workspace string) {
	t.Helper()

	dir := filepath.Join(workspace, config.DefaultTargetFolder, testModuleEslint)

	err := os.MkdirAll(dir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	writeFileWithDir(
		t,
		filepath.Join(dir, testFileUserTxt),
		[]byte(contentKeep),
	)
}

func resolveEslintMod(
	t *testing.T,
	cfg *config.Config,
	snap *storedomain.Snapshot,
) resolvesvc.Resolution {
	t.Helper()

	return mustResolveTask(t, &resolvesvc.ResolveInput{
		Task: testModuleEslint, Catalog: snap.Catalog,
		PackageManager: cfg.NodePackageManager,
	})
}

func buildEslintSyncInput(
	t *testing.T,
	cfg *config.Config,
	snap *storedomain.Snapshot,
) syncdomain.SyncInput {
	t.Helper()

	res := resolveEslintMod(t, cfg, snap)

	return eslintSyncInput(cfg, snap, res.SourceModule)
}

func eslintSyncInput(
	cfg *config.Config,
	snap *storedomain.Snapshot,
	source string,
) syncdomain.SyncInput {
	return variantModuleSyncInput(&variantModuleSyncArgs{
		cfg: cfg, snap: snap, task: testModuleEslint, source: source,
	})
}

func mutateEslintPnpm(cfg *config.Config) {
	cfg.Tasks = []string{testModuleEslint}
	cfg.NodePackageManager = config.PMPnpm
	cfg.IncludesDoc = true
}

// TestUnmanagedDestinationConflict verifies planning refuses to overwrite an unmanaged existing file.
func TestUnmanagedDestinationConflict(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	writeUnmanagedFile(t, workspace)

	snap := fixtureStore(t)
	cfg := testConfig(workspace, mutateEslintPnpm)
	si := buildEslintSyncInput(t, cfg, snap)

	expectBuildPlanError(t, &si, "expected unmanaged destination conflict")
}

func mutateEslintPnpmNoDocs(cfg *config.Config) {
	cfg.Tasks = []string{testModuleEslint}
	cfg.NodePackageManager = config.PMPnpm
	cfg.IncludesDoc = false
}

func assertNoReadmeManaged(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	for i := range plan.ManagedFiles {
		if filepath.Base(plan.ManagedFiles[i].Path) == testReadmeName {
			t.Fatal("README should be excluded when includes-doc=false")
		}
	}
}

func goManagedPath(rel string) string {
	return pathutil.JoinRelative(config.DefaultTargetFolder, consts.Go, rel)
}

func managedPathSet(plan *syncdomain.Plan) map[string]struct{} {
	out := make(map[string]struct{}, len(plan.ManagedFiles))

	for i := range plan.ManagedFiles {
		out[plan.ManagedFiles[i].Path] = struct{}{}
	}

	return out
}

func assertManagedHasPath(t *testing.T, paths map[string]struct{}, rel string) {
	t.Helper()

	full := goManagedPath(rel)

	if _, ok := paths[full]; !ok {
		t.Fatalf(errFmtExpectedManagedPath, full)
	}
}

func assertManagedLacksPath(t *testing.T, paths map[string]struct{}, rel string) {
	t.Helper()

	full := goManagedPath(rel)

	if _, ok := paths[full]; ok {
		t.Fatalf("managed path %q should be excluded when includes-doc=false", full)
	}
}

func assertNoDocPathsManaged(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	paths := managedPathSet(plan)

	assertNoReadmeManaged(t, plan)
	assertManagedLacksPath(t, paths, docGuideMD)
	assertManagedLacksPath(t, paths, docNestedNoteMD)
}

func assertDocPathsManaged(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	paths := managedPathSet(plan)

	assertManagedHasPath(t, paths, testReadmeName)
	assertManagedHasPath(t, paths, docGuideMD)
	assertManagedHasPath(t, paths, docNestedNoteMD)
}

func eslintManagedReadmePath() string {
	return pathutil.JoinRelative(config.DefaultTargetFolder, testModuleEslint, testReadmeName)
}

func assertEslintReadmeManaged(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	wantPath := eslintManagedReadmePath()

	for i := range plan.ManagedFiles {
		file := &plan.ManagedFiles[i]

		if file.Path != wantPath {
			continue
		}

		if file.SourcePath != wantPath {
			t.Fatalf("README SourcePath = %q, want %q", file.SourcePath, wantPath)
		}

		return
	}

	t.Fatalf(errFmtExpectedManagedPath, wantPath)
}

func buildEslintPlan(t *testing.T, mutate func(*config.Config)) *syncdomain.Plan {
	t.Helper()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	snap := fixtureStore(t)
	cfg := testConfig(workspace, mutate)
	si := buildEslintSyncInput(t, cfg, snap)

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	return plan
}

// TestIncludesDocFalseSkipsReadme verifies README is excluded when includes-doc is false.
func TestIncludesDocFalseSkipsReadme(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	assertNoReadmeManaged(t, buildEslintPlan(t, mutateEslintPnpmNoDocs))
}

// TestIncludesDocTrueIncludesEslintReadme verifies nested eslint pulls README when includes-doc is true.
func TestIncludesDocTrueIncludesEslintReadme(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	assertEslintReadmeManaged(t, buildEslintPlan(t, mutateEslintPnpm))
}

// TestIncludesDocFalseSkipsFixtureDocs verifies go fixture README and docs/ are excluded when includes-doc is false.
func TestIncludesDocFalseSkipsFixtureDocs(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{consts.Go}
		cfg.IncludesDoc = false
	})
	si := buildSingleSyncIn(&moduleTestInput{t: t, cfg: cfg, snap: fixtureStore(t)})

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	assertNoDocPathsManaged(t, plan)
}

// TestIncludesDocTrueIncludesFixtureDocs verifies go fixture README and docs/ are managed when includes-doc is true.
func TestIncludesDocTrueIncludesFixtureDocs(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	cfg := testConfig(workspace, mutateGoWithDocs)
	si := buildSingleSyncIn(&moduleTestInput{t: t, cfg: cfg, snap: fixtureStore(t)})

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	assertDocPathsManaged(t, plan)
}

func fatalOnErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func mustResolveTask(t *testing.T, input *resolvesvc.ResolveInput) resolvesvc.Resolution {
	t.Helper()

	res, err := resolvesvc.Resolve(input)
	fatalOnErr(t, err)

	return res
}

func resolveGoModule(t *testing.T, snap *storedomain.Snapshot) resolvesvc.Resolution {
	t.Helper()

	return mustResolveTask(t, &resolvesvc.ResolveInput{
		Task: consts.Go, Catalog: snap.Catalog,
		PackageManager: consts.Empty,
	})
}

func buildSingleSyncIn(input *moduleTestInput) syncdomain.SyncInput {
	input.t.Helper()

	res := resolveGoModule(input.t, input.snap)

	return variantModuleSyncInput(&variantModuleSyncArgs{
		cfg: input.cfg, snap: input.snap, task: consts.Go, source: res.SourceModule,
	})
}

func variantModuleSyncInput(args *variantModuleSyncArgs) syncdomain.SyncInput {
	return syncdomain.SyncInput{
		Config:      args.cfg,
		TaskfileOps: synctaskfile.NewOps(),
		Snapshot:    syncsvc.SnapshotPort(args.snap),
		Requested: map[string]lockmodel.ModuleRecord{
			args.task: {
				SourceModule:      args.source,
				DestinationModule: args.task,
				Path:              config.DefaultTargetFolder + "/" + args.task,
			},
		},
		Dependencies: nil,
		SourceToDest: map[string]string{args.source: args.task},
		DestByTask:   map[string]string{args.task: args.task},
	}
}

func testStoreRefInfo() *storedomain.RefInfo {
	return &storedomain.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: consts.Empty,
		SourceRef:        testStoreSourceRef,
		ResolvedCommit:   testStoreResolvedCommit,
		DefaultBranch:    testStoreDefaultBranch,
	}
}

func resolveAllModules(input *moduleTestInput) []resolvesvc.Resolution {
	input.t.Helper()

	resolutions, err := resolvesvc.ResolveAll(&resolvesvc.ResolveAllInput{
		Tasks: input.cfg.Tasks, Catalog: input.snap.Catalog,
		PackageManager: input.cfg.NodePackageManager,
	})
	fatalOnErr(input.t, err)

	return resolutions
}

func runApplyPlan(t *testing.T, plan *syncdomain.Plan, syncInput *syncdomain.SyncInput) error {
	t.Helper()

	err := syncsvc.ApplyPlan(plan, syncInput)
	if err != nil {
		return fmt.Errorf("apply plan: %w", err)
	}

	return nil
}

func withCopyFileHook(input *copyHookInput) {
	input.t.Helper()

	syncsvc.SetCopyFileToHookForTest(input.plan, input.hook)
	input.t.Cleanup(func() { syncsvc.SetCopyFileToHookForTest(input.plan, nil) })

	input.run()
}

func defaultTestConfig(workspace string) *config.Config {
	return &config.Config{
		Tasks:              nil,
		JSRuntime:          consts.Empty,
		NodePackageManager: consts.Empty,
		IncludesDoc:        false,
		SyncRoot:           true,
		FailOnChanges:      false,
		StoreVersion:       consts.Empty,
		TargetFolder:       config.DefaultTargetFolder,
		RootTaskfile:       testTaskfileName,
		GitHubToken:        consts.Empty,
		Workspace:          workspace,
		Repository:         consts.Empty,
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}
}

func testConfig(workspace string, mutate func(*config.Config)) *config.Config {
	cfg := defaultTestConfig(workspace)

	if mutate != nil {
		mutate(cfg)
	}

	return cfg
}

func writeFileWithDir(t *testing.T, path string, data []byte) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, data, consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}
}

func writeTaskotterMetadata(input *metadataWriteInput) {
	input.t.Helper()

	path := filepath.Join(input.workspace, input.targetFolder, testMetadataRelPath)
	writeFileWithDir(input.t, path, input.meta)
}

func writeLockFile(input *lockWriteInput) {
	input.t.Helper()

	var lock lockmodel.LockFile

	lock.Configuration.TargetFolder = input.targetFolder
	lock.ManagedFiles = input.files

	lockPath := filepath.Join(input.workspace, input.targetFolder, testLockFileName)
	writeFileWithDir(input.t, lockPath, syncsvc.MarshalLock(&lock))
}

func writeMinimalLock(input *lockWriteInput) {
	input.t.Helper()

	writeLockFile(input)

	meta := []byte(
		"target_folder: " + input.targetFolder +
			"\nlock_file: " + input.targetFolder + "/.taskotter-lock.yml\nconfiguration_hash: x\n",
	)

	writeTaskotterMetadata(&metadataWriteInput{
		t: input.t, workspace: input.workspace, targetFolder: input.targetFolder, meta: meta,
	})
}

func writeModuleFile(input *moduleFileInput) {
	input.t.Helper()

	writeFileWithDir(input.t, filepath.Join(input.dir, input.rel), []byte(input.content))
}

func setupPlan(
	args *setupPlanArgs,
) (cfg *config.Config, syncInput syncdomain.SyncInput, plan *syncdomain.Plan) {
	args.t.Helper()

	writeRootTaskfile(args.t, args.workspace)

	cfg = testConfig(args.workspace, args.mutate)
	syncInput, plan = preparePlan(args.t, args.workspace, cfg)

	return cfg, syncInput, plan
}

func replan(t *testing.T, workspace string, cfg *config.Config) *syncdomain.Plan {
	t.Helper()

	syncInput, plan := preparePlan(t, workspace, cfg)
	iox.Discard(syncInput)

	return plan
}

func expectBuildPlanError(t *testing.T, si *syncdomain.SyncInput, wantErr string) {
	t.Helper()

	plan, err := syncsvc.BuildPlan(si)
	iox.Discard(plan)

	if err == nil {
		t.Fatal(wantErr)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	stat, err := os.Stat(path)
	iox.Discard(stat)

	if err != nil {
		t.Fatal(err)
	}
}

func assertApplyPlanPreservesPath(input *preservePathInput) {
	input.t.Helper()

	err := runApplyPlan(input.t, input.plan, input.syncInput)
	if err != nil {
		input.t.Fatal(err)
	}

	assertFileExists(input.t, input.path)
}

func setupPlanInput(args *setupPlanArgs) (syncInput syncdomain.SyncInput, plan *syncdomain.Plan) {
	args.t.Helper()

	var cfg *config.Config

	cfg, syncInput, plan = setupPlan(args)
	discardCfg(cfg)

	return syncInput, plan
}

func discardCfg(cfg *config.Config) {
	iox.Discard(cfg)
}
