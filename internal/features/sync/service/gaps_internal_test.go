// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:exhaustruct_v5 // test fixtures only set fields exercised by the unit
package service

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	dirSnapshot struct {
		root string
	}
)

const (
	srcModuleName = "src"
	emptyTaskYAML = "version: \"3\"\ntasks: {}\n"
	goDestPath    = "taskfiles/go"
	targetWantFmt = "target = %q, want %q"
)

// TestBuildPlanFromStateReportsFinalizeFailure verifies finalizeBuiltPlan failures surface.
func TestBuildPlanFromStateReportsFinalizeFailure(t *testing.T) {
	t.Parallel()

	workspace, storeRoot := prepFinalizeFailDirs(t)
	input := goModuleSyncInput(workspace, storeRoot)

	plan, err := buildPlanFromState(
		input,
		&previousState{lock: managedGoLock(), target: consts.Empty},
	)
	iox.Discard(plan)
	assertFails(t, err)
}

// TestBuildPlanFromStateReportsPlanFailure verifies planAllFiles failures surface.
func TestBuildPlanFromStateReportsPlanFailure(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	input := minimalSyncInput(workspace)

	input.Config.SyncRoot = true
	input.Snapshot = stubSnapshot{root: workspace}

	plan, err := buildPlanFromState(input, &previousState{lock: nil, target: consts.Empty})
	iox.Discard(plan)
	assertFails(t, err)
}

// TestBuildRootPlanResultReportsReadFailure verifies root read failures surface.
func TestBuildRootPlanResultReportsReadFailure(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	assertNoErr(t, mkdirAll(filepath.Join(workspace, rootTaskfileName), dirModePerm))

	result, err := buildRootPlanResult(&buildRootPlanInput{
		oldLock:   nil,
		syncInput: minimalSyncInput(workspace),
	})

	iox.Discard(result)
	assertFails(t, err)
}

// TestBuildRootTaskfileReportsUpdateFailure verifies updateRootTaskfile failures surface.
func TestBuildRootTaskfileReportsUpdateFailure(t *testing.T) {
	t.Parallel()

	bytes, tasks, err := buildRootTaskfile(&buildRootArgs{
		syncInput:      failingRootSyncInput(t),
		moduleContents: mcMap{},
		rootBytes:      []byte(byteX),
	})

	iox.Discard(bytes)
	iox.Discard(tasks)
	assertFails(t, err)
}

// TestCollectAndTrackModuleFilesReportsFailure verifies collectModuleContents failures.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestCollectAndTrackModuleFilesReportsFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	contents, managed, err := collectAndTrackModuleFiles(sampleCollectArgs(t.TempDir()))
	iox.Discard(contents)
	iox.Discard(managed)
	assertFails(t, err)
}

// TestCollectModuleContentsReportsMergeFailure verifies mergeLogicalRootDocs failures.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestCollectModuleContentsReportsMergeFailure(t *testing.T) {
	args := distinctDocCollectArgs(t)
	swapWalkThenFail(t)

	contents, docs, err := collectModuleContents(args)
	iox.Discard(contents)
	iox.Discard(docs)
	assertFails(t, err)
}

// TestCollectModuleContentsReportsScanFailure verifies scanModuleFiles failures surface.
//
//nolint:paralleltest // swaps a package-level FS seam
func TestCollectModuleContentsReportsScanFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	contents, docs, err := collectModuleContents(sampleCollectArgs(t.TempDir()))
	iox.Discard(contents)
	iox.Discard(docs)
	assertFails(t, err)
}

// TestCollectModuleFileReportsStoreFailure verifies storeCollectedModuleFile failures.
func TestCollectModuleFileReportsStoreFailure(t *testing.T) {
	t.Parallel()

	err := collectModuleFile(&moduleCollectArgs{
		ops:          nil,
		sourceToDest: nil,
		fromDest:     consts.Empty,
		docPolicy:    0,
		sourceDir:    srcDir,
		absPath:      srcFileA,
		entry:        fakeDirEntry{name: pathA, dir: false},
		contents:     fMap{},
	})

	assertFails(t, err)
}

// TestDiffFilesReportsLockMetadataFailure verifies lock/metadata failures after managed OK.
func TestDiffFilesReportsLockMetadataFailure(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	assertNoErr(t, mkdirAll(filepath.Join(workspace, filepath.FromSlash(lockRelPath)), dirModePerm))

	lists, err := diffFiles(&diffInput{
		plan:         planWithLockMeta(lockRelPath),
		workspace:    workspace,
		metadataPath: metaRelPath,
		plannedMeta:  []byte(byteX),
		syncRoot:     syncRootDisabled,
	})

	iox.Discard(lists)
	assertFails(t, err)
}

