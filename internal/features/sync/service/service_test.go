// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/testsupport"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
	yaml "go.yaml.in/yaml/v3"
)

// TestApplyFileChangeDefaultIsNoOp verifies unknown change kinds leave lists untouched.
func TestApplyFileChangeDefaultIsNoOp(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	lists := applyFileChange(
		&diffLists{added: nil, updated: nil, removed: nil},
		fileNameTxt,
		fileChangeKind(unknownFileChange),
	)

	if len(lists.added)+len(lists.updated) != consts.IndexZero {
		t.Fatalf(listsFmt, lists)
	}
}

// TestCleanupFailedStagingJoinsRemoveError verifies cleanup joins remove failures.
func TestCleanupFailedStagingJoinsRemoveError(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemoveAll(t, failingRemove)

	err := cleanupFailedStaging(stagingName, errStub)

	if !errors.Is(err, errStub) {
		t.Fatalf(errWantFmt, err, errStub)
	}
}

// TestCleanupFailedStagingReturnsCopyError verifies successful cleanup keeps copyErr.
func TestCleanupFailedStagingReturnsCopyError(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := cleanupFailedStaging(t.TempDir(), errStub)

	if !errors.Is(err, errStub) {
		t.Fatalf(errWantFmt, err, errStub)
	}
}

// TestCleanupLegacyMetadataSkipsLegacyPath verifies legacy metadata path is left alone.
func TestCleanupLegacyMetadataSkipsLegacyPath(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	assertNoErr(t, cleanupLegacyMetadata(t.TempDir(), config.LegacyMetadataPath))
}

// TestCleanupStagingDirReportsRemoveFailure verifies staging cleanup surfaces remove errors.
func TestCleanupStagingDirReportsRemoveFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemoveAll(t, failingRemove)

	assertFails(t, cleanupStagingDir(stagingName))
}

// TestCleanupStagingOnExitKeepsPrimaryError verifies cleanup errors do not clobber primary.
func TestCleanupStagingOnExitKeepsPrimaryError(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemoveAll(t, failingRemove)

	primary := errStub
	cleanupStagingOnExit(stagingName, &primary)

	if !errors.Is(primary, errStub) {
		t.Fatalf("err = %v, want primary", primary)
	}
}

// TestCleanupStagingOnExitSurfacesCleanupError verifies nil primary adopts cleanup error.
func TestCleanupStagingOnExitSurfacesCleanupError(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemoveAll(t, failingRemove)

	var err error

	cleanupStagingOnExit(stagingName, &err)
	assertFails(t, err)
}

// TestCopyStagedFilesReportsFailure verifies staging copy failures surface.
func TestCopyStagedFilesReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := copyStagedFiles(t.TempDir(), []stagedFile{{
		finalRel: fileNameTxt,
		entry:    domain.FileEntry{Data: []byte(byteX), Mode: fileModeRegular},
	}}, func(string, *domain.FileEntry) error { return errStub })
	assertFails(t, err)
}

// TestFileChangeFromDataDetectsUpdate verifies mismatched hashes mark updates.
func TestFileChangeFromDataDetectsUpdate(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	kind := fileChangeFromData(
		[]byte(pathA),
		managedAtPtr(consts.Empty, consts.Empty, "deadbeef"),
	)

	if kind != fileUpdated {
		t.Fatalf("kind = %v, want updated", kind)
	}
}

// TestPrepareStagingRootReportsMkdirFailure verifies parent mkdir failures surface.
func TestPrepareStagingRootReportsMkdirFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapMkdirAll(t, failingMkdirAll)

	root, err := prepareStagingRoot(t.TempDir(), config.DefaultTargetFolder)
	iox.Discard(root)
	assertFails(t, err)
}

// TestPrepareStagingRootReportsTempFailure verifies MkdirTemp failures surface.
func TestPrepareStagingRootReportsTempFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapMkdirTemp(t, failingMkdirTemp)

	root, err := prepareStagingRoot(t.TempDir(), config.DefaultTargetFolder)
	iox.Discard(root)
	assertFails(t, err)
}

// TestPruneDirsUntilStopReportsFailure verifies prune stops on remove errors.
func TestPruneDirsUntilStopReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	workspace := t.TempDir()
	nested := filepath.Join(workspace, pathA, pathB)
	assertNoErr(t, os.MkdirAll(nested, dirModePerm))

	assertFails(t, pruneDirsUntilStop(nested, workspace))
}

// TestPruneEmptyParentDirsSkipsEmptyStop verifies empty stopRel is a no-op.
func TestPruneEmptyParentDirsSkipsEmptyStop(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	assertNoErr(t, pruneEmptyParentDirs(t.TempDir(), fileNameTxt, consts.Empty))
}

// TestRemoveDirIfEmptyIgnoresNotEmpty verifies ENOTEMPTY is ignored.
func TestRemoveDirIfEmptyIgnoresNotEmpty(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, func(string) error { return syscall.ENOTEMPTY })

	assertNoErr(t, removeDirIfEmpty(stagingName, removeEmptyCtx))
}

// TestRemoveDirIfEmptyReportsFailure verifies unexpected remove errors surface.
func TestRemoveDirIfEmptyReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	assertFails(t, removeDirIfEmpty(stagingName, removeEmptyCtx))
}

// TestRemoveEmptyParentDirReportsFailure verifies prune propagates remove failures.
func TestRemoveEmptyParentDirReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	exists, err := removeEmptyParentDir(stagingName)
	iox.Discard(exists)
	assertFails(t, err)
}

// TestRemoveIfExistsReportsFailure verifies unexpected remove errors surface.
func TestRemoveIfExistsReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	assertFails(t, removeIfExists(t.TempDir(), fileNameTxt))
}

// TestRemoveLegacyMetadataDirReportsFailure verifies legacy dir remove failures.
func TestRemoveLegacyMetadataDirReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	assertFails(t, removeLegacyMetadataDir(t.TempDir()))
}

// TestRemoveLegacyMetadataFileReportsFailure verifies legacy metadata remove failures.
func TestRemoveLegacyMetadataFileReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	assertFails(t, removeLegacyMetadataFile(t.TempDir()))
}

// TestRemoveObsoleteFileReportsFailure verifies unexpected remove errors surface.
func TestRemoveObsoleteFileReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	assertFails(t, removeObsoleteFile(t.TempDir(), fileNameTxt))
}

// TestRemoveStaleManagedFileReportsRemoveFailure verifies stale remove failures surface.
func TestRemoveStaleManagedFileReportsRemoveFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	err := removeStaleManagedFile(&removeStaleFileArgs{
		old: &managedFile{
			Path:              goOldTxtRel,
			DestinationModule: consts.Go,
		},
		current:      map[string]struct{}{},
		workspace:    t.TempDir(),
		targetFolder: config.DefaultTargetFolder,
	})

	assertFails(t, err)
}

// TestRemoveStaleManagedFileSkipsCurrent verifies current paths are kept.
func TestRemoveStaleManagedFileSkipsCurrent(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	path := goTaskfileRel
	err := removeStaleManagedFile(&removeStaleFileArgs{
		old: &managedFile{
			Path:              path,
			SourceModule:      consts.Empty,
			DestinationModule: consts.Empty,
			SourcePath:        consts.Empty,
			SHA256:            consts.Empty,
		},
		current:      map[string]struct{}{path: {}},
		workspace:    t.TempDir(),
		targetFolder: config.DefaultTargetFolder,
	})

	assertNoErr(t, err)
}

