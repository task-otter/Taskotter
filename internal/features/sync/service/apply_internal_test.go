// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:exhaustruct_v5 // test fixtures only set fields exercised by the unit
package service

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// TestApplyFileChangeDefaultIsNoOp verifies unknown change kinds leave lists untouched.
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

// TestCleanupFailedStagingJoinsRemoveError verifies cleanup joins remove failures.
//
//nolint:paralleltest // swaps the package-level removeAll seam
func TestCleanupFailedStagingJoinsRemoveError(t *testing.T) {
	swapRemoveAll(t, failingRemove)

	err := cleanupFailedStaging(stagingName, errStub)

	if !errors.Is(err, errStub) {
		t.Fatalf(errWantFmt, err, errStub)
	}
}

// TestCleanupFailedStagingReturnsCopyError verifies successful cleanup keeps copyErr.
func TestCleanupFailedStagingReturnsCopyError(t *testing.T) {
	t.Parallel()

	err := cleanupFailedStaging(t.TempDir(), errStub)

	if !errors.Is(err, errStub) {
		t.Fatalf(errWantFmt, err, errStub)
	}
}

// TestCleanupLegacyMetadataSkipsLegacyPath verifies legacy metadata path is left alone.
func TestCleanupLegacyMetadataSkipsLegacyPath(t *testing.T) {
	t.Parallel()

	assertNoErr(t, cleanupLegacyMetadata(t.TempDir(), config.LegacyMetadataPath))
}

// TestCleanupStagingDirReportsRemoveFailure verifies staging cleanup surfaces remove errors.
//
//nolint:paralleltest // swaps the package-level removeAll seam
func TestCleanupStagingDirReportsRemoveFailure(t *testing.T) {
	swapRemoveAll(t, failingRemove)

	assertFails(t, cleanupStagingDir(stagingName))
}

// TestCleanupStagingOnExitKeepsPrimaryError verifies cleanup errors do not clobber primary.
//
//nolint:paralleltest // swaps the package-level removeAll seam
func TestCleanupStagingOnExitKeepsPrimaryError(t *testing.T) {
	swapRemoveAll(t, failingRemove)

	primary := errStub
	cleanupStagingOnExit(stagingName, &primary)

	if !errors.Is(primary, errStub) {
		t.Fatalf("err = %v, want primary", primary)
	}
}

// TestCleanupStagingOnExitSurfacesCleanupError verifies nil primary adopts cleanup error.
//
//nolint:paralleltest // swaps the package-level removeAll seam
func TestCleanupStagingOnExitSurfacesCleanupError(t *testing.T) {
	swapRemoveAll(t, failingRemove)

	var err error

	cleanupStagingOnExit(stagingName, &err)
	assertFails(t, err)
}

// TestCopyStagedFilesReportsFailure verifies staging copy failures surface.
func TestCopyStagedFilesReportsFailure(t *testing.T) {
	t.Parallel()

	err := copyStagedFiles(t.TempDir(), []stagedFile{{
		finalRel: fileNameTxt,
		entry:    domain.FileEntry{Data: []byte(byteX), Mode: fileModeRegular},
	}}, func(string, *domain.FileEntry) error { return errStub })
	assertFails(t, err)
}

// TestFileChangeFromDataDetectsUpdate verifies mismatched hashes mark updates.
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

// TestPrepareStagingRootReportsMkdirFailure verifies parent mkdir failures surface.
//
//nolint:paralleltest // swaps the package-level mkdirAll seam
func TestPrepareStagingRootReportsMkdirFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	root, err := prepareStagingRoot(t.TempDir(), config.DefaultTargetFolder)
	iox.Discard(root)
	assertFails(t, err)
}

// TestPrepareStagingRootReportsTempFailure verifies MkdirTemp failures surface.
//
//nolint:paralleltest // swaps the package-level mkdirTemp seam
func TestPrepareStagingRootReportsTempFailure(t *testing.T) {
	swapMkdirTemp(t, failingMkdirTemp)

	root, err := prepareStagingRoot(t.TempDir(), config.DefaultTargetFolder)
	iox.Discard(root)
	assertFails(t, err)
}

// TestPruneDirsUntilStopReportsFailure verifies prune stops on remove errors.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestPruneDirsUntilStopReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	workspace := t.TempDir()
	nested := filepath.Join(workspace, pathA, pathB)
	assertNoErr(t, os.MkdirAll(nested, dirModePerm))

	assertFails(t, pruneDirsUntilStop(nested, workspace))
}

