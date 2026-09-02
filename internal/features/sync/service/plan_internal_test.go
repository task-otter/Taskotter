// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:exhaustruct_v5 // test fixtures only set fields exercised by the unit
package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// TestBuildFileEntryReportsInfoFailure verifies DirEntry.Info failures surface.
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

// TestCollectModuleFilesReportsWalkFailure verifies CollectModuleFiles wraps walk errors.
//
//nolint:paralleltest // swaps the package-level walkDir seam
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
}

// TestEnsureSourceDirExistsReportsMissing verifies missing source dirs fail.
func TestEnsureSourceDirExistsReportsMissing(t *testing.T) {
	t.Parallel()

	err := ensureSourceDirExists(
		filepath.Join(t.TempDir(), pathMissing),
		emptyModulePtr(consts.Go, consts.Empty),
	)
	assertFails(t, err)
}

// TestIsDestinationManagedFindsModule verifies matching destinations are managed.
func TestIsDestinationManagedFindsModule(t *testing.T) {
	t.Parallel()

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

	managed := isDestinationManaged(nil, emptyModulePtr(consts.Empty, consts.Go))

	if managed {
		t.Fatal("nil lock should be unmanaged")
	}
}

// TestLogicalRootReadyMissingReturnsFalse verifies missing dirs are not ready.
func TestLogicalRootReadyMissingReturnsFalse(t *testing.T) {
	t.Parallel()

	ready, err := logicalRootReady(filepath.Join(t.TempDir(), pathMissing))
	assertNoErr(t, err)

	if ready {
		t.Fatal("expected missing root")
	}
}

// TestLogicalRootReadyReportsStatFailure verifies non-missing stat errors surface.
//
//nolint:paralleltest // swaps the package-level statPath seam
func TestLogicalRootReadyReportsStatFailure(t *testing.T) {
	swapStatPath(t, failingStat)

	ready, err := logicalRootReady(stagingName)
	iox.Discard(ready)
	assertFails(t, err)
}

// TestMergeParentDocFilesSkipsUnready verifies missing parent roots are skipped.
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

// TestReadRootTaskfileReportsTemplateFailure verifies nil ops fail for missing roots.
func TestReadRootTaskfileReportsTemplateFailure(t *testing.T) {
	t.Parallel()

	data, state, err := readRootTaskfile(nil, t.TempDir(), rootTaskfileName)
	iox.Discard(data)
	iox.Discard(state)
	assertFails(t, err)
}

// TestRelSlashPathReportsFailure verifies Rel failures surface.
//
//nolint:paralleltest // swaps the package-level relPath seam
func TestRelSlashPathReportsFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	rel, err := relSlashPath(srcDir, srcFileA)
	iox.Discard(rel)
	assertFails(t, err)
}

// TestRootTemplateOrErrorRequiresOps verifies nil ops are rejected.
func TestRootTemplateOrErrorRequiresOps(t *testing.T) {
	t.Parallel()

	data, err := rootTemplateOrError(nil)
	iox.Discard(data)

	if !errors.Is(err, errTaskfileOpsNotConfigured) {
		t.Fatalf(errBareFmt, err)
	}
}

// TestScanModuleFilesReportsWalkFailure verifies walkDir failures surface.
//
//nolint:paralleltest // swaps a package-level FS seam
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
}

// TestSortManagedFilesOrdersByPath verifies managed files sort by path.
func TestSortManagedFilesOrdersByPath(t *testing.T) {
	t.Parallel()

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
//
//nolint:paralleltest // swaps a package-level FS seam
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
}

// TestValidateExistingDestinationRejectsFile verifies file destinations are rejected.
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

// TestWalkCollectModuleFileReportsWalkError verifies walk errors are wrapped.
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