// TestStagePlanFilesCleansFailedStaging verifies failed staging invokes cleanupFailedStaging.
func TestStagePlanFilesCleansFailedStaging(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemoveAll(t, failingRemove)

	root, err := stagePlanFiles(&stagePlanArgs{
		staged: []stagedFile{{
			finalRel: fileNameTxt,
			entry:    domain.FileEntry{Data: []byte(byteX), Mode: fileModeRegular},
		}},
		workspace:    t.TempDir(),
		targetFolder: config.DefaultTargetFolder,
		copyFile:     func(string, *domain.FileEntry) error { return errStub },
	})

	iox.Discard(root)
	assertFails(t, err)
}

// TestStagePlanFilesPrepareRootFailure verifies prepareStagingRoot failures surface.
func TestStagePlanFilesPrepareRootFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapMkdirAll(t, failingMkdirAll)

	root, err := stagePlanFiles(&stagePlanArgs{
		staged:       nil,
		workspace:    t.TempDir(),
		targetFolder: config.DefaultTargetFolder,
		copyFile:     copyFileTo,
	})

	iox.Discard(root)
	assertFails(t, err)
}

// TestValidateAndWriteStagedReportsValidateFailure verifies validation errors surface.
func TestValidateAndWriteStagedReportsValidateFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := validateAndWriteStaged(&validateWriteStagedInput{
		args: validateStagedArgs{
			staged: []stagedFile{{
				finalRel: lockFileName,
				entry:    domain.FileEntry{Data: []byte(badYAMLText), Mode: fileModeRegular},
			}},
			rootPath: rootTaskfileName,
		},
		workspace: t.TempDir(),
		copyFile:  copyFileTo,
	})

	assertFails(t, err)
}

// TestValidateGeneratedYAMLReportsFailure verifies the first invalid staged entry fails.
func TestValidateGeneratedYAMLReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	staged := []stagedFile{{
		finalRel: lockFileName,
		entry:    domain.FileEntry{Data: []byte(badYAMLText), Mode: fileModeRegular},
	}}
	assertFails(t, validateGeneratedYAML(staged, rootTaskfileName))
}

// TestValidateStagedYAMLReportsFailure verifies staged YAML validation wraps errors.
func TestValidateStagedYAMLReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := validateStagedYAML(&stagedFile{
		finalRel: lockFileName,
		entry:    domain.FileEntry{Data: []byte(badYAMLText), Mode: fileModeRegular},
	}, rootTaskfileName)
	assertFails(t, err)
}

// TestValidateYAMLReportsFailures verifies lock, metadata, and root validators reject bad YAML.
func TestValidateYAMLReportsFailures(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	bad := []byte(badYAMLText)

	assertFails(t, validateLockFileYAML(bad))

	assertFails(t, validateMetadataYAML(bad))
	assertFails(t, validateRootTaskfileYAML(bad))
}

// TestWriteStagedFilesReportsCopyFailure verifies copy hook failures surface.
func TestWriteStagedFilesReportsCopyFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := writeStagedFiles(&writeStagedArgs{
		staged: []stagedFile{{
			finalRel: fileNameTxt,
			entry:    domain.FileEntry{Data: []byte(byteX), Mode: fileModeRegular},
		}},
		workspace: t.TempDir(),
		copyFile:  func(string, *domain.FileEntry) error { return errStub },
	})

	assertFails(t, err)
}

// TestWriteStagedFilesReportsMkdirFailure verifies destination mkdir failures surface.
func TestWriteStagedFilesReportsMkdirFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapMkdirAll(t, failingMkdirAll)

	err := writeStagedFiles(&writeStagedArgs{
		staged: []stagedFile{{
			finalRel: fileNameTxt,
			entry:    domain.FileEntry{Data: []byte(byteX), Mode: fileModeRegular},
		}},
		workspace: t.TempDir(),
		copyFile:  copyFileTo,
	})

	assertFails(t, err)
}

// TestApplyFileChangeAddedAndUpdated verifies added and updated kinds append paths.
func TestApplyFileChangeAddedAndUpdated(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
func TestApplyPlanWithCleanupReportsSessionFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapMkdirAll(t, failingMkdirAll)

	assertFails(t, applyPlanWithCleanup(minimalPlan(), minimalSyncInput(t.TempDir())))
}

// TestApplyStagedPlanReportsCleanupFailure verifies post-write cleanup failures surface.
func TestApplyStagedPlanReportsCleanupFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
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
	lockSeams(t)

	entry, err := buildFileEntry(missingFileEntryArgs(t))
	iox.Discard(entry)
	assertFails(t, err)
}

// TestCleanupAfterApplyReportsLegacyFailure verifies legacy cleanup failures surface.
func TestCleanupAfterApplyReportsLegacyFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	assertFails(t, cleanupAfterApply(minimalPlan(), t.TempDir(), metaRelPath))
}

// TestCleanupAfterApplyReportsObsoleteFailure verifies obsolete cleanup failures surface.
func TestCleanupAfterApplyReportsObsoleteFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldLock = lockWithManaged(goOldTxtRel)
	plan.OldLock.Configuration.TargetFolder = config.DefaultTargetFolder
	assertFails(t, cleanupAfterApply(plan, t.TempDir(), metaRelPath))
}

// TestCleanupLegacyMetadataReportsDirFailure verifies dir cleanup failures surface.
func TestCleanupLegacyMetadataReportsDirFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
func TestCleanupLegacyMetadataReportsFileFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	assertFails(t, cleanupLegacyMetadata(t.TempDir(), metaRelPath))
}

// TestCleanupOldTargetReportsStepFailure verifies old-target cleanup failures surface.
func TestCleanupOldTargetReportsStepFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = lockWithManaged(oldTargetFileRel)
	assertFails(t, removeOldTargetFiles(plan, t.TempDir()))
}

// TestRemoveOldTargetLockReportsFailure verifies old lock remove failures surface.
func TestRemoveOldTargetLockReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	assertFails(t, removeOldTargetLock(plan, t.TempDir()))
}

// TestRemoveOldTargetMetadataReportsDirFailure verifies old metadata dir prune failures.
func TestRemoveOldTargetMetadataReportsDirFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
func TestRemoveOldTargetMetadataReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	assertFails(t, removeOldTargetMetadata(plan, t.TempDir()))
}

// TestRemoveStaleManagedFileReportsPruneFailure verifies prune failures after remove surface.
func TestRemoveStaleManagedFileReportsPruneFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
func TestRemoveStaleManagedFilesReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(goOldTxtRel, consts.Go, consts.Empty)}
	lock.Configuration.TargetFolder = config.DefaultTargetFolder
	assertFails(t, removeStaleManagedFiles(&lock, map[string]struct{}{}, t.TempDir()))
}

// TestScanLogicalRootDocsReportsWalkFailure verifies logical-root scan failures.
func TestScanLogicalRootDocsReportsWalkFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
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
func TestStagePreparedFilesReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
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
func TestStartApplySessionReportsStagingFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapMkdirAll(t, failingMkdirAll)

	session, err := startApplySession(minimalPlan(), minimalSyncInput(t.TempDir()))
	iox.Discard(session)
	assertFails(t, err)
}

// TestStoreCollectedModuleFileReportsFailure verifies buildFileEntry failures surface.
func TestStoreCollectedModuleFileReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
	lockSeams(t)

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
	lockSeams(t)

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