// TestPruneEmptyParentDirsSkipsEmptyStop verifies empty stopRel is a no-op.
func TestPruneEmptyParentDirsSkipsEmptyStop(t *testing.T) {
	t.Parallel()

	assertNoErr(t, pruneEmptyParentDirs(t.TempDir(), fileNameTxt, consts.Empty))
}

// TestRemoveDirIfEmptyIgnoresNotEmpty verifies ENOTEMPTY is ignored.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveDirIfEmptyIgnoresNotEmpty(t *testing.T) {
	swapRemovePath(t, func(string) error { return syscall.ENOTEMPTY })

	assertNoErr(t, removeDirIfEmpty(stagingName, removeEmptyCtx))
}

// TestRemoveDirIfEmptyReportsFailure verifies unexpected remove errors surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveDirIfEmptyReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeDirIfEmpty(stagingName, removeEmptyCtx))
}

// TestRemoveEmptyParentDirReportsFailure verifies prune propagates remove failures.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveEmptyParentDirReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	exists, err := removeEmptyParentDir(stagingName)
	iox.Discard(exists)
	assertFails(t, err)
}

// TestRemoveIfExistsReportsFailure verifies unexpected remove errors surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveIfExistsReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeIfExists(t.TempDir(), fileNameTxt))
}

// TestRemoveLegacyMetadataDirReportsFailure verifies legacy dir remove failures.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveLegacyMetadataDirReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeLegacyMetadataDir(t.TempDir()))
}

// TestRemoveLegacyMetadataFileReportsFailure verifies legacy metadata remove failures.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveLegacyMetadataFileReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeLegacyMetadataFile(t.TempDir()))
}

// TestRemoveObsoleteFileReportsFailure verifies unexpected remove errors surface.
//
//nolint:paralleltest // swaps the package-level removePath seam
func TestRemoveObsoleteFileReportsFailure(t *testing.T) {
	swapRemovePath(t, failingRemove)

	assertFails(t, removeObsoleteFile(t.TempDir(), fileNameTxt))
}

// TestRemoveStaleManagedFileReportsRemoveFailure verifies stale remove failures surface.
//
//nolint:paralleltest // swaps a package-level FS seam
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
}

// TestRemoveStaleManagedFileSkipsCurrent verifies current paths are kept.
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

// TestStagePlanFilesCleansFailedStaging verifies failed staging invokes cleanupFailedStaging.
//
//nolint:paralleltest // swaps the package-level removeAll seam
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
}

// TestStagePlanFilesPrepareRootFailure verifies prepareStagingRoot failures surface.
//
//nolint:paralleltest // swaps a package-level FS seam
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
}

// TestValidateAndWriteStagedReportsValidateFailure verifies validation errors surface.
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

// TestValidateGeneratedYAMLReportsFailure verifies the first invalid staged entry fails.
func TestValidateGeneratedYAMLReportsFailure(t *testing.T) {
	t.Parallel()

	staged := []stagedFile{{
		finalRel: lockFileName,
		entry:    domain.FileEntry{Data: []byte(badYAMLText), Mode: fileModeRegular},
	}}
	assertFails(t, validateGeneratedYAML(staged, rootTaskfileName))
}

// TestValidateStagedYAMLReportsFailure verifies staged YAML validation wraps errors.
func TestValidateStagedYAMLReportsFailure(t *testing.T) {
	t.Parallel()

	err := validateStagedYAML(&stagedFile{
		finalRel: lockFileName,
		entry:    domain.FileEntry{Data: []byte(badYAMLText), Mode: fileModeRegular},
	}, rootTaskfileName)
	assertFails(t, err)
}

// TestValidateYAMLReportsFailures verifies lock, metadata, and root validators reject bad YAML.
func TestValidateYAMLReportsFailures(t *testing.T) {
	t.Parallel()

	bad := []byte(badYAMLText)

	assertFails(t, validateLockFileYAML(bad))

	assertFails(t, validateMetadataYAML(bad))
	assertFails(t, validateRootTaskfileYAML(bad))
}

// TestWriteStagedFilesReportsCopyFailure verifies copy hook failures surface.
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

// TestWriteStagedFilesReportsMkdirFailure verifies destination mkdir failures surface.
//
//nolint:paralleltest // swaps the package-level mkdirAll seam
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
}
