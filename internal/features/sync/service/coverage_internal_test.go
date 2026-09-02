// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:exhaustruct_v5 // test fixtures only set fields exercised by the unit
package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	failingOps struct{}
)

// TestApplyFileChangeAddedAndUpdated verifies added and updated kinds append paths.
func TestApplyFileChangeAddedAndUpdated(t *testing.T) {
	t.Parallel()

	added := applyFileChange(
		&diffLists{added: nil, updated: nil, removed: nil},
		pathA,
		fileAdded,
	)
	updated := applyFileChange(
		&diffLists{added: nil, updated: nil, removed: nil},
		pathB,
		fileUpdated,
	)

	if len(added.added) != consts.IndexOne || len(updated.updated) != consts.IndexOne {
		t.Fatalf("added=%v updated=%v", added, updated)
	}
}

// TestApplyPlanWithCleanupReportsSessionFailure verifies session start failures surface.
//
//nolint:paralleltest // swaps the package-level mkdirAll seam
func TestApplyPlanWithCleanupReportsSessionFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	assertFails(t, applyPlanWithCleanup(minimalPlan(), minimalSyncInput(t.TempDir())))
}

// TestApplyStagedPlanReportsCleanupFailure verifies post-write cleanup failures surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestApplyStagedPlanReportsCleanupFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldLock = lockWithManaged(goOldTxtRel)
	plan.OldLock.Configuration.TargetFolder = config.DefaultTargetFolder

	err := applyStagedPlan(&applyStagedInput{
		plan:      plan,
		syncInput: minimalSyncInput(t.TempDir()),
		workspace: t.TempDir(),
		session:   newStagingSession(copyFileTo),
	})

	assertFails(t, err)
}

// TestBuildFileEntryReportsReadFailure verifies missing module files fail after Info.
func TestBuildFileEntryReportsReadFailure(t *testing.T) {
	t.Parallel()

	entry, err := buildFileEntry(missingFileEntryArgs(t))
	iox.Discard(entry)
	assertFails(t, err)
}

// TestCleanupAfterApplyReportsLegacyFailure verifies legacy cleanup failures surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestCleanupAfterApplyReportsLegacyFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, cleanupAfterApply(minimalPlan(), t.TempDir(), metaRelPath))
}

// TestCleanupAfterApplyReportsObsoleteFailure verifies obsolete cleanup failures surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestCleanupAfterApplyReportsObsoleteFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldLock = lockWithManaged(goOldTxtRel)
	plan.OldLock.Configuration.TargetFolder = config.DefaultTargetFolder
	assertFails(t, cleanupAfterApply(plan, t.TempDir(), metaRelPath))
}

// TestCleanupLegacyMetadataReportsDirFailure verifies dir cleanup failures surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestCleanupLegacyMetadataReportsDirFailure(t *testing.T) {
	calls := consts.IndexZero

	swapRemovePath(t, func(string) error {
		calls++

		if calls == consts.IndexOne {
			return os.ErrNotExist
		}

		return errStub
	})

	assertFails(t, cleanupLegacyMetadata(t.TempDir(), metaRelPath))
}

// TestCleanupLegacyMetadataReportsFileFailure verifies legacy file remove failures.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestCleanupLegacyMetadataReportsFileFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, cleanupLegacyMetadata(t.TempDir(), metaRelPath))
}

// TestCleanupOldTargetReportsStepFailure verifies old-target cleanup failures surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestCleanupOldTargetReportsStepFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = lockWithManaged(oldTargetFileRel)
	assertFails(t, removeOldTargetFiles(plan, t.TempDir()))
}

// TestRemoveOldTargetLockReportsFailure verifies old lock remove failures surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveOldTargetLockReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	assertFails(t, removeOldTargetLock(plan, t.TempDir()))
}

// TestRemoveOldTargetMetadataReportsDirFailure verifies old metadata dir prune failures.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveOldTargetMetadataReportsDirFailure(t *testing.T) {
	calls := consts.IndexZero

	swapRemovePath(t, func(string) error {
		calls++

		if calls == consts.IndexOne {
			return os.ErrNotExist
		}

		return errStub
	})

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	assertFails(t, removeOldTargetMetadata(plan, t.TempDir()))
}

// TestRemoveOldTargetMetadataReportsFailure verifies old metadata remove failures.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveOldTargetMetadataReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	assertFails(t, removeOldTargetMetadata(plan, t.TempDir()))
}