func (failingOps) UpdateRootTaskfile([]byte, *ports.RootUpdateInput) ([]byte, error) {
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

// TestApplyFileChangeUnchangedIsNoOp verifies unchanged kinds leave lists untouched.
func TestApplyFileChangeUnchangedIsNoOp(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	lists := applyFileChange(
		&diffLists{added: nil, updated: nil, removed: nil},
		fileNameTxt,
		fileUnchanged,
	)

	if len(lists.added)+len(lists.updated) != consts.IndexZero {
		t.Fatalf(listsFmt, lists)
	}
}

// TestDiffLockFileReportsReadFailure verifies lock path read failures surface.
func TestDiffLockFileReportsReadFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	assertNoErr(
		t,
		os.MkdirAll(filepath.Join(workspace, filepath.FromSlash(lockRelPath)), dirModePerm),
	)

	lists, err := diffLockFile(&diffLockArgs{
		plan:      planWithLockMeta(lockRelPath),
		workspace: workspace,
		lockPath:  lockRelPath,
		lists:     diffLists{added: nil, updated: nil, removed: nil},
	})

	iox.Discard(lists)
	assertFails(t, err)
}

// TestDiffManagedFilePathsReportsFailure verifies managed-file read failures surface.
func TestDiffManagedFilePathsReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	path := goTaskfileRel
	assertNoErr(t, os.MkdirAll(filepath.Join(workspace, filepath.FromSlash(path)), dirModePerm))

	lists, err := diffManagedFilePaths(map[string]managedFile{
		path: {
			Path:              path,
			SHA256:            byteX,
			SourceModule:      consts.Empty,
			DestinationModule: consts.Empty,
			SourcePath:        consts.Empty,
		},
	}, workspace)
	iox.Discard(lists)
	assertFails(t, err)
}

// TestDiffMetadataFileReportsReadFailure verifies metadata path read failures surface.
func TestDiffMetadataFileReportsReadFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	assertNoErr(
		t,
		os.MkdirAll(filepath.Join(workspace, filepath.FromSlash(metaRelPath)), dirModePerm),
	)

	lists, err := diffMetadataFile(&diffMetadataArgs{
		workspace:    workspace,
		metadataPath: metaRelPath,
		plannedMeta:  []byte(byteX),
		lists:        diffLists{added: nil, updated: nil, removed: nil},
	})

	iox.Discard(lists)
	assertFails(t, err)
}

// TestFileChangeFromDataUnchanged verifies matching hashes stay unchanged.
func TestFileChangeFromDataUnchanged(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	sum := "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"
	kind := fileChangeFromData(
		[]byte(pathA),
		managedAtPtr(consts.Empty, consts.Empty, sum),
	)

	if kind != fileUnchanged {
		t.Fatalf("kind = %v, want unchanged", kind)
	}
}

// TestFileChangedReportsReadFailure verifies non-missing read errors surface.
func TestFileChangedReportsReadFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	path := goTaskfileRel
	assertNoErr(t, os.MkdirAll(filepath.Join(workspace, filepath.FromSlash(path)), dirModePerm))

	kind, err := fileChanged(
		workspace,
		path,
		managedAtPtr(consts.Empty, consts.Empty, byteX),
	)
	iox.Discard(kind)
	assertFails(t, err)
}

// TestLockContentChangedDetectsDifference verifies lock payloads are compared.
func TestLockContentChangedDetectsDifference(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	oldLock := emptyLockPtr()

	oldLock.Configuration.TargetFolder = config.DefaultTargetFolder

	newLock := emptyLockPtr()

	newLock.Configuration.TargetFolder = "other"

	changed, err := lockContentChanged(oldLock, newLock)
	assertNoErr(t, err)

	if !changed {
		t.Fatal("expected lock content change")
	}
}

// TestMarshalLockForCompareNilReturnsNil verifies nil locks marshal to nil bytes.
func TestMarshalLockForCompareNilReturnsNil(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	data, err := marshalLockForCompare(nil)
	assertNoErr(t, err)

	if data != nil {
		t.Fatalf("data = %v, want nil", data)
	}
}

// TestCleanupTempFileRunsWhenFlagged verifies cleanup closes and removes temps.
func TestCleanupTempFileRunsWhenFlagged(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	tmp := createRealTemp(t)
	path := tmp.Name()
	cleanup := true

	cleanupTempFile(tmp, path, &cleanup)

	info, err := os.Stat(path)
	iox.Discard(info)

	if !os.IsNotExist(err) {
		t.Fatalf("temp still present: %v", err)
	}
}

// TestCopyFileCopiesRelativeSource verifies CopyFile reads and writes successfully.
func TestCopyFileCopiesRelativeSource(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(payloadText))

	dst := filepath.Join(t.TempDir(), "out.txt")
	assertNoErr(t, CopyFile(&copyFileArgs{
		root: root, rel: fileNameTxt, dst: dst, mode: fileModeRegular,
	}))
	assertFilePayload(t, dst, payloadText)
}

// TestCopyFileReportsMissingSource verifies missing sources fail.
func TestCopyFileReportsMissingSource(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := CopyFile(&copyFileArgs{
		root: t.TempDir(), rel: fileNameTxt, dst: filepath.Join(t.TempDir(), outName),
		mode: fileModeRegular,
	})

	assertFails(t, err)
}

// TestCopyFileReportsWriteFailure verifies destination write failures surface.
func TestCopyFileReportsWriteFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(byteX))
	swapMkdirAll(t, failingMkdirAll)

	err := CopyFile(&copyFileArgs{
		root: root, rel: fileNameTxt, dst: filepath.Join(t.TempDir(), outName),
		mode: fileModeRegular,
	})

	assertFails(t, err)
}

// TestCopyFileToReportsWriteFailure verifies copyFileTo wraps write failures.
func TestCopyFileToReportsWriteFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapMkdirAll(t, failingMkdirAll)

	err := copyFileTo(filepath.Join(t.TempDir(), pathA, fileNameTxt), &domain.FileEntry{
		Data: []byte(byteX), Mode: fileModeRegular,
	})

	assertFails(t, err)
}

// TestCreateTempFileReportsCreateFailure verifies CreateTemp failures surface.
func TestCreateTempFileReportsCreateFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapCreateTemp(t, failingCreateTemp)

	file, err := createTempFile(t.TempDir())
	discardTemp(file)
	assertFails(t, err)
}

// TestCreateTempFileReportsMkdirFailure verifies parent mkdir failures surface.
func TestCreateTempFileReportsMkdirFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapMkdirAll(t, failingMkdirAll)

	file, err := createTempFile(filepath.Join(t.TempDir(), "nested"))
	discardTemp(file)
	assertFails(t, err)
}

// TestReadRelativeFileReportsOpenFailure verifies open failures surface.
func TestReadRelativeFileReportsOpenFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapOpenRelative(t, failingOpenRelative)

	data, err := readRelativeFile(t.TempDir(), fileNameTxt)
	iox.Discard(data)
	assertFails(t, err)
}

// TestReadRelativeFileReportsReadFailure verifies read failures surface.
func TestReadRelativeFileReportsReadFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(byteX))
	swapReadAll(t, failingReadAll)

	data, err := readRelativeFile(root, fileNameTxt)
	iox.Discard(data)
	assertFails(t, err)
}

// TestRenameTempFileReportsFailure verifies rename failures surface.
func TestRenameTempFileReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRenamePath(t, failingRename)

	assertFails(t, renameTempFile(pathA, pathB))
}

// TestWriteAndFinalizeTempReportsChmodFailure verifies chmod failures surface.
func TestWriteAndFinalizeTempReportsChmodFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapChmodFile(t, failingChmod)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
}

