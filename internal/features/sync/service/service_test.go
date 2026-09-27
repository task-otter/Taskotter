// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
	yaml "go.yaml.in/yaml/v3"
)

type (
	failingOps struct{}

	dirSnapshot struct {
		root string
	}

	tempEntry struct {
		entry os.DirEntry
		root  string
	}

	fakeDirEntry struct {
		name string
		dir  bool
	}

	stubSnapshot struct {
		root string
	}
)

const (
	lockRelPath = "taskfiles/.taskotter-lock.yml"
	metaRelPath = "taskfiles/.taskotter/metadata.yml"

	srcModuleName = "src"
	emptyTaskYAML = "version: \"3\"\ntasks: {}\n"
	goDestPath    = "taskfiles/go"
	targetWantFmt = "target = %q, want %q"

	wantErrText      = "expected error"
	unexpectFmt      = "unexpected error: %v"
	stagingName      = "staging-root"
	fileNameTxt      = "file.txt"
	payloadText      = "payload"
	badYAMLText      = "{{bad"
	byteX            = "x"
	pathA            = "a"
	pathB            = "b"
	outName          = "o"
	srcDir           = "/src"
	srcFileA         = "/src/a"
	wsRoot           = "/ws"
	wsFileX          = "/ws/x"
	rootDir          = "/root"
	rootMetaPath     = "/root/go/metadata.yml"
	goTaskfileRel    = "taskfiles/go/Taskfile.yml"
	goOldTxtRel      = "taskfiles/go/old.txt"
	oldTargetFolder  = "old-taskfiles"
	oldTargetFileRel = "old-taskfiles/go/a.txt"
	otherMetaRel     = "other/.taskotter/metadata.yml"
	missingTxt       = "missing.txt"
	gitDirName       = ".git"
	errWantFmt       = "err = %v, want %v"
	errSkipDirFmt    = "err = %v, want SkipDir"
	listsFmt         = "lists = %+v"
	relScanFmt       = "rel=%q scan=%v"
	errBareFmt       = "err = %v"
	removeEmptyCtx   = "remove empty"
	eslintNodePNPM   = "eslint/node/pnpm"
	eslintBun        = "eslint/bun"
	taskCI           = "ci"
	pathMissing      = "missing"

	unknownFileChange = 99

	subDirName  = "sub"
	goSubOldRel = "taskfiles/go/sub/old.txt"
	gotFmt      = "got = %v"
	modXName    = "mod-x"
)

//nolint:gochecknoglobals // Test seam restoration is shared across helpers and guarded by testSeamMu.
var (
	errStub          = errors.New("stub failure")
	testSeamRestores = make(map[*testing.T][]func())
	testSeamMu       sync.Mutex
)

// TestApplyFileChangeDefaultIsNoOp verifies the expected behavior.
func TestApplyFileChangeDefaultIsNoOp(t *testing.T) {
	t.Parallel()

	lists := applyFileChange(
		&diffLists{added: nil, updated: nil, removed: nil},
		fileNameTxt,
		fileChangeKind(unknownFileChange),
	)

	if len(lists.added)+len(lists.updated) != consts.IndexZero {
		t.Fatalf(listsFmt, lists)
	}
}

// TestCleanupFailedStagingJoinsRemoveError verifies the expected behavior.
func TestCleanupFailedStagingJoinsRemoveError(t *testing.T) {
	swapRemoveAll(t, failingRemove)

	err := cleanupFailedStaging(stagingName, errStub)

	if !errors.Is(err, errStub) {
		t.Fatalf(errWantFmt, err, errStub)
	}

	restoreTestSeams(t)
	t.Parallel()
}

// TestCleanupFailedStagingReturnsCopyError verifies the expected behavior.
func TestCleanupFailedStagingReturnsCopyError(t *testing.T) {
	t.Parallel()

	err := cleanupFailedStaging(t.TempDir(), errStub)

	if !errors.Is(err, errStub) {
		t.Fatalf(errWantFmt, err, errStub)
	}
}

// TestCleanupLegacyMetadataSkipsLegacyPath verifies the expected behavior.
func TestCleanupLegacyMetadataSkipsLegacyPath(t *testing.T) {
	t.Parallel()

	assertNoErr(t, cleanupLegacyMetadata(t.TempDir(), config.LegacyMetadataPath))
}

// TestCleanupStagingDirReportsRemoveFailure verifies the expected behavior.
func TestCleanupStagingDirReportsRemoveFailure(t *testing.T) {
	swapRemoveAll(t, failingRemove)

	assertFails(t, cleanupStagingDir(stagingName))
	restoreTestSeams(t)
	t.Parallel()
}

// TestCleanupStagingOnExitKeepsPrimaryError verifies the expected behavior.
func TestCleanupStagingOnExitKeepsPrimaryError(t *testing.T) {
	swapRemoveAll(t, failingRemove)

	primary := errStub
	cleanupStagingOnExit(stagingName, &primary)

	if !errors.Is(primary, errStub) {
		t.Fatalf("err = %v, want primary", primary)
	}

	restoreTestSeams(t)
	t.Parallel()
}

