// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:exhaustruct_v5 // test fixtures only set fields exercised by the unit
package service

import (
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// TestLockContentChangedReportsNewMarshalFailure verifies new-lock marshal failures.
//
//nolint:paralleltest // swaps the package-level marshalYAML seam
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
}

// TestDiffLockAndMetadataReportsMetadataFailure verifies metadata section failures.
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

// TestDiffMetadataFileSectionReportsFailure verifies metadata section wrapper failures.
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

// TestCommitTempFileReportsWriteFailure verifies writeAndFinalizeTemp failures.
//
//nolint:paralleltest // swaps the package-level writeFull seam
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
}

// TestLoadMetadataFallbacksReportsLegacyFailure verifies non-notfound legacy errors.
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

// TestDiscoverPreviousMetadataReportsLoadFailure verifies corrupt candidate load failures.
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

// TestProcessMetadataCandidateReportsUnexpectedFailure verifies non-SkipDir errors.
//
//nolint:paralleltest // swaps the package-level relPath seam
func TestProcessMetadataCandidateReportsUnexpectedFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	err := processMetadataCandidate(&metadataWalkerArgs{
		workspace:           wsRoot,
		currentMetadataPath: metaRelPath,
		candidates:          &[]string{},
	}, wsFileX, fakeDirEntry{name: fileNameTxt, dir: false})
	assertFails(t, err)
}

// TestMetadataCandidateReportsRelFailure verifies Rel failures for both candidates.
//
//nolint:paralleltest // swaps the package-level relPath seam
func TestMetadataCandidateReportsRelFailure(t *testing.T) {
	assertCandidateRelFails(t, previousMetadataCandidate)
	assertCandidateRelFails(t, metadataFileCandidate)
}

// TestCollectModuleFileReportsRelFailure verifies Rel failures during collection.
//
//nolint:paralleltest // swaps the package-level relPath seam
func TestCollectModuleFileReportsRelFailure(t *testing.T) {
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
//
//nolint:paralleltest // swaps the package-level statPath seam
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
}

// TestIsDestinationManagedMissesOtherModule verifies non-matching destinations.
func TestIsDestinationManagedMissesOtherModule(t *testing.T) {
	t.Parallel()

	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(pathA, pathA, consts.Empty)}

	mod := emptyModule(consts.Empty, consts.Go)

	if isDestinationManaged(&lock, &mod) {
		t.Fatal("expected unmanaged destination")
	}
}

// TestLoadOneStoreMetadataFileReportsRelFailure verifies module-name Rel failures.
//
//nolint:paralleltest // swaps the package-level relPath seam
func TestLoadOneStoreMetadataFileReportsRelFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	err := loadOneStoreMetadataFile(rootDir, rootMetaPath, map[string]storeTaskMetadata{})
	assertFails(t, err)
}

// TestReadStoreTaskMetadataReportsReadFailure verifies missing metadata files fail.
func TestReadStoreTaskMetadataReportsReadFailure(t *testing.T) {
	t.Parallel()

	meta, err := readStoreTaskMetadata(filepath.Join(t.TempDir(), storeMetadataFileName), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestParseStoreTaskMetadataFillsEmptyModule verifies empty module names are filled.
func TestParseStoreTaskMetadataFillsEmptyModule(t *testing.T) {
	t.Parallel()

	data := []byte("schema: " + storeMetadataSchema + "\nmodule: \"\"\nexported_tasks: []\n")
	meta, err := parseStoreTaskMetadata(data, consts.Go)
	assertNoErr(t, err)

	if meta.Module != consts.Go {
		t.Fatalf("module = %q, want %q", meta.Module, consts.Go)
	}
}
