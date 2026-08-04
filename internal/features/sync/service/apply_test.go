// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	assertRootSkippedInput struct {
		t           *testing.T
		workspace   string
		rootPath    string
		rootContent string
	}

	writeOrderInput struct {
		t         *testing.T
		plan      *syncdomain.Plan
		syncInput *syncdomain.SyncInput
		workspace string
	}

	copyRecordInput struct {
		order     *[]string
		workspace string
		marker    string
	}
)

var errSimulatedPromoteFailure = errors.New("simulated promote failure")

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
) (syncdomain.SyncInput, *syncdomain.Plan) {
	t.Helper()

	snap := fixtureStore(t)
	resolutions, depSources := resolveModsForTest(&moduleTestInput{t: t, cfg: cfg, snap: snap})

	syncInput, err := syncsvc.PrepareSyncInput(&syncsvc.PrepareSyncInputArgs{
		Cfg: cfg, Snapshot: snap, TaskfileOps: synctaskfile.Ops{},
		Resolutions: resolutions, DepSources: depSources,
	})
	if err != nil {
		t.Fatal(err)
	}

	plan := buildPlanFromSyncInput(t, &syncInput)

	return syncInput, plan
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

func setupPlanWithRootContent(args *setupPlanRootArgs) (syncdomain.SyncInput, *syncdomain.Plan) {
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

	assertFileExists(t, filepath.Join(workspace, cfg.MetadataPath()))

	stat, err := os.Stat(filepath.Join(workspace, config.LegacyMetadataPath))
	iox.Discard(stat)

	if !os.IsNotExist(err) {
		t.Fatalf("legacy metadata should be removed, stat returned: %v", err)
	}
}

// TestApplyPlanWritesFiles verifies applying a plan writes module and metadata files.
func TestApplyPlanWritesFiles(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join(workspace, config.DefaultTargetFolder, testGoTaskfilePath))
	assertFileExists(t, filepath.Join(workspace, cfg.MetadataPath()))
}

// TestApplyPlanMigratesLegacyMetadataPath verifies legacy metadata is migrated and the old file removed.
func TestApplyPlanMigratesLegacyMetadataPath(t *testing.T) {
	t.Parallel()

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