// TestCleanupStagingOnExitSurfacesCleanupError verifies the expected behavior.
func TestCleanupStagingOnExitSurfacesCleanupError(t *testing.T) {
	swapRemoveAll(t, failingRemove)

	var err error

	cleanupStagingOnExit(stagingName, &err)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestCopyStagedFilesReportsFailure verifies the expected behavior.
func TestCopyStagedFilesReportsFailure(t *testing.T) {
	t.Parallel()

	err := copyStagedFiles(t.TempDir(), []stagedFile{{
		finalRel: fileNameTxt,
		entry:    domain.FileEntry{Data: []byte(byteX), Mode: fileModeRegular},
	}}, func(string, *domain.FileEntry) error { return errStub })
	assertFails(t, err)
}

// TestFileChangeFromDataDetectsUpdate verifies the expected behavior.
func TestFileChangeFromDataDetectsUpdate(t *testing.T) {
	t.Parallel()

	kind := fileChangeFromData(
		[]byte(pathA),
		managedAtPtr(consts.Empty, consts.Empty, "deadbeef"),
	)

	if kind != fileUpdated {
		t.Fatalf("kind = %v, want updated", kind)
	}
}

// TestPrepareStagingRootReportsMkdirFailure verifies the expected behavior.
func TestPrepareStagingRootReportsMkdirFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	root, err := prepareStagingRoot(t.TempDir(), config.DefaultTargetFolder)
	iox.Discard(root)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestPrepareStagingRootReportsTempFailure verifies the expected behavior.
func TestPrepareStagingRootReportsTempFailure(t *testing.T) {
	swapMkdirTemp(t, failingMkdirTemp)

	root, err := prepareStagingRoot(t.TempDir(), config.DefaultTargetFolder)
	iox.Discard(root)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestPruneDirsUntilStopReportsFailure verifies the expected behavior.
func TestPruneDirsUntilStopReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	workspace := t.TempDir()
	nested := filepath.Join(workspace, pathA, pathB)
	assertNoErr(t, os.MkdirAll(nested, dirModePerm))

	assertFails(t, pruneDirsUntilStop(nested, workspace))
	restoreTestSeams(t)
	t.Parallel()
}

// TestPruneEmptyParentDirsSkipsEmptyStop verifies the expected behavior.
func TestPruneEmptyParentDirsSkipsEmptyStop(t *testing.T) {
	t.Parallel()

	assertNoErr(t, pruneEmptyParentDirs(t.TempDir(), fileNameTxt, consts.Empty))
}

// TestRemoveDirIfEmptyIgnoresNotEmpty verifies the expected behavior.
func TestRemoveDirIfEmptyIgnoresNotEmpty(t *testing.T) {
	swapRemovePath(t, func(string) error { return syscall.ENOTEMPTY })

	assertNoErr(t, removeDirIfEmpty(stagingName, removeEmptyCtx))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveDirIfEmptyReportsFailure verifies the expected behavior.
func TestRemoveDirIfEmptyReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeDirIfEmpty(stagingName, removeEmptyCtx))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveEmptyParentDirReportsFailure verifies the expected behavior.
func TestRemoveEmptyParentDirReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	exists, err := removeEmptyParentDir(stagingName)
	iox.Discard(exists)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveIfExistsReportsFailure verifies the expected behavior.
func TestRemoveIfExistsReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeIfExists(t.TempDir(), fileNameTxt))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveLegacyMetadataDirReportsFailure verifies the expected behavior.
func TestRemoveLegacyMetadataDirReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeLegacyMetadataDir(t.TempDir()))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveLegacyMetadataFileReportsFailure verifies the expected behavior.
func TestRemoveLegacyMetadataFileReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeLegacyMetadataFile(t.TempDir()))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveObsoleteFileReportsFailure verifies the expected behavior.
func TestRemoveObsoleteFileReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeObsoleteFile(t.TempDir(), fileNameTxt))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveStaleManagedFileReportsRemoveFailure verifies the expected behavior.
func TestRemoveStaleManagedFileReportsRemoveFailure(t *testing.T) {
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveStaleManagedFileSkipsCurrent verifies the expected behavior.
func TestRemoveStaleManagedFileSkipsCurrent(t *testing.T) {
	t.Parallel()

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

// TestStagePlanFilesCleansFailedStaging verifies the expected behavior.
func TestStagePlanFilesCleansFailedStaging(t *testing.T) {
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestStagePlanFilesPrepareRootFailure verifies the expected behavior.
func TestStagePlanFilesPrepareRootFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	root, err := stagePlanFiles(&stagePlanArgs{
		staged:       nil,
		workspace:    t.TempDir(),
		targetFolder: config.DefaultTargetFolder,
		copyFile:     copyFileTo,
	})

	iox.Discard(root)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestValidateAndWriteStagedReportsValidateFailure verifies the expected behavior.
func TestValidateAndWriteStagedReportsValidateFailure(t *testing.T) {
	t.Parallel()

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

// TestValidateGeneratedYAMLReportsFailure verifies the expected behavior.
func TestValidateGeneratedYAMLReportsFailure(t *testing.T) {
	t.Parallel()

	staged := []stagedFile{{
		finalRel: lockFileName,
		entry:    domain.FileEntry{Data: []byte(badYAMLText), Mode: fileModeRegular},
	}}
	assertFails(t, validateGeneratedYAML(staged, rootTaskfileName))
}

// TestValidateStagedYAMLReportsFailure verifies the expected behavior.
func TestValidateStagedYAMLReportsFailure(t *testing.T) {
	t.Parallel()

	err := validateStagedYAML(&stagedFile{
		finalRel: lockFileName,
		entry:    domain.FileEntry{Data: []byte(badYAMLText), Mode: fileModeRegular},
	}, rootTaskfileName)
	assertFails(t, err)
}

// TestValidateYAMLReportsFailures verifies the expected behavior.
func TestValidateYAMLReportsFailures(t *testing.T) {
	t.Parallel()

	bad := []byte(badYAMLText)

	assertFails(t, validateLockFileYAML(bad))

	assertFails(t, validateMetadataYAML(bad))
	assertFails(t, validateRootTaskfileYAML(bad))
}

// TestWriteStagedFilesReportsCopyFailure verifies the expected behavior.
func TestWriteStagedFilesReportsCopyFailure(t *testing.T) {
	t.Parallel()

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

// TestWriteStagedFilesReportsMkdirFailure verifies the expected behavior.
func TestWriteStagedFilesReportsMkdirFailure(t *testing.T) {
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestApplyFileChangeAddedAndUpdated verifies the expected behavior.
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

// TestApplyPlanWithCleanupReportsSessionFailure verifies the expected behavior.
func TestApplyPlanWithCleanupReportsSessionFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	assertFails(t, applyPlanWithCleanup(minimalPlan(), minimalSyncInput(t.TempDir())))
	restoreTestSeams(t)
	t.Parallel()
}

// TestApplyStagedPlanReportsCleanupFailure verifies the expected behavior.
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestBuildFileEntryReportsReadFailure verifies the expected behavior.
func TestBuildFileEntryReportsReadFailure(t *testing.T) {
	t.Parallel()

	entry, err := buildFileEntry(missingFileEntryArgs(t))
	iox.Discard(entry)
	assertFails(t, err)
}

// TestCleanupAfterApplyReportsLegacyFailure verifies the expected behavior.
func TestCleanupAfterApplyReportsLegacyFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, cleanupAfterApply(minimalPlan(), t.TempDir(), metaRelPath))
	restoreTestSeams(t)
	t.Parallel()
}

// TestCleanupAfterApplyReportsObsoleteFailure verifies the expected behavior.
func TestCleanupAfterApplyReportsObsoleteFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldLock = lockWithManaged(goOldTxtRel)
	plan.OldLock.Configuration.TargetFolder = config.DefaultTargetFolder
	assertFails(t, cleanupAfterApply(plan, t.TempDir(), metaRelPath))
	restoreTestSeams(t)
	t.Parallel()
}

// TestCleanupLegacyMetadataReportsDirFailure verifies the expected behavior.
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestCleanupLegacyMetadataReportsFileFailure verifies the expected behavior.
func TestCleanupLegacyMetadataReportsFileFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, cleanupLegacyMetadata(t.TempDir(), metaRelPath))
	restoreTestSeams(t)
	t.Parallel()
}