// TestDiffLockFileReportsContentFailure verifies lockContentChanged failures surface.
//
//nolint:paralleltest // swaps the package-level marshalYAML seam
func TestDiffLockFileReportsContentFailure(t *testing.T) {
	swapMarshalYAML(t, failingMarshalYAML)

	plan := emptyPlan()

	plan.OldLock = emptyLockPtr()
	plan.Lock = emptyLock()
	plan.Metadata = emptyMetadata(lockRelPath)

	lists, err := diffLockFile(&diffLockArgs{
		plan:      &plan,
		workspace: t.TempDir(),
		lockPath:  lockRelPath,
	})
	iox.Discard(lists)
	assertFails(t, err)
}

// TestDiscoverPreviousMetadataReportsWalkFailure verifies candidate collection failures.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestDiscoverPreviousMetadataReportsWalkFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	meta, err := discoverPreviousMetadata(t.TempDir(), metaRelPath)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestFinalizeBuiltPlanReportsDiffFailure verifies finalizePlanDiff failures surface.
func TestFinalizeBuiltPlanReportsDiffFailure(t *testing.T) {
	t.Parallel()

	workspace := prepFinalizeDiffWorkspace(t)
	out, err := finalizeBuiltPlan(finalizeDiffFailInput(workspace))
	iox.Discard(out)
	assertFails(t, err)
}

// TestLoadPreviousLockUsesLockTarget verifies empty metadata target falls back to lock.
func TestLoadPreviousLockUsesLockTarget(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeValidLock(t, workspace, oldTargetFolder)

	lock, target, err := loadPreviousLock(
		workspace,
		emptyConfigPtr(),
		emptyMetadataPtr(lockRelPath),
	)
	assertNoErr(t, err)

	iox.Discard(lock)

	if target != oldTargetFolder {
		t.Fatalf(targetWantFmt, target, oldTargetFolder)
	}
}

// TestMergeLogicalRootDocsReportsMergeFailure verifies parent-doc merge failures.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestMergeLogicalRootDocsReportsMergeFailure(t *testing.T) {
	args := distinctDocCollectArgs(t)
	assertNoErr(t, mkdirAll(args.syncInput.Snapshot.ModuleDir(consts.Go), dirModePerm))

	swapWalkDir(t, failingWalk)

	docs, err := mergeLogicalRootDocs(args, fMap{}, docPolicyInclude)
	iox.Discard(docs)
	assertFails(t, err)
}

// TestMergeParentDocsIfDistinctReportsFailure verifies mergeParentDocFiles failures.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestMergeParentDocsIfDistinctReportsFailure(t *testing.T) {
	args := distinctDocCollectArgs(t)
	assertNoErr(t, mkdirAll(args.syncInput.Snapshot.ModuleDir(consts.Go), dirModePerm))

	swapWalkDir(t, failingWalk)

	docs, err := mergeParentDocsIfDistinct(args, fMap{}, map[string]struct{}{})
	iox.Discard(docs)
	assertFails(t, err)
}

// TestPlanModuleFilesReportsCollectFailure verifies collectAndTrackModuleFiles failures.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestPlanModuleFilesReportsCollectFailure(t *testing.T) {
	workspace := t.TempDir()
	assertNoErr(t, mkdirAll(filepath.Join(workspace, consts.Go), dirModePerm))
	swapWalkDir(t, failingWalk)

	contents, managed, err := planModuleFiles(goModulePlanArgs(workspace))
	iox.Discard(contents)
	iox.Discard(managed)
	assertFails(t, err)
}

// TestPrepareModulePlanDirsReportsMissingSource verifies missing source dirs fail.
func TestPrepareModulePlanDirsReportsMissingSource(t *testing.T) {
	t.Parallel()

	rel, err := prepareModulePlanDirs(&modulePlanDirsInput{
		syncInput: minimalSyncInput(t.TempDir()),
		mod:       emptyModulePtr(consts.Go, consts.Go),
		sourceDir: filepath.Join(t.TempDir(), pathMissing),
	})

	iox.Discard(rel)
	assertFails(t, err)
}

// TestReadAndMaybeRewriteModuleFileReportsRewriteFailure verifies rewrite failures.
func TestReadAndMaybeRewriteModuleFileReportsRewriteFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, rootTaskfileName), []byte(byteX))

	data, err := readAndMaybeRewriteModuleFile(
		&rewriteModuleArgs{
			sourceToDest: nil,
			fromDest:     consts.Empty,
			ops:          failingOps{},
			sourceDir:    root,
			rel:          rootTaskfileName,
			absPath:      rootTaskfileName,
		},
	)

	iox.Discard(data)
	assertFails(t, err)
}

// TestReadRootPlanFinishInputReportsFailure verifies readRootTaskfile failures surface.
func TestReadRootPlanFinishInputReportsFailure(t *testing.T) {
	t.Parallel()

	input, err := readRootPlanFinishInput(
		&buildRootPlanInput{
			oldLock:        nil,
			moduleContents: nil,
			syncInput:      minimalSyncInput(t.TempDir()),
		},
	)

	iox.Discard(input)
	assertFails(t, err)
}

// TestRemoveObsoleteReportsOldTargetFailure verifies cleanupOldTarget failures surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveObsoleteReportsOldTargetFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = emptyLockPtr()
	assertFails(t, removeObsolete(plan, t.TempDir()))
}