// TestRemoveStaleManagedFileReportsPruneFailure verifies prune failures after remove surface.
//
//nolint:paralleltest // swaps a package-level FS seam
func TestRemoveStaleManagedFileReportsPruneFailure(t *testing.T) {
	workspace := prepGoSubDir(t)
	swapRemovePath(t, succeedThenFail())

	assertFails(t, removeStaleManagedFile(&removeStaleFileArgs{
		old: &managedFile{
			SourceModule:      consts.Empty,
			Path:              goSubOldRel,
			DestinationModule: consts.Go,
		},
		current:      map[string]struct{}{},
		workspace:    workspace,
		targetFolder: config.DefaultTargetFolder,
	}))
}

// TestRemoveStaleManagedFilesReportsFailure verifies first stale remove failure surfaces.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveStaleManagedFilesReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(goOldTxtRel, consts.Go, consts.Empty)}
	lock.Configuration.TargetFolder = config.DefaultTargetFolder
	assertFails(t, removeStaleManagedFiles(&lock, map[string]struct{}{}, t.TempDir()))
}

// TestScanLogicalRootDocsReportsWalkFailure verifies logical-root scan failures.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestScanLogicalRootDocsReportsWalkFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	var args mergeParentDocsArgs

	args.destRoot = t.TempDir()
	args.collect = &collectModuleArgs{
		syncInput: syncInputWithConfig(),
		mod:       emptyModulePtr(consts.Empty, consts.Go),
	}

	contents, err := scanLogicalRootDocs(&args)
	iox.Discard(contents)
	assertFails(t, err)
}

// TestStagePreparedFilesReportsFailure verifies staging failures surface.
//
//nolint:paralleltest // swaps the package-level mkdirAll seam
func TestStagePreparedFilesReportsFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	session, err := stagePreparedFiles(&stagePreparedInput{
		staged:    nil,
		plan:      minimalPlan(),
		syncInput: minimalSyncInput(t.TempDir()),
		workspace: t.TempDir(),
	})

	iox.Discard(session)
	assertFails(t, err)
}

// TestStartApplySessionReportsStagingFailure verifies prepareStaging failures surface.
//
//nolint:paralleltest // swaps the package-level mkdirAll seam
func TestStartApplySessionReportsStagingFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	session, err := startApplySession(minimalPlan(), minimalSyncInput(t.TempDir()))
	iox.Discard(session)
	assertFails(t, err)
}

// TestStoreCollectedModuleFileReportsFailure verifies buildFileEntry failures surface.
func TestStoreCollectedModuleFileReportsFailure(t *testing.T) {
	t.Parallel()

	err := storeCollectedModuleFile(&moduleCollectArgs{
		ops:          nil,
		sourceToDest: nil,
		sourceDir:    consts.Empty,
		fromDest:     consts.Empty,
		docPolicy:    0,
		entry:        fakeDirEntry{name: fileNameTxt, dir: false},
		absPath:      fileNameTxt,
		contents:     fMap{},
	}, fileNameTxt)
	assertFails(t, err)
}

// TestTryLegacyMetadataReportsCorrupt verifies corrupt legacy metadata fails.
func TestTryLegacyMetadataReportsCorrupt(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, filepath.FromSlash(config.LegacyMetadataPath))
	assertNoErr(t, mkdirAll(filepath.Dir(path), dirModePerm))

	writeTempFile(t, path, []byte(badYAMLText))

	meta, found, err := tryLegacyMetadata(workspace, metaRelPath)
	iox.Discard(meta)
	iox.Discard(found)
	assertFails(t, err)
}

// TestUpdateRootTaskfileReportsOpsFailure verifies TaskfileOps update failures surface.
func TestUpdateRootTaskfileReportsOpsFailure(t *testing.T) {
	t.Parallel()

	syncIn := emptySyncInput()
	cfg := emptyConfig()

	syncIn.Config = &cfg
	syncIn.TaskfileOps = failingOps{}

	root, tasks, err := updateRootTaskfile(&updateRootArgs{
		args: buildRootArgs{
			syncInput:      &syncIn,
			moduleContents: nil,
			oldLock:        nil,
		},
	})

	iox.Discard(root)
	iox.Discard(tasks)
	assertFails(t, err)
}

func (failingOps) NewRootTemplate() []byte { return nil }

func (failingOps) RewriteIncludes([]byte, map[string]string, string) ([]byte, error) {
	return nil, errStub
}

func (failingOps) UpdateRootTaskfile([]byte, *rootupd.RootUpdateInput) ([]byte, error) {
	return nil, errStub
}

func minimalPlan() *domain.Plan {
	plan := emptyPlan()

	plan.Metadata = emptyMetadata(lockRelPath)

	return &plan
}

func minimalSyncInput(workspace string) *domain.SyncInput {
	input := emptySyncInput()
	cfg := emptyConfig()

	cfg.Workspace = workspace
	cfg.TargetFolder = config.DefaultTargetFolder
	cfg.RootTaskfile = rootTaskfileName
	input.Config = &cfg

	return &input
}