// TestWriteAndFinalizeTempReportsCloseFailure verifies close failures surface.
func TestWriteAndFinalizeTempReportsCloseFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapCloseFile(t, failingClose)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
}

// TestWriteAndFinalizeTempReportsWriteFailure verifies writeFull failures surface.
func TestWriteAndFinalizeTempReportsWriteFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWriteFull(t, failingWriteFull)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
}

// TestWriteFileAtomicReportsFinalizeFailure verifies rename failures abort atomic writes.
func TestWriteFileAtomicReportsFinalizeFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRenamePath(t, failingRename)

	path := filepath.Join(t.TempDir(), fileNameTxt)
	assertFails(t, writeFileAtomic(path, []byte(byteX), fileModeRegular))
}

// TestWriteFullStubWriterUsedDocumentsFaults verifies StubWriter stays referenced.
func TestWriteFullStubWriterUsedDocumentsFaults(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	writer := &faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault}
	assertFails(t, writeFull(writer, []byte(byteX)))
}

func assertFilePayload(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	assertNoErr(t, err)

	if string(data) != want {
		t.Fatalf("payload = %q, want %q", data, want)
	}
}

func createRealTemp(t *testing.T) *os.File {
	t.Helper()

	tmp, err := os.CreateTemp(t.TempDir(), stagingTempPattern)
	assertNoErr(t, err)

	t.Cleanup(func() { iox.Discard(os.Remove(tmp.Name())) })

	return tmp
}

func discardTemp(file *os.File) {
	if file == nil {
		return
	}

	iox.Discard(file.Close())
}

func writeTempFile(t *testing.T, path string, data []byte) {
	t.Helper()

	assertNoErr(t, os.WriteFile(path, data, fileModeRegular))
}

// TestBuildPlanFromStateReportsFinalizeFailure verifies finalizeBuiltPlan failures surface.
func TestBuildPlanFromStateReportsFinalizeFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
	lockSeams(t)

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
	lockSeams(t)

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
	lockSeams(t)

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
func TestCollectAndTrackModuleFilesReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWalkDir(t, failingWalk)

	contents, managed, err := collectAndTrackModuleFiles(sampleCollectArgs(t.TempDir()))
	iox.Discard(contents)
	iox.Discard(managed)
	assertFails(t, err)
}

// TestCollectModuleContentsReportsMergeFailure verifies mergeLogicalRootDocs failures.
func TestCollectModuleContentsReportsMergeFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	args := distinctDocCollectArgs(t)
	swapWalkThenFail(t)

	contents, docs, err := collectModuleContents(args)
	iox.Discard(contents)
	iox.Discard(docs)
	assertFails(t, err)
}

// TestCollectModuleContentsReportsScanFailure verifies scanModuleFiles failures surface.
func TestCollectModuleContentsReportsScanFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWalkDir(t, failingWalk)

	contents, docs, err := collectModuleContents(sampleCollectArgs(t.TempDir()))
	iox.Discard(contents)
	iox.Discard(docs)
	assertFails(t, err)
}

// TestCollectModuleFileReportsStoreFailure verifies storeCollectedModuleFile failures.
func TestCollectModuleFileReportsStoreFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
	lockSeams(t)

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
func TestDiffLockFileReportsContentFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
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
func TestDiscoverPreviousMetadataReportsWalkFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWalkDir(t, failingWalk)

	meta, err := discoverPreviousMetadata(t.TempDir(), metaRelPath)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestFinalizeBuiltPlanReportsDiffFailure verifies finalizePlanDiff failures surface.
func TestFinalizeBuiltPlanReportsDiffFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := prepFinalizeDiffWorkspace(t)
	out, err := finalizeBuiltPlan(finalizeDiffFailInput(workspace))
	iox.Discard(out)
	assertFails(t, err)
}

// TestLoadPreviousLockUsesLockTarget verifies empty metadata target falls back to lock.
func TestLoadPreviousLockUsesLockTarget(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
func TestMergeLogicalRootDocsReportsMergeFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	args := distinctDocCollectArgs(t)
	assertNoErr(t, mkdirAll(args.syncInput.Snapshot.ModuleDir(consts.Go), dirModePerm))

	swapWalkDir(t, failingWalk)

	docs, err := mergeLogicalRootDocs(args, fMap{}, docPolicyInclude)
	iox.Discard(docs)
	assertFails(t, err)
}

// TestMergeParentDocsIfDistinctReportsFailure verifies mergeParentDocFiles failures.
func TestMergeParentDocsIfDistinctReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	args := distinctDocCollectArgs(t)
	assertNoErr(t, mkdirAll(args.syncInput.Snapshot.ModuleDir(consts.Go), dirModePerm))

	swapWalkDir(t, failingWalk)

	docs, err := mergeParentDocsIfDistinct(args, fMap{}, map[string]struct{}{})
	iox.Discard(docs)
	assertFails(t, err)
}

// TestPlanModuleFilesReportsCollectFailure verifies collectAndTrackModuleFiles failures.
func TestPlanModuleFilesReportsCollectFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
	lockSeams(t)

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
	lockSeams(t)

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
	lockSeams(t)

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
func TestRemoveObsoleteReportsOldTargetFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = emptyLockPtr()
	assertFails(t, removeObsolete(plan, t.TempDir()))
}

// TestRemoveOldTargetFilesSkipsOutsidePrefix verifies unrelated paths are skipped.
func TestRemoveOldTargetFilesSkipsOutsidePrefix(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = lockWithManaged(goOldTxtRel)
	assertNoErr(t, removeOldTargetFiles(plan, t.TempDir()))
}

// TestWalkCollectModuleFileReportsCollectFailure verifies walked collection failures surface.
func TestWalkCollectModuleFileReportsCollectFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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

func (dirSnapshot) ResolvedCommit() string { return consts.Empty }

func (dirSnapshot) SourceRef() string { return consts.Empty }

func (snap dirSnapshot) WorkspaceRoot() string { return snap.root }

func emptyLock() syncLock {
	var lock syncLock

	return lock
}

func emptyLockPtr() *syncLock {
	lock := emptyLock()

	return &lock
}

func emptyMetadata(lockFile string) domain.Metadata {
	var meta domain.Metadata

	meta.LockFile = lockFile

	return meta
}

func emptyMetadataPtr(lockFile string) *domain.Metadata {
	meta := emptyMetadata(lockFile)

	return &meta
}

func emptyModule(source, dest string) moduleRecord {
	var mod moduleRecord

	mod.SourceModule = source
	mod.DestinationModule = dest

	return mod
}

func emptyModulePtr(source, dest string) *moduleRecord {
	mod := emptyModule(source, dest)

	return &mod
}

func managedAt(path, dest, sha string) managedFile {
	var file managedFile

	file.Path = path
	file.DestinationModule = dest
	file.SHA256 = sha

	return file
}

func managedAtPtr(path, dest, sha string) *managedFile {
	file := managedAt(path, dest, sha)

	return &file
}

func emptyConfig() config.Config {
	var cfg config.Config

	return cfg
}

func emptyConfigPtr() *config.Config {
	cfg := emptyConfig()

	return &cfg
}

func emptySyncInput() domain.SyncInput {
	var input domain.SyncInput

	return input
}

func emptySyncInputPtr() *domain.SyncInput {
	input := emptySyncInput()

	return &input
}

func emptyPlan() domain.Plan {
	var plan domain.Plan

	return plan
}

func planWithLockMeta(lockFile string) *domain.Plan {
	plan := emptyPlan()

	plan.Metadata = emptyMetadata(lockFile)

	return &plan
}