// TestCleanupOldTargetReportsStepFailure verifies the expected behavior.
func TestCleanupOldTargetReportsStepFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = lockWithManaged(oldTargetFileRel)
	assertFails(t, removeOldTargetFiles(plan, t.TempDir()))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveOldTargetLockReportsFailure verifies the expected behavior.
func TestRemoveOldTargetLockReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	assertFails(t, removeOldTargetLock(plan, t.TempDir()))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveOldTargetMetadataReportsDirFailure verifies the expected behavior.
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveOldTargetMetadataReportsFailure verifies the expected behavior.
func TestRemoveOldTargetMetadataReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	assertFails(t, removeOldTargetMetadata(plan, t.TempDir()))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveStaleManagedFileReportsPruneFailure verifies the expected behavior.
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveStaleManagedFilesReportsFailure verifies the expected behavior.
func TestRemoveStaleManagedFilesReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(goOldTxtRel, consts.Go, consts.Empty)}
	lock.Configuration.TargetFolder = config.DefaultTargetFolder
	assertFails(t, removeStaleManagedFiles(&lock, map[string]struct{}{}, t.TempDir()))
	restoreTestSeams(t)
	t.Parallel()
}

// TestScanLogicalRootDocsReportsWalkFailure verifies the expected behavior.
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestStagePreparedFilesReportsFailure verifies the expected behavior.
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestStartApplySessionReportsStagingFailure verifies the expected behavior.
func TestStartApplySessionReportsStagingFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	session, err := startApplySession(minimalPlan(), minimalSyncInput(t.TempDir()))
	iox.Discard(session)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestStoreCollectedModuleFileReportsFailure verifies the expected behavior.
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

// TestTryLegacyMetadataReportsCorrupt verifies the expected behavior.
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

// TestUpdateRootTaskfileReportsOpsFailure verifies the expected behavior.
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

// TestApplyFileChangeUnchangedIsNoOp verifies the expected behavior.
func TestApplyFileChangeUnchangedIsNoOp(t *testing.T) {
	t.Parallel()

	lists := applyFileChange(
		&diffLists{added: nil, updated: nil, removed: nil},
		fileNameTxt,
		fileUnchanged,
	)

	if len(lists.added)+len(lists.updated) != consts.IndexZero {
		t.Fatalf(listsFmt, lists)
	}
}