// TestRemoveOldTargetFilesSkipsOutsidePrefix verifies unrelated paths are skipped.
func TestRemoveOldTargetFilesSkipsOutsidePrefix(t *testing.T) {
	t.Parallel()

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = lockWithManaged(goOldTxtRel)
	assertNoErr(t, removeOldTargetFiles(plan, t.TempDir()))
}

// TestWalkCollectModuleFileReportsCollectFailure verifies walked collection failures surface.
func TestWalkCollectModuleFileReportsCollectFailure(t *testing.T) {
	t.Parallel()

	err := walkCollectModuleFile(&walkCollectArgs{
		opts: &collectOptions{
			sourceDir:    srcDir,
			ops:          nil,
			sourceToDest: nil,
			fromDest:     consts.Empty,
			docPolicy:    0,
		},
		contents: fMap{},
		absPath:  srcFileA,
		entry:    fakeDirEntry{name: pathA, dir: false},
	})

	assertFails(t, err)
}

func distinctDocCollectArgs(t *testing.T) *collectModuleArgs {
	t.Helper()

	workspace := t.TempDir()
	source := filepath.Join(workspace, srcModuleName)
	assertNoErr(t, mkdirAll(source, dirModePerm))

	args := sampleCollectArgs(source)

	args.syncInput.Config.IncludesDoc = true
	args.syncInput.Snapshot = stubSnapshot{root: workspace}
	args.mod = emptyModulePtr(srcModuleName, consts.Go)

	return args
}

func failingRootSyncInput(t *testing.T) *domain.SyncInput {
	t.Helper()

	root := t.TempDir()
	assertNoErr(t, mkdirAll(filepath.Join(root, taskfilesDirName), dirModePerm))

	return &domain.SyncInput{
		Config:      emptyConfigPtr(),
		Snapshot:    stubSnapshot{root: root},
		TaskfileOps: failingOps{},
		Requested: map[string]moduleRecord{
			consts.Go: {
				SourceModule:      consts.Go,
				DestinationModule: consts.Empty,
				Path:              consts.Empty,
			},
		},
		DestByTask:   map[string]string{consts.Go: consts.Go},
		SourceToDest: map[string]string{consts.Go: consts.Go},
	}
}

func goModuleSyncInput(workspace, storeRoot string) *domain.SyncInput {
	input := minimalSyncInput(workspace)

	input.Snapshot = dirSnapshot{root: storeRoot}
	input.Requested = map[string]moduleRecord{
		consts.Go: {
			SourceModule: consts.Go, DestinationModule: consts.Go, Path: goDestPath,
		},
	}

	return input
}

func managedGoLock() *syncLock {
	lock := lockWithManaged(goTaskfileRel)

	lock.Configuration.TargetFolder = config.DefaultTargetFolder

	return lock
}

func prepFinalizeFailDirs(t *testing.T) (workspace, storeRoot string) {
	t.Helper()

	workspace = t.TempDir()
	storeRoot = t.TempDir()

	srcTask := filepath.Join(storeRoot, consts.Go, rootTaskfileName)
	assertNoErr(t, mkdirAll(filepath.Dir(srcTask), dirModePerm))

	writeTempFile(t, srcTask, []byte(emptyTaskYAML))
	assertNoErr(
		t,
		mkdirAll(filepath.Join(workspace, filepath.FromSlash(goTaskfileRel)), dirModePerm),
	)

	return workspace, storeRoot
}

func sampleCollectArgs(sourceDir string) *collectModuleArgs {
	return &collectModuleArgs{
		syncInput:  syncInputWithConfig(),
		mod:        emptyModulePtr(consts.Go, consts.Go),
		sourceDir:  sourceDir,
		destDirRel: config.DefaultTargetFolder + "/" + consts.Go,
	}
}

func swapWalkThenFail(t *testing.T) {
	t.Helper()

	original := walkDir
	calls := consts.IndexZero

	walkDir = func(root string, walker fs.WalkDirFunc) error {
		calls++

		if calls == consts.IndexOne {
			return original(root, walker)
		}

		return errStub
	}

	t.Cleanup(func() { walkDir = original })
}

func writeValidLock(t *testing.T, workspace, targetFolder string) {
	t.Helper()

	var lock syncLock

	lock.Configuration.TargetFolder = targetFolder

	path := filepath.Join(workspace, filepath.FromSlash(lockRelPath))
	assertNoErr(t, mkdirAll(filepath.Dir(path), dirModePerm))

	writeTempFile(t, path, MarshalLock(&lock))
}

func (dirSnapshot) DefaultBranch() string { return consts.Empty }
func (snap dirSnapshot) ModuleDir(name string) string {
	return filepath.Join(snap.root, name)
}

func (dirSnapshot) ResolvedCommit() string     { return consts.Empty }
func (dirSnapshot) SourceRef() string          { return consts.Empty }
func (snap dirSnapshot) WorkspaceRoot() string { return snap.root }