func syncInputWithConfig() *domain.SyncInput {
	syncIn := emptySyncInput()
	cfg := emptyConfig()

	syncIn.Config = &cfg

	return &syncIn
}

func lockWithManaged(path string) *syncLock {
	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(path, consts.Go, consts.Empty)}

	return &lock
}

func newStagingSession(copyFile func(string, *domain.FileEntry) error) stagingSession {
	var session stagingSession

	session.copyFile = copyFile

	return session
}

func zeroDiffLists() *diffLists {
	var lists diffLists

	return &lists
}

func newTempEntry(t *testing.T) tempEntry {
	t.Helper()

	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(byteX))

	entries, err := os.ReadDir(root)
	assertNoErr(t, err)

	return tempEntry{entry: entries[consts.IndexZero], root: root}
}

func missingFileEntryArgs(t *testing.T) *fileEntryArgs {
	t.Helper()

	temp := newTempEntry(t)

	var args fileEntryArgs

	args.entry = temp.entry
	args.sourceDir = temp.root
	args.rel = missingTxt
	args.absPath = missingTxt

	return &args
}

func goModulePlanArgs(workspace string) *modulePlanArgs {
	var args modulePlanArgs

	var syncIn domain.SyncInput

	syncIn.Config = emptyConfigPtr()
	syncIn.Snapshot = dirSnapshot{root: workspace}

	args.syncInput = &syncIn
	args.mod = emptyModulePtr(consts.Go, consts.Go)
	args.moduleContents = mcMap{}

	return &args
}

func goTaskMetadataInput(want *storeTaskMetadata) *groupModulesInput {
	var input groupModulesInput

	input.requestedRecords = map[string]moduleRecord{
		consts.Go: emptyModule(consts.Go, consts.Empty),
	}
	input.metadata = storeTaskMetaMap{consts.Go: *want}

	return &input
}

func sortedManagedFixture() []managedFile {
	planned := []managedFile{
		managedAt(pathB, consts.Empty, consts.Empty),
		managedAt(pathA, consts.Empty, consts.Empty),
		managedAt(pathA, consts.Empty, consts.Empty),
	}

	planned[consts.IndexZero].SourceModule = "mod-z"
	planned[consts.IndexOne].SourceModule = "mod-y"
	planned[consts.IndexTwo].SourceModule = modXName

	return planned
}

func prepFinalizeDiffWorkspace(t *testing.T) string {
	t.Helper()

	workspace := t.TempDir()
	assertNoErr(
		t,
		mkdirAll(filepath.Join(workspace, filepath.FromSlash(goTaskfileRel)), dirModePerm),
	)

	return workspace
}

func finalizeDiffFailInput(workspace string) *finalizeBuiltPlanInput {
	plan := emptyPlan()

	plan.ManagedFiles = []managedFile{managedAt(goTaskfileRel, consts.Empty, byteX)}
	plan.Metadata = emptyMetadata(lockRelPath)

	var artifacts planArtifacts

	return &finalizeBuiltPlanInput{
		syncInput: minimalSyncInput(workspace),
		plan:      &plan,
		meta:      emptyMetadataPtr(lockRelPath),
		artifacts: &artifacts,
	}
}

func assertCandidateRelFails(
	t *testing.T,
	call func(*metadataCandidateArgs) (string, metadataScanResult, error),
) {
	t.Helper()

	swapRelPath(t, failingRelPath)

	rel, scan, err := call(&metadataCandidateArgs{
		workspace:           wsRoot,
		currentMetadataPath: metaRelPath,
		abs:                 wsFileX,
		entry:               fakeDirEntry{name: fileNameTxt, dir: false},
	})

	iox.Discard(rel)
	iox.Discard(scan)
	assertFails(t, err)
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf(unexpectFmt, err)
	}
}

func failingChmod(*os.File, os.FileMode) error { return errStub }

func failingClose(*os.File) error { return errStub }

func failingCreateTemp(string, string) (*os.File, error) {
	return nil, errStub
}

func failingMarshalYAML(any) ([]byte, error) { return nil, errStub }

func failingMkdirAll(string, os.FileMode) error { return errStub }

func failingMkdirTemp(string, string) (string, error) {
	return consts.Empty, errStub
}

func failingOpenRelative(string, string) (*os.File, error) {
	return nil, errStub
}

func failingReadAll(io.Reader) ([]byte, error) { return nil, errStub }

func failingRelPath(string, string) (string, error) {
	return consts.Empty, errStub
}

func failingRemove(string) error { return errStub }

func failingRename(string, string) error { return errStub }

func failingStat(string) (os.FileInfo, error) { return nil, errStub }

func failingWalk(string, fs.WalkDirFunc) error { return errStub }

func failingWriteFull(io.Writer, []byte) error { return errStub }

func prepGoSubDir(t *testing.T) string {
	t.Helper()

	workspace := t.TempDir()
	nested := filepath.Join(workspace, taskfilesDirName, consts.Go, subDirName)
	assertNoErr(t, mkdirAll(nested, dirModePerm))

	return workspace
}

func succeedThenFail() func(string) error {
	calls := consts.IndexZero

	return func(string) error {
		calls++

		if calls == consts.IndexOne {
			return nil
		}

		return errStub
	}
}

func swapSeam[T any](t *testing.T, target *T, stub T) {
	t.Helper()

	original := *target

	*target = stub

	t.Cleanup(func() { *target = original })
}

func lockSeams(t *testing.T) {
	t.Helper()
	t.Cleanup(testsupport.Lock())
}

func swapChmodFile(t *testing.T, stub func(*os.File, os.FileMode) error) {
	t.Helper()

	swapSeam(t, &chmodFile, stub)
}

func swapCloseFile(t *testing.T, stub func(*os.File) error) {
	t.Helper()

	swapSeam(t, &closeFile, stub)
}

func swapCreateTemp(t *testing.T, stub func(string, string) (*os.File, error)) {
	t.Helper()

	swapSeam(t, &createTemp, stub)
}

func swapMarshalYAML(t *testing.T, stub func(any) ([]byte, error)) {
	t.Helper()

	swapSeam(t, &marshalYAML, stub)
}

func swapMkdirAll(t *testing.T, stub func(string, os.FileMode) error) {
	t.Helper()

	swapSeam(t, &mkdirAll, stub)
}

func swapMkdirTemp(t *testing.T, stub func(string, string) (string, error)) {
	t.Helper()

	swapSeam(t, &mkdirTemp, stub)
}

func swapOpenRelative(t *testing.T, stub func(string, string) (*os.File, error)) {
	t.Helper()

	swapSeam(t, &openRelativeFile, stub)
}

func swapReadAll(t *testing.T, stub func(io.Reader) ([]byte, error)) {
	t.Helper()

	swapSeam(t, &readAll, stub)
}

func swapRelPath(t *testing.T, stub func(string, string) (string, error)) {
	t.Helper()

	swapSeam(t, &relPath, stub)
}

func swapRemoveAll(t *testing.T, stub func(string) error) {
	t.Helper()

	swapSeam(t, &removeAll, stub)
}

func swapRemovePath(t *testing.T, stub func(string) error) {
	t.Helper()

	swapSeam(t, &removePath, stub)
}

func swapRenamePath(t *testing.T, stub func(string, string) error) {
	t.Helper()

	swapSeam(t, &renamePath, stub)
}

func swapStatPath(t *testing.T, stub func(string) (os.FileInfo, error)) {
	t.Helper()

	swapSeam(t, &statPath, stub)
}