// TestDiffLockFileReportsReadFailure verifies the expected behavior.
func TestDiffLockFileReportsReadFailure(t *testing.T) {
	t.Parallel()

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

// TestDiffManagedFilePathsReportsFailure verifies the expected behavior.
func TestDiffManagedFilePathsReportsFailure(t *testing.T) {
	t.Parallel()

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

// TestDiffMetadataFileReportsReadFailure verifies the expected behavior.
func TestDiffMetadataFileReportsReadFailure(t *testing.T) {
	t.Parallel()

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

// TestFileChangeFromDataUnchanged verifies the expected behavior.
func TestFileChangeFromDataUnchanged(t *testing.T) {
	t.Parallel()

	sum := "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"
	kind := fileChangeFromData(
		[]byte(pathA),
		managedAtPtr(consts.Empty, consts.Empty, sum),
	)

	if kind != fileUnchanged {
		t.Fatalf("kind = %v, want unchanged", kind)
	}
}

// TestFileChangedReportsReadFailure verifies the expected behavior.
func TestFileChangedReportsReadFailure(t *testing.T) {
	t.Parallel()

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

// TestLockContentChangedDetectsDifference verifies the expected behavior.
func TestLockContentChangedDetectsDifference(t *testing.T) {
	t.Parallel()

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

// TestMarshalLockForCompareNilReturnsNil verifies the expected behavior.
func TestMarshalLockForCompareNilReturnsNil(t *testing.T) {
	t.Parallel()

	data, err := marshalLockForCompare(nil)
	assertNoErr(t, err)

	if data != nil {
		t.Fatalf("data = %v, want nil", data)
	}
}

// TestCleanupTempFileRunsWhenFlagged verifies the expected behavior.
func TestCleanupTempFileRunsWhenFlagged(t *testing.T) {
	t.Parallel()

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

// TestCopyFileCopiesRelativeSource verifies the expected behavior.
func TestCopyFileCopiesRelativeSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(payloadText))

	dst := filepath.Join(t.TempDir(), "out.txt")
	assertNoErr(t, CopyFile(&copyFileArgs{
		root: root, rel: fileNameTxt, dst: dst, mode: fileModeRegular,
	}))
	assertFilePayload(t, dst, payloadText)
}

// TestCopyFileReportsMissingSource verifies the expected behavior.
func TestCopyFileReportsMissingSource(t *testing.T) {
	t.Parallel()

	err := CopyFile(&copyFileArgs{
		root: t.TempDir(), rel: fileNameTxt, dst: filepath.Join(t.TempDir(), outName),
		mode: fileModeRegular,
	})

	assertFails(t, err)
}

// TestCopyFileReportsWriteFailure verifies the expected behavior.
func TestCopyFileReportsWriteFailure(t *testing.T) {
	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(byteX))
	swapMkdirAll(t, failingMkdirAll)

	err := CopyFile(&copyFileArgs{
		root: root, rel: fileNameTxt, dst: filepath.Join(t.TempDir(), outName),
		mode: fileModeRegular,
	})

	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestCopyFileToReportsWriteFailure verifies the expected behavior.
func TestCopyFileToReportsWriteFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	err := copyFileTo(filepath.Join(t.TempDir(), pathA, fileNameTxt), &domain.FileEntry{
		Data: []byte(byteX), Mode: fileModeRegular,
	})

	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestCreateTempFileReportsCreateFailure verifies the expected behavior.
func TestCreateTempFileReportsCreateFailure(t *testing.T) {
	swapCreateTemp(t, failingCreateTemp)

	file, err := createTempFile(t.TempDir())
	discardTemp(file)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestCreateTempFileReportsMkdirFailure verifies the expected behavior.
func TestCreateTempFileReportsMkdirFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	file, err := createTempFile(filepath.Join(t.TempDir(), "nested"))
	discardTemp(file)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestReadRelativeFileReportsOpenFailure verifies the expected behavior.
func TestReadRelativeFileReportsOpenFailure(t *testing.T) {
	swapOpenRelative(t, failingOpenRelative)

	data, err := readRelativeFile(t.TempDir(), fileNameTxt)
	iox.Discard(data)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestReadRelativeFileReportsReadFailure verifies the expected behavior.
func TestReadRelativeFileReportsReadFailure(t *testing.T) {
	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(byteX))
	swapReadAll(t, failingReadAll)

	data, err := readRelativeFile(root, fileNameTxt)
	iox.Discard(data)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestRenameTempFileReportsFailure verifies the expected behavior.
func TestRenameTempFileReportsFailure(t *testing.T) {
	swapRenamePath(t, failingRename)

	assertFails(t, renameTempFile(pathA, pathB))
	restoreTestSeams(t)
	t.Parallel()
}

// TestWriteAndFinalizeTempReportsChmodFailure verifies the expected behavior.
func TestWriteAndFinalizeTempReportsChmodFailure(t *testing.T) {
	swapChmodFile(t, failingChmod)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
	restoreTestSeams(t)
	t.Parallel()
}

// TestWriteAndFinalizeTempReportsCloseFailure verifies the expected behavior.
func TestWriteAndFinalizeTempReportsCloseFailure(t *testing.T) {
	swapCloseFile(t, failingClose)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
	restoreTestSeams(t)
	t.Parallel()
}

// TestWriteAndFinalizeTempReportsWriteFailure verifies the expected behavior.
func TestWriteAndFinalizeTempReportsWriteFailure(t *testing.T) {
	swapWriteFull(t, failingWriteFull)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
	restoreTestSeams(t)
	t.Parallel()
}

// TestWriteFileAtomicReportsFinalizeFailure verifies the expected behavior.
func TestWriteFileAtomicReportsFinalizeFailure(t *testing.T) {
	swapRenamePath(t, failingRename)

	path := filepath.Join(t.TempDir(), fileNameTxt)
	assertFails(t, writeFileAtomic(path, []byte(byteX), fileModeRegular))
	restoreTestSeams(t)
	t.Parallel()
}

// TestWriteFullStubWriterUsedDocumentsFaults verifies the expected behavior.
func TestWriteFullStubWriterUsedDocumentsFaults(t *testing.T) {
	t.Parallel()

	writer := &faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault}
	assertFails(t, writeFull(writer, []byte(byteX)))
}

func assertFilePayload(t *testing.T, path, want string) {
	t.Helper()

	//nolint:gosec // Test helper reads a path created by its caller.
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

// TestBuildPlanFromStateReportsFinalizeFailure verifies the expected behavior.
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

// TestBuildPlanFromStateReportsPlanFailure verifies the expected behavior.
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

// TestBuildRootPlanResultReportsReadFailure verifies the expected behavior.
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

// TestBuildRootTaskfileReportsUpdateFailure verifies the expected behavior.
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

// TestCollectAndTrackModuleFilesReportsFailure verifies the expected behavior.
func TestCollectAndTrackModuleFilesReportsFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	contents, managed, err := collectAndTrackModuleFiles(sampleCollectArgs(t.TempDir()))
	iox.Discard(contents)
	iox.Discard(managed)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestCollectModuleContentsReportsMergeFailure verifies the expected behavior.
func TestCollectModuleContentsReportsMergeFailure(t *testing.T) {
	args := distinctDocCollectArgs(t)
	swapWalkThenFail(t)

	assertCollectModuleContentsFails(t, args)
	restoreTestSeams(t)
	t.Parallel()
}

// TestCollectModuleContentsReportsScanFailure verifies the expected behavior.
func TestCollectModuleContentsReportsScanFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	assertCollectModuleContentsFails(t, sampleCollectArgs(t.TempDir()))
	restoreTestSeams(t)
	t.Parallel()
}

func assertCollectModuleContentsFails(t *testing.T, args *collectModuleArgs) {
	t.Helper()

	contents, docs, err := collectModuleContents(args)
	iox.Discard(contents)
	iox.Discard(docs)
	assertFails(t, err)
}

// TestCollectModuleFileReportsStoreFailure verifies the expected behavior.
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

// TestDiffFilesReportsLockMetadataFailure verifies the expected behavior.
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

// TestDiffLockFileReportsContentFailure verifies the expected behavior.
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestDiscoverPreviousMetadataReportsWalkFailure verifies the expected behavior.
func TestDiscoverPreviousMetadataReportsWalkFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	meta, err := discoverPreviousMetadata(t.TempDir(), metaRelPath)
	iox.Discard(meta)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestFinalizeBuiltPlanReportsDiffFailure verifies the expected behavior.
func TestFinalizeBuiltPlanReportsDiffFailure(t *testing.T) {
	t.Parallel()

	workspace := prepFinalizeDiffWorkspace(t)
	out, err := finalizeBuiltPlan(finalizeDiffFailInput(workspace))
	iox.Discard(out)
	assertFails(t, err)
}

// TestLoadPreviousLockUsesLockTarget verifies the expected behavior.
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

// TestMergeLogicalRootDocsReportsMergeFailure verifies the expected behavior.
func TestMergeLogicalRootDocsReportsMergeFailure(t *testing.T) {
	args := distinctDocCollectArgs(t)
	assertNoErr(t, mkdirAll(args.syncInput.Snapshot.ModuleDir(consts.Go), dirModePerm))

	swapWalkDir(t, failingWalk)

	docs, err := mergeLogicalRootDocs(args, fMap{}, DocPolicyInclude)
	iox.Discard(docs)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestMergeParentDocsIfDistinctReportsFailure verifies the expected behavior.
func TestMergeParentDocsIfDistinctReportsFailure(t *testing.T) {
	args := distinctDocCollectArgs(t)
	assertNoErr(t, mkdirAll(args.syncInput.Snapshot.ModuleDir(consts.Go), dirModePerm))

	swapWalkDir(t, failingWalk)

	docs, err := mergeParentDocsIfDistinct(args, fMap{}, map[string]struct{}{})
	iox.Discard(docs)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestPlanModuleFilesReportsCollectFailure verifies the expected behavior.
func TestPlanModuleFilesReportsCollectFailure(t *testing.T) {
	workspace := t.TempDir()
	assertNoErr(t, mkdirAll(filepath.Join(workspace, consts.Go), dirModePerm))
	swapWalkDir(t, failingWalk)

	contents, managed, err := planModuleFiles(goModulePlanArgs(workspace))
	iox.Discard(contents)
	iox.Discard(managed)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestPrepareModulePlanDirsReportsMissingSource verifies the expected behavior.
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

// TestReadAndMaybeRewriteModuleFileReportsRewriteFailure verifies the expected behavior.
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

// TestReadRootPlanFinishInputReportsFailure verifies the expected behavior.
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

// TestRemoveObsoleteReportsOldTargetFailure verifies the expected behavior.
func TestRemoveObsoleteReportsOldTargetFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = emptyLockPtr()
	assertFails(t, removeObsolete(plan, t.TempDir()))
	restoreTestSeams(t)
	t.Parallel()
}

// TestRemoveOldTargetFilesSkipsOutsidePrefix verifies the expected behavior.
func TestRemoveOldTargetFilesSkipsOutsidePrefix(t *testing.T) {
	t.Parallel()

	plan := minimalPlan()

	plan.OldTargetFolder = oldTargetFolder
	plan.OldLock = lockWithManaged(goOldTxtRel)
	assertNoErr(t, removeOldTargetFiles(plan, t.TempDir()))
}

// TestWalkCollectModuleFileReportsCollectFailure verifies the expected behavior.
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

	swapSeam(t, &walkDir, func(root string, walker fs.WalkDirFunc) error {
		calls++

		if calls == consts.IndexOne {
			return original(root, walker)
		}

		return errStub
	})
}

func writeValidLock(t *testing.T, workspace, targetFolder string) {
	t.Helper()

	var lock syncLock

	lock.Configuration.TargetFolder = targetFolder

	path := filepath.Join(workspace, filepath.FromSlash(lockRelPath))
	assertNoErr(t, mkdirAll(filepath.Dir(path), dirModePerm))

	writeTempFile(t, path, lockmodel.MarshalLock(&lock))
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

//nolint:ireturn // os.FileInfo is required by the filesystem seam signature.
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

	if len(testSeamRestores[t]) == consts.IndexZero {
		testSeamMu.Lock()
		t.Cleanup(func() { restoreTestSeams(t) })
	}

	original := *target
	restore := func() {
		*target = original
	}

	*target = stub

	testSeamRestores[t] = append(testSeamRestores[t], restore)
}

func restoreTestSeams(t *testing.T) {
	t.Helper()

	restores := testSeamRestores[t]

	if len(restores) == consts.IndexZero {
		return
	}

	for idx := range slices.Backward(restores) {
		restores[idx]()
	}

	delete(testSeamRestores, t)
	testSeamMu.Unlock()
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

// TestCollectMetadataCandidatesReportsWalkFailure verifies the expected behavior.
func TestCollectMetadataCandidatesReportsWalkFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	cands, err := collectMetadataCandidates(t.TempDir(), metaRelPath)
	iox.Discard(cands)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestDiscoverPreviousMetadataReportsMissing verifies the expected behavior.
func TestDiscoverPreviousMetadataReportsMissing(t *testing.T) {
	t.Parallel()

	meta, err := discoverPreviousMetadata(t.TempDir(), metaRelPath)
	iox.Discard(meta)

	if !errors.Is(err, errPreviousMetadataNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
}

// TestHandleDirEntryAllowsNormalDir verifies the expected behavior.
func TestHandleDirEntryAllowsNormalDir(t *testing.T) {
	t.Parallel()

	rel, scan, err := handleDirEntry(fakeDirEntry{name: taskfilesDirName, dir: true})
	assertNoErr(t, err)

	if rel != consts.Empty || scan != metadataNotCandidate {
		t.Fatalf(relScanFmt, rel, scan)
	}
}

// TestHandleDirEntrySkipsGit verifies the expected behavior.
func TestHandleDirEntrySkipsGit(t *testing.T) {
	t.Parallel()

	rel, scan, err := handleDirEntry(fakeDirEntry{name: gitDirName, dir: true})
	iox.Discard(rel)
	iox.Discard(scan)

	if !errors.Is(err, filepath.SkipDir) {
		t.Fatalf(errSkipDirFmt, err)
	}
}

// TestLoadFirstCandidateReportsCorrupt verifies the expected behavior.
func TestLoadFirstCandidateReportsCorrupt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := otherMetaRel
	path := filepath.Join(root, filepath.FromSlash(rel))
	assertNoErr(t, os.MkdirAll(filepath.Dir(path), dirModePerm))

	writeTempFile(t, path, []byte(badYAMLText))

	meta, err := loadFirstCandidate(root, []string{rel})
	iox.Discard(meta)
	assertFails(t, err)
}

// TestMetadataCandidateWalkerReportsWalkError verifies the expected behavior.
func TestMetadataCandidateWalkerReportsWalkError(t *testing.T) {
	t.Parallel()

	walker := metadataCandidateWalker(&metadataWalkerArgs{
		workspace:           t.TempDir(),
		currentMetadataPath: metaRelPath,
		candidates:          &[]string{},
	})

	assertFails(t, walker(stagingName, nil, errStub))
}

// TestMetadataFileCandidateSkipsCurrentAndLegacy verifies the expected behavior.
func TestMetadataFileCandidateSkipsCurrentAndLegacy(t *testing.T) {
	t.Parallel()

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

// TestProcessMetadataCandidatePropagatesSkipDir verifies the expected behavior.
func TestProcessMetadataCandidatePropagatesSkipDir(t *testing.T) {
	t.Parallel()

	err := processMetadataCandidate(&metadataWalkerArgs{
		workspace:           t.TempDir(),
		currentMetadataPath: metaRelPath,
		candidates:          &[]string{},
	}, filepath.Join(t.TempDir(), gitDirName), fakeDirEntry{name: gitDirName, dir: true})

	if !errors.Is(err, filepath.SkipDir) {
		t.Fatalf(errSkipDirFmt, err)
	}
}

// TestRelMetadataPathReportsFailure verifies the expected behavior.
func TestRelMetadataPathReportsFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	rel, err := relMetadataPath(wsRoot, "/ws/meta.yml")
	iox.Discard(rel)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestTryLegacyMetadataSkipsWhenAlreadyLegacy verifies the expected behavior.
func TestTryLegacyMetadataSkipsWhenAlreadyLegacy(t *testing.T) {
	t.Parallel()

	meta, found, err := tryLegacyMetadata(t.TempDir(), config.LegacyMetadataPath)
	iox.Discard(meta)

	if found || !errors.Is(err, errMetadataNotFound) {
		t.Fatalf("found=%t err=%v", found, err)
	}
}

//nolint:ireturn // os.FileInfo is required by fs.DirEntry.
func (fakeDirEntry) Info() (os.FileInfo, error) { return nil, errStub }

func (entry fakeDirEntry) IsDir() bool { return entry.dir }

func (entry fakeDirEntry) Name() string { return entry.name }

func (fakeDirEntry) Type() fs.FileMode { return consts.IndexZero }

// TestBuildFileEntryReportsInfoFailure verifies the expected behavior.
func TestBuildFileEntryReportsInfoFailure(t *testing.T) {
	t.Parallel()

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

// TestCollectModuleFilesReportsWalkFailure verifies the expected behavior.
func TestCollectModuleFilesReportsWalkFailure(t *testing.T) {
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestEnsureSourceDirExistsReportsMissing verifies the expected behavior.
func TestEnsureSourceDirExistsReportsMissing(t *testing.T) {
	t.Parallel()

	err := ensureSourceDirExists(
		filepath.Join(t.TempDir(), pathMissing),
		emptyModulePtr(consts.Go, consts.Empty),
	)
	assertFails(t, err)
}

// TestIsDestinationManagedFindsModule verifies the expected behavior.
func TestIsDestinationManagedFindsModule(t *testing.T) {
	t.Parallel()

	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(consts.Empty, consts.Go, consts.Empty)}

	managed := isDestinationManaged(&lock, emptyModulePtr(consts.Empty, consts.Go))

	if !managed {
		t.Fatal("expected managed destination")
	}
}

// TestIsDestinationManagedNilLock verifies the expected behavior.
func TestIsDestinationManagedNilLock(t *testing.T) {
	t.Parallel()

	managed := isDestinationManaged(nil, emptyModulePtr(consts.Empty, consts.Go))

	if managed {
		t.Fatal("nil lock should be unmanaged")
	}
}

// TestLogicalRootReadyMissingReturnsFalse verifies the expected behavior.
func TestLogicalRootReadyMissingReturnsFalse(t *testing.T) {
	t.Parallel()

	ready, err := logicalRootReady(filepath.Join(t.TempDir(), pathMissing))
	assertNoErr(t, err)

	if ready {
		t.Fatal("expected missing root")
	}
}

// TestLogicalRootReadyReportsStatFailure verifies the expected behavior.
func TestLogicalRootReadyReportsStatFailure(t *testing.T) {
	swapStatPath(t, failingStat)

	ready, err := logicalRootReady(stagingName)
	iox.Discard(ready)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestMergeParentDocFilesSkipsUnready verifies the expected behavior.
func TestMergeParentDocFilesSkipsUnready(t *testing.T) {
	t.Parallel()

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

// TestReadRootTaskfileReportsTemplateFailure verifies the expected behavior.
func TestReadRootTaskfileReportsTemplateFailure(t *testing.T) {
	t.Parallel()

	data, state, err := readRootTaskfile(nil, t.TempDir(), rootTaskfileName)
	iox.Discard(data)
	iox.Discard(state)
	assertFails(t, err)
}

// TestRelSlashPathReportsFailure verifies the expected behavior.
func TestRelSlashPathReportsFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	rel, err := relSlashPath(srcDir, srcFileA)
	iox.Discard(rel)
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestRootTemplateOrErrorRequiresOps verifies the expected behavior.
func TestRootTemplateOrErrorRequiresOps(t *testing.T) {
	t.Parallel()

	data, err := rootTemplateOrError(nil)
	iox.Discard(data)

	if !errors.Is(err, errTaskfileOpsNotConfigured) {
		t.Fatalf(errBareFmt, err)
	}
}

// TestScanModuleFilesReportsWalkFailure verifies the expected behavior.
func TestScanModuleFilesReportsWalkFailure(t *testing.T) {
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestSortManagedFilesOrdersByPath verifies the expected behavior.
func TestSortManagedFilesOrdersByPath(t *testing.T) {
	t.Parallel()

	planned := sortedManagedFixture()
	sortManagedFiles(planned)

	first := planned[consts.IndexZero]

	if first.Path != pathA || first.SourceModule != modXName {
		t.Fatalf("planned = %+v", planned)
	}
}

// TestUpdateRootTaskfileRequiresOps verifies the expected behavior.
func TestUpdateRootTaskfileRequiresOps(t *testing.T) {
	t.Parallel()

	root, tasks, err := updateRootTaskfile(&updateRootArgs{
		args: buildRootArgs{syncInput: emptySyncInputPtr()},
	})

	iox.Discard(root)
	iox.Discard(tasks)

	if !errors.Is(err, errTaskfileOpsNotConfigured) {
		t.Fatalf(errBareFmt, err)
	}
}

// TestValidateDestinationReportsStatFailure verifies the expected behavior.
func TestValidateDestinationReportsStatFailure(t *testing.T) {
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestValidateExistingDestinationRejectsFile verifies the expected behavior.
func TestValidateExistingDestinationRejectsFile(t *testing.T) {
	t.Parallel()

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

// TestWalkCollectModuleFileReportsWalkError verifies the expected behavior.
func TestWalkCollectModuleFileReportsWalkError(t *testing.T) {
	t.Parallel()

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

// TestLockContentChangedReportsNewMarshalFailure verifies the expected behavior.
func TestLockContentChangedReportsNewMarshalFailure(t *testing.T) {
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestDiffLockAndMetadataReportsMetadataFailure verifies the expected behavior.
func TestDiffLockAndMetadataReportsMetadataFailure(t *testing.T) {
	t.Parallel()

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

// TestDiffMetadataFileSectionReportsFailure verifies the expected behavior.
func TestDiffMetadataFileSectionReportsFailure(t *testing.T) {
	t.Parallel()

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

// TestCommitTempFileReportsWriteFailure verifies the expected behavior.
func TestCommitTempFileReportsWriteFailure(t *testing.T) {
	swapWriteFull(t, failingWriteFull)

	tmp := createRealTemp(t)
	cleanup := true

	assertFails(t, commitTempFile(&finalizeTempArgs{
		tmp:  tmp,
		data: []byte(byteX),
		mode: fileModeRegular,
		path: filepath.Join(t.TempDir(), outName),
	}, tmp.Name(), &cleanup))
	restoreTestSeams(t)
	t.Parallel()
}

// TestLoadMetadataFallbacksReportsLegacyFailure verifies the expected behavior.
func TestLoadMetadataFallbacksReportsLegacyFailure(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, filepath.FromSlash(config.LegacyMetadataPath))
	assertNoErr(t, mkdirAll(filepath.Dir(path), dirModePerm))
	writeTempFile(t, path, []byte(badYAMLText))

	meta, err := loadMetadataFallbacks(workspace, metaRelPath)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestDiscoverPreviousMetadataReportsLoadFailure verifies the expected behavior.
func TestDiscoverPreviousMetadataReportsLoadFailure(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cand := filepath.Join(workspace, filepath.FromSlash(otherMetaRel))
	assertNoErr(t, mkdirAll(filepath.Dir(cand), dirModePerm))
	writeTempFile(t, cand, []byte(badYAMLText))

	meta, err := discoverPreviousMetadata(workspace, metaRelPath)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestProcessMetadataCandidateReportsUnexpectedFailure verifies the expected behavior.
func TestProcessMetadataCandidateReportsUnexpectedFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	err := processMetadataCandidate(&metadataWalkerArgs{
		workspace:           wsRoot,
		currentMetadataPath: metaRelPath,
		candidates:          &[]string{},
	}, wsFileX, fakeDirEntry{name: fileNameTxt, dir: false})
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestMetadataCandidateReportsRelFailure verifies the expected behavior.
func TestMetadataCandidateReportsRelFailure(t *testing.T) {
	assertCandidateRelFails(t, previousMetadataCandidate)
	assertCandidateRelFails(t, metadataFileCandidate)
	restoreTestSeams(t)
	t.Parallel()
}

// TestCollectModuleFileReportsRelFailure verifies the expected behavior.
func TestCollectModuleFileReportsRelFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	err := collectModuleFile(&moduleCollectArgs{
		sourceDir: srcDir,
		absPath:   srcFileA,
		entry:     fakeDirEntry{name: pathA, dir: false},
		contents:  fMap{},
	})

	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestMergeParentDocFilesReportsReadyFailure verifies the expected behavior.
func TestMergeParentDocFilesReportsReadyFailure(t *testing.T) {
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
	restoreTestSeams(t)
	t.Parallel()
}

// TestIsDestinationManagedMissesOtherModule verifies the expected behavior.
func TestIsDestinationManagedMissesOtherModule(t *testing.T) {
	t.Parallel()

	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(pathA, pathA, consts.Empty)}

	mod := emptyModule(consts.Empty, consts.Go)

	if isDestinationManaged(&lock, &mod) {
		t.Fatal("expected unmanaged destination")
	}
}

// TestLoadOneStoreMetadataFileReportsRelFailure verifies the expected behavior.
func TestLoadOneStoreMetadataFileReportsRelFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	err := loadOneStoreMetadataFile(rootDir, rootMetaPath, map[string]storeTaskMetadata{})
	assertFails(t, err)
	restoreTestSeams(t)
	t.Parallel()
}

// TestReadStoreTaskMetadataReportsReadFailure verifies the expected behavior.
func TestReadStoreTaskMetadataReportsReadFailure(t *testing.T) {
	t.Parallel()

	meta, err := readStoreTaskMetadata(filepath.Join(t.TempDir(), storeMetadataFileName), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestParseStoreTaskMetadataFillsEmptyModule verifies the expected behavior.
func TestParseStoreTaskMetadataFillsEmptyModule(t *testing.T) {
	t.Parallel()

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
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestLoadStoreTaskMetadataReportsWalkFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	meta, err := loadStoreTaskMetadata(stubSnapshot{root: t.TempDir()})
	iox.Discard(meta)
	assertFails(t, err)
}

// TestModuleNameForReportsRelFailure verifies Rel failures surface.
//
//nolint:paralleltest // swaps the package-level relPath seam
func TestModuleNameForReportsRelFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	name, err := moduleNameFor(rootDir, rootMetaPath)
	iox.Discard(name)
	assertFails(t, err)
	restoreTestSeams(t)
}

// TestParseStoreTaskMetadataRejectsBadSchema verifies unsupported schemas fail.
func TestParseStoreTaskMetadataRejectsBadSchema(t *testing.T) {
	t.Parallel()

	meta, err := parseStoreTaskMetadata([]byte("schema: other\n"), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestParseStoreTaskMetadataRejectsCorruptYAML verifies corrupt metadata fails.
func TestParseStoreTaskMetadataRejectsCorruptYAML(t *testing.T) {
	t.Parallel()

	meta, err := parseStoreTaskMetadata([]byte(badYAMLText), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestStoreMetadataWalkerReportsWalkError verifies walk errors are wrapped.
func TestStoreMetadataWalkerReportsWalkError(t *testing.T) {
	t.Parallel()

	walker := storeMetadataWalker(t.TempDir(), map[string]storeTaskMetadata{})
	assertFails(t, walker(stagingName, nil, errStub))
}

// TestUnmarshalYAMLReportsFailure verifies non-mapping nodes fail.
func TestUnmarshalYAMLReportsFailure(t *testing.T) {
	t.Parallel()

	var meta storeTaskMetadata

	assertFails(t, yaml.Unmarshal([]byte("plain\n"), &meta))
}

func (stubSnapshot) DefaultBranch() string { return consts.Empty }

func (snap stubSnapshot) ModuleDir(string) string { return snap.root }

func (stubSnapshot) ResolvedCommit() string { return consts.Empty }

func (stubSnapshot) SourceRef() string { return consts.Empty }

func (snap stubSnapshot) WorkspaceRoot() string { return snap.root }
