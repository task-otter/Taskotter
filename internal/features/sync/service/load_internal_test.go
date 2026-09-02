// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:exhaustruct_v5 // test fixtures only set fields exercised by the unit
package service

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	fakeDirEntry struct {
		name string
		dir  bool
	}
)

// TestCollectMetadataCandidatesReportsWalkFailure verifies walk failures surface.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestCollectMetadataCandidatesReportsWalkFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	cands, err := collectMetadataCandidates(t.TempDir(), metaRelPath)
	iox.Discard(cands)
	assertFails(t, err)
}

// TestDiscoverPreviousMetadataReportsMissing verifies empty candidate sets fail.
func TestDiscoverPreviousMetadataReportsMissing(t *testing.T) {
	t.Parallel()

	meta, err := discoverPreviousMetadata(t.TempDir(), metaRelPath)
	iox.Discard(meta)

	if !errors.Is(err, errPreviousMetadataNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
}

// TestHandleDirEntryAllowsNormalDir verifies normal dirs are not skipped.
func TestHandleDirEntryAllowsNormalDir(t *testing.T) {
	t.Parallel()

	rel, scan, err := handleDirEntry(fakeDirEntry{name: taskfilesDirName, dir: true})
	assertNoErr(t, err)

	if rel != consts.Empty || scan != metadataNotCandidate {
		t.Fatalf(relScanFmt, rel, scan)
	}
}

// TestHandleDirEntrySkipsGit verifies .git directories return SkipDir.
func TestHandleDirEntrySkipsGit(t *testing.T) {
	t.Parallel()

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
//
//nolint:paralleltest // swaps the package-level relPath seam
func TestRelMetadataPathReportsFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	rel, err := relMetadataPath(wsRoot, "/ws/meta.yml")
	iox.Discard(rel)
	assertFails(t, err)
}

// TestTryLegacyMetadataSkipsWhenAlreadyLegacy verifies legacy path short-circuits.
func TestTryLegacyMetadataSkipsWhenAlreadyLegacy(t *testing.T) {
	t.Parallel()

	meta, found, err := tryLegacyMetadata(t.TempDir(), config.LegacyMetadataPath)
	iox.Discard(meta)

	if found || !errors.Is(err, errMetadataNotFound) {
		t.Fatalf("found=%t err=%v", found, err)
	}
}

//nolint:ireturn // interface required by stdlib signature
func (fakeDirEntry) Info() (os.FileInfo, error) { return nil, errStub }
func (entry fakeDirEntry) IsDir() bool          { return entry.dir }
func (entry fakeDirEntry) Name() string         { return entry.name }

func (fakeDirEntry) Type() fs.FileMode { return consts.IndexZero }