func swapWalkDir(t *testing.T, stub func(string, fs.WalkDirFunc) error) {
	t.Helper()

	swapSeam(t, &walkDir, stub)
}

func swapWriteFull(t *testing.T, stub func(io.Writer, []byte) error) {
	t.Helper()

	swapSeam(t, &writeFull, stub)
}

// TestCollectMetadataCandidatesReportsWalkFailure verifies walk failures surface.
func TestCollectMetadataCandidatesReportsWalkFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWalkDir(t, failingWalk)

	cands, err := collectMetadataCandidates(t.TempDir(), metaRelPath)
	iox.Discard(cands)
	assertFails(t, err)
}

// TestDiscoverPreviousMetadataReportsMissing verifies empty candidate sets fail.
func TestDiscoverPreviousMetadataReportsMissing(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	meta, err := discoverPreviousMetadata(t.TempDir(), metaRelPath)
	iox.Discard(meta)

	if !errors.Is(err, errPreviousMetadataNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
}

// TestHandleDirEntryAllowsNormalDir verifies normal dirs are not skipped.
func TestHandleDirEntryAllowsNormalDir(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	rel, scan, err := handleDirEntry(fakeDirEntry{name: taskfilesDirName, dir: true})
	assertNoErr(t, err)

	if rel != consts.Empty || scan != metadataNotCandidate {
		t.Fatalf(relScanFmt, rel, scan)
	}
}

// TestHandleDirEntrySkipsGit verifies .git directories return SkipDir.
func TestHandleDirEntrySkipsGit(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	rel, scan, err := handleDirEntry(fakeDirEntry{name: gitDirName, dir: true})
	iox.Discard(rel)
	iox.Discard(scan)

	if !errors.Is(err, filepath.SkipDir) {
		t.Fatalf(errSkipDirFmt, err)
	}
}

// TestLoadFirstCandidateReportsCorrupt verifies corrupt candidate metadata fails.
func TestLoadFirstCandidateReportsCorrupt(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	root := t.TempDir()
	rel := otherMetaRel
	path := filepath.Join(root, filepath.FromSlash(rel))
	assertNoErr(t, os.MkdirAll(filepath.Dir(path), dirModePerm))

	writeTempFile(t, path, []byte(badYAMLText))

	meta, err := loadFirstCandidate(root, []string{rel})
	iox.Discard(meta)
	assertFails(t, err)
}

// TestMetadataCandidateWalkerReportsWalkError verifies walk errors are wrapped.
func TestMetadataCandidateWalkerReportsWalkError(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	walker := metadataCandidateWalker(&metadataWalkerArgs{
		workspace:           t.TempDir(),
		currentMetadataPath: metaRelPath,
		candidates:          &[]string{},
	})

	assertFails(t, walker(stagingName, nil, errStub))
}

// TestMetadataFileCandidateSkipsCurrentAndLegacy verifies current/legacy paths are ignored.
func TestMetadataFileCandidateSkipsCurrentAndLegacy(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	rel, scan, err := metadataFileCandidate(&metadataCandidateArgs{
		workspace:           wsRoot,
		currentMetadataPath: metaRelPath,
		abs:                 "/ws/" + metaRelPath,
		entry: fakeDirEntry{
			name: storeMetadataFileName,
		},
	})

	assertNoErr(t, err)

	if rel != consts.Empty || scan != metadataNotCandidate {
		t.Fatalf(relScanFmt, rel, scan)
	}
}

// TestProcessMetadataCandidatePropagatesSkipDir verifies SkipDir is preserved.
func TestProcessMetadataCandidatePropagatesSkipDir(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := processMetadataCandidate(&metadataWalkerArgs{
		workspace:           t.TempDir(),
		currentMetadataPath: metaRelPath,
		candidates:          &[]string{},
	}, filepath.Join(t.TempDir(), gitDirName), fakeDirEntry{name: gitDirName, dir: true})

	if !errors.Is(err, filepath.SkipDir) {
		t.Fatalf(errSkipDirFmt, err)
	}
}

// TestRelMetadataPathReportsFailure verifies Rel failures surface.
func TestRelMetadataPathReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRelPath(t, failingRelPath)

	rel, err := relMetadataPath(wsRoot, "/ws/meta.yml")
	iox.Discard(rel)
	assertFails(t, err)
}

// TestTryLegacyMetadataSkipsWhenAlreadyLegacy verifies legacy path short-circuits.
func TestTryLegacyMetadataSkipsWhenAlreadyLegacy(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	meta, found, err := tryLegacyMetadata(t.TempDir(), config.LegacyMetadataPath)
	iox.Discard(meta)

	if found || !errors.Is(err, errMetadataNotFound) {
		t.Fatalf("found=%t err=%v", found, err)
	}
}

func (fakeDirEntry) Info() (os.FileInfo, error) { return nil, errStub }

func (entry fakeDirEntry) IsDir() bool { return entry.dir }

func (entry fakeDirEntry) Name() string { return entry.name }

func (fakeDirEntry) Type() fs.FileMode { return consts.IndexZero }

// TestBuildFileEntryReportsInfoFailure verifies DirEntry.Info failures surface.
func TestBuildFileEntryReportsInfoFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	entry, err := buildFileEntry(&fileEntryArgs{
		ops:          nil,
		sourceToDest: nil,
		sourceDir:    consts.Empty,
		fromDest:     consts.Empty,
		entry:        fakeDirEntry{name: fileNameTxt, dir: false},
		rel:          fileNameTxt,
		absPath:      fileNameTxt,
	})

	iox.Discard(entry)
	assertFails(t, err)
}

// TestCollectModuleFilesReportsWalkFailure verifies CollectModuleFiles wraps walk errors.
func TestCollectModuleFilesReportsWalkFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWalkDir(t, failingWalk)

	contents, err := CollectModuleFiles(&CollectOptions{
		TaskfileOps:  nil,
		SourceToDest: nil,
		SourceDir:    t.TempDir(),
		FromDest:     consts.Go,
		DocPolicy:    DocPolicySkip,
	})

	iox.Discard(contents)
	assertFails(t, err)
}

// TestEnsureSourceDirExistsReportsMissing verifies missing source dirs fail.
func TestEnsureSourceDirExistsReportsMissing(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := ensureSourceDirExists(
		filepath.Join(t.TempDir(), pathMissing),
		emptyModulePtr(consts.Go, consts.Empty),
	)
	assertFails(t, err)
}

// TestIsDestinationManagedFindsModule verifies matching destinations are managed.
func TestIsDestinationManagedFindsModule(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(consts.Empty, consts.Go, consts.Empty)}

	managed := isDestinationManaged(&lock, emptyModulePtr(consts.Empty, consts.Go))

	if !managed {
		t.Fatal("expected managed destination")
	}
}

// TestIsDestinationManagedNilLock verifies nil locks are unmanaged.
func TestIsDestinationManagedNilLock(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	managed := isDestinationManaged(nil, emptyModulePtr(consts.Empty, consts.Go))

	if managed {
		t.Fatal("nil lock should be unmanaged")
	}
}

// TestLogicalRootReadyMissingReturnsFalse verifies missing dirs are not ready.
func TestLogicalRootReadyMissingReturnsFalse(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	ready, err := logicalRootReady(filepath.Join(t.TempDir(), pathMissing))
	assertNoErr(t, err)

	if ready {
		t.Fatal("expected missing root")
	}
}

// TestLogicalRootReadyReportsStatFailure verifies non-missing stat errors surface.
func TestLogicalRootReadyReportsStatFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapStatPath(t, failingStat)

	ready, err := logicalRootReady(stagingName)
	iox.Discard(ready)
	assertFails(t, err)
}

// TestMergeParentDocFilesSkipsUnready verifies missing parent roots are skipped.
func TestMergeParentDocFilesSkipsUnready(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := mergeParentDocFiles(&mergeParentDocsArgs{
		destRoot:   filepath.Join(t.TempDir(), pathMissing),
		contents:   fMap{},
		parentDocs: map[string]struct{}{},
		collect: &collectModuleArgs{
			sourceDir:  consts.Empty,
			destDirRel: consts.Empty,
			syncInput:  syncInputWithConfig(),
			mod: &moduleRecord{
				SourceModule:      consts.Empty,
				Path:              consts.Empty,
				DestinationModule: consts.Go,
			},
		},
	})

	assertNoErr(t, err)
}

// TestReadRootTaskfileReportsTemplateFailure verifies nil ops fail for missing roots.
func TestReadRootTaskfileReportsTemplateFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	data, state, err := readRootTaskfile(nil, t.TempDir(), rootTaskfileName)
	iox.Discard(data)
	iox.Discard(state)
	assertFails(t, err)
}

// TestRelSlashPathReportsFailure verifies Rel failures surface.
func TestRelSlashPathReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRelPath(t, failingRelPath)

	rel, err := relSlashPath(srcDir, srcFileA)
	iox.Discard(rel)
	assertFails(t, err)
}

// TestRootTemplateOrErrorRequiresOps verifies nil ops are rejected.
func TestRootTemplateOrErrorRequiresOps(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	data, err := rootTemplateOrError(nil)
	iox.Discard(data)

	if !errors.Is(err, errTaskfileOpsNotConfigured) {
		t.Fatalf(errBareFmt, err)
	}
}

// TestScanModuleFilesReportsWalkFailure verifies walkDir failures surface.
func TestScanModuleFilesReportsWalkFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWalkDir(t, failingWalk)

	contents, err := scanModuleFiles(
		&collectOptions{
			sourceDir:    t.TempDir(),
			ops:          nil,
			sourceToDest: nil,
			fromDest:     consts.Empty,
			docPolicy:    0,
		},
	)
	iox.Discard(contents)
	assertFails(t, err)
}

// TestSortManagedFilesOrdersByPath verifies managed files sort by path.
func TestSortManagedFilesOrdersByPath(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	planned := sortedManagedFixture()
	sortManagedFiles(planned)

	first := planned[consts.IndexZero]

	if first.Path != pathA || first.SourceModule != modXName {
		t.Fatalf("planned = %+v", planned)
	}
}

// TestUpdateRootTaskfileRequiresOps verifies nil ops are rejected.
func TestUpdateRootTaskfileRequiresOps(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	root, tasks, err := updateRootTaskfile(&updateRootArgs{
		args: buildRootArgs{syncInput: emptySyncInputPtr()},
	})

	iox.Discard(root)
	iox.Discard(tasks)

	if !errors.Is(err, errTaskfileOpsNotConfigured) {
		t.Fatalf(errBareFmt, err)
	}
}

// TestValidateDestinationReportsStatFailure verifies destination stat failures surface.
func TestValidateDestinationReportsStatFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapStatPath(t, failingStat)

	err := validateDestination(
		stagingName,
		&moduleRecord{
			Path:              "taskfiles/go",
			SourceModule:      consts.Empty,
			DestinationModule: consts.Empty,
		},
		nil,
	)
	assertFails(t, err)
}

// TestValidateExistingDestinationRejectsFile verifies file destinations are rejected.
func TestValidateExistingDestinationRejectsFile(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	path := filepath.Join(t.TempDir(), fileNameTxt)
	writeTempFile(t, path, []byte(byteX))

	info, err := os.Stat(path)
	assertNoErr(t, err)
	assertFails(
		t,
		validateExistingDestination(
			info,
			&moduleRecord{Path: byteX, SourceModule: consts.Empty, DestinationModule: consts.Empty},
			nil,
		),
	)
}

// TestWalkCollectModuleFileReportsWalkError verifies walk errors are wrapped.
func TestWalkCollectModuleFileReportsWalkError(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	err := walkCollectModuleFile(&walkCollectArgs{
		absPath: stagingName,
		walkErr: errStub,
		opts: &collectOptions{
			sourceDir:    t.TempDir(),
			ops:          nil,
			sourceToDest: nil,
			fromDest:     consts.Empty,
			docPolicy:    0,
		},
		entry: fakeDirEntry{name: fileNameTxt, dir: false},
	})

	assertFails(t, err)
}

// TestCollectRequestedSourcesPreservesOrder verifies resolution sources are collected.
func TestCollectRequestedSourcesPreservesOrder(t *testing.T) {
	t.Parallel()
	lockSeams(t)

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
	lockSeams(t)

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
	lockSeams(t)

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

// TestLockContentChangedReportsNewMarshalFailure verifies new-lock marshal failures.
func TestLockContentChangedReportsNewMarshalFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	calls := consts.IndexZero

	swapMarshalYAML(t, func(any) ([]byte, error) {
		calls++

		if calls == consts.IndexOne {
			return []byte(byteX), nil
		}

		return nil, errStub
	})

	oldLock := emptyLock()
	newLock := emptyLock()

	changed, err := lockContentChanged(&oldLock, &newLock)
	iox.Discard(changed)
	assertFails(t, err)
}

// TestDiffLockAndMetadataReportsMetadataFailure verifies metadata section failures.
func TestDiffLockAndMetadataReportsMetadataFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	assertNoErr(t, mkdirAll(filepath.Join(workspace, filepath.FromSlash(metaRelPath)), dirModePerm))

	plan := emptyPlan()

	plan.Metadata = emptyMetadata(lockRelPath)
	plan.Lock = emptyLock()

	lists, err := diffLockAndMetadata(&diffInput{
		plan:         &plan,
		workspace:    workspace,
		metadataPath: metaRelPath,
		plannedMeta:  []byte(byteX),
		syncRoot:     syncRootDisabled,
	}, zeroDiffLists())
	iox.Discard(lists)
	assertFails(t, err)
}

// TestDiffMetadataFileSectionReportsFailure verifies metadata section wrapper failures.
func TestDiffMetadataFileSectionReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	assertNoErr(t, mkdirAll(filepath.Join(workspace, filepath.FromSlash(metaRelPath)), dirModePerm))

	lists, err := diffMetadataFileSection(&diffInput{
		workspace:    workspace,
		metadataPath: metaRelPath,
		plannedMeta:  []byte(byteX),
	}, zeroDiffLists())
	iox.Discard(lists)
	assertFails(t, err)
}

// TestCommitTempFileReportsWriteFailure verifies writeAndFinalizeTemp failures.
func TestCommitTempFileReportsWriteFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWriteFull(t, failingWriteFull)

	tmp := createRealTemp(t)
	cleanup := true

	assertFails(t, commitTempFile(&finalizeTempArgs{
		tmp:  tmp,
		data: []byte(byteX),
		mode: fileModeRegular,
		path: filepath.Join(t.TempDir(), outName),
	}, tmp.Name(), &cleanup))
}

// TestLoadMetadataFallbacksReportsLegacyFailure verifies non-notfound legacy errors.
func TestLoadMetadataFallbacksReportsLegacyFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	path := filepath.Join(workspace, filepath.FromSlash(config.LegacyMetadataPath))
	assertNoErr(t, mkdirAll(filepath.Dir(path), dirModePerm))
	writeTempFile(t, path, []byte(badYAMLText))

	meta, err := loadMetadataFallbacks(workspace, metaRelPath)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestDiscoverPreviousMetadataReportsLoadFailure verifies corrupt candidate load failures.
func TestDiscoverPreviousMetadataReportsLoadFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	workspace := t.TempDir()
	cand := filepath.Join(workspace, filepath.FromSlash(otherMetaRel))
	assertNoErr(t, mkdirAll(filepath.Dir(cand), dirModePerm))
	writeTempFile(t, cand, []byte(badYAMLText))

	meta, err := discoverPreviousMetadata(workspace, metaRelPath)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestProcessMetadataCandidateReportsUnexpectedFailure verifies non-SkipDir errors.
func TestProcessMetadataCandidateReportsUnexpectedFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRelPath(t, failingRelPath)

	err := processMetadataCandidate(&metadataWalkerArgs{
		workspace:           wsRoot,
		currentMetadataPath: metaRelPath,
		candidates:          &[]string{},
	}, wsFileX, fakeDirEntry{name: fileNameTxt, dir: false})
	assertFails(t, err)
}

// TestMetadataCandidateReportsRelFailure verifies Rel failures for both candidates.
func TestMetadataCandidateReportsRelFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	assertCandidateRelFails(t, previousMetadataCandidate)
	assertCandidateRelFails(t, metadataFileCandidate)
}

// TestCollectModuleFileReportsRelFailure verifies Rel failures during collection.
func TestCollectModuleFileReportsRelFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRelPath(t, failingRelPath)

	err := collectModuleFile(&moduleCollectArgs{
		sourceDir: srcDir,
		absPath:   srcFileA,
		entry:     fakeDirEntry{name: pathA, dir: false},
		contents:  fMap{},
	})

	assertFails(t, err)
}

// TestMergeParentDocFilesReportsReadyFailure verifies ready-check failures.
func TestMergeParentDocFilesReportsReadyFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapStatPath(t, failingStat)

	err := mergeParentDocFiles(&mergeParentDocsArgs{
		destRoot:   t.TempDir(),
		contents:   fMap{},
		parentDocs: map[string]struct{}{},
		collect: &collectModuleArgs{
			syncInput: &domain.SyncInput{Config: &config.Config{}},
			mod:       emptyModulePtr(consts.Empty, consts.Go),
		},
	})

	assertFails(t, err)
}

// TestIsDestinationManagedMissesOtherModule verifies non-matching destinations.
func TestIsDestinationManagedMissesOtherModule(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(pathA, pathA, consts.Empty)}

	mod := emptyModule(consts.Empty, consts.Go)

	if isDestinationManaged(&lock, &mod) {
		t.Fatal("expected unmanaged destination")
	}
}

// TestLoadOneStoreMetadataFileReportsRelFailure verifies module-name Rel failures.
func TestLoadOneStoreMetadataFileReportsRelFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRelPath(t, failingRelPath)

	err := loadOneStoreMetadataFile(rootDir, rootMetaPath, map[string]storeTaskMetadata{})
	assertFails(t, err)
}

// TestReadStoreTaskMetadataReportsReadFailure verifies missing metadata files fail.
func TestReadStoreTaskMetadataReportsReadFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	meta, err := readStoreTaskMetadata(filepath.Join(t.TempDir(), storeMetadataFileName), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestParseStoreTaskMetadataFillsEmptyModule verifies empty module names are filled.
func TestParseStoreTaskMetadataFillsEmptyModule(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	data := []byte("schema: " + storeMetadataSchema + "\nmodule: \"\"\nexported_tasks: []\n")
	meta, err := parseStoreTaskMetadata(data, consts.Go)
	assertNoErr(t, err)

	if meta.Module != consts.Go {
		t.Fatalf("module = %q, want %q", meta.Module, consts.Go)
	}
}

// TestAppendUniqueExportedTaskSkipsEmptyAndDupes verifies empty/dup task filtering.
func TestAppendUniqueExportedTaskSkipsEmptyAndDupes(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	seen := map[string]struct{}{}

	out := appendUniqueExportedTask(nil, seen, consts.Empty)

	out = appendUniqueExportedTask(out, seen, taskCI)
	out = appendUniqueExportedTask(out, seen, taskCI)

	if len(out) != consts.IndexOne || out[consts.IndexZero] != taskCI {
		t.Fatalf("out = %v", out)
	}
}

// TestGeneratedTaskMetadataMissingRecord verifies absent requested records.
func TestGeneratedTaskMetadataMissingRecord(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	meta, ok := generatedTaskMetadata(&groupModulesInput{
		requestedRecords: map[string]moduleRecord{},
		metadata:         storeTaskMetaMap{},
	}, consts.Go)
	iox.Discard(meta)

	if ok {
		t.Fatal("expected missing record")
	}
}

// TestGeneratedTaskMetadataResolvesRecord verifies known records resolve metadata.
func TestGeneratedTaskMetadataResolvesRecord(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	var want storeTaskMetadata

	want.Schema = storeMetadataSchema
	want.Module = consts.Go
	want.ExportedTasks = []string{taskCI}

	meta, ok := generatedTaskMetadata(goTaskMetadataInput(&want), consts.Go)

	if !ok || meta.Module != consts.Go {
		t.Fatalf("ok=%t meta=%+v", ok, meta)
	}
}

// TestLoadStoreTaskMetadataReportsWalkFailure verifies load wraps walk failures.
func TestLoadStoreTaskMetadataReportsWalkFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapWalkDir(t, failingWalk)

	meta, err := loadStoreTaskMetadata(stubSnapshot{root: t.TempDir()})
	iox.Discard(meta)
	assertFails(t, err)
}

// TestModuleNameForReportsRelFailure verifies Rel failures surface.
func TestModuleNameForReportsRelFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)
	swapRelPath(t, failingRelPath)

	name, err := moduleNameFor(rootDir, rootMetaPath)
	iox.Discard(name)
	assertFails(t, err)
}

// TestParseStoreTaskMetadataRejectsBadSchema verifies unsupported schemas fail.
func TestParseStoreTaskMetadataRejectsBadSchema(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	meta, err := parseStoreTaskMetadata([]byte("schema: other\n"), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestParseStoreTaskMetadataRejectsCorruptYAML verifies corrupt metadata fails.
func TestParseStoreTaskMetadataRejectsCorruptYAML(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	meta, err := parseStoreTaskMetadata([]byte(badYAMLText), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestStoreMetadataWalkerReportsWalkError verifies walk errors are wrapped.
func TestStoreMetadataWalkerReportsWalkError(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	walker := storeMetadataWalker(t.TempDir(), map[string]storeTaskMetadata{})
	assertFails(t, walker(stagingName, nil, errStub))
}

// TestUnmarshalYAMLReportsFailure verifies non-mapping nodes fail.
func TestUnmarshalYAMLReportsFailure(t *testing.T) {
	t.Parallel()
	lockSeams(t)

	var meta storeTaskMetadata

	assertFails(t, yaml.Unmarshal([]byte("plain\n"), &meta))
}

func (stubSnapshot) DefaultBranch() string { return consts.Empty }

func (snap stubSnapshot) ModuleDir(string) string { return snap.root }

func (stubSnapshot) ResolvedCommit() string { return consts.Empty }

func (stubSnapshot) SourceRef() string { return consts.Empty }

func (snap stubSnapshot) WorkspaceRoot() string { return snap.root }
