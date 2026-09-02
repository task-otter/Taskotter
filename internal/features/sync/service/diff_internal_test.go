// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	lockRelPath = "taskfiles/.taskotter-lock.yml"
	metaRelPath = "taskfiles/.taskotter/metadata.yml"
)

// TestApplyFileChangeUnchangedIsNoOp verifies unchanged kinds leave lists untouched.
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

// TestDiffLockFileReportsReadFailure verifies lock path read failures surface.
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

// TestDiffManagedFilePathsReportsFailure verifies managed-file read failures surface.
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

// TestDiffMetadataFileReportsReadFailure verifies metadata path read failures surface.
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

// TestFileChangeFromDataUnchanged verifies matching hashes stay unchanged.
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

// TestFileChangedReportsReadFailure verifies non-missing read errors surface.
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

// TestLockContentChangedDetectsDifference verifies lock payloads are compared.
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

// TestMarshalLockForCompareNilReturnsNil verifies nil locks marshal to nil bytes.
func TestMarshalLockForCompareNilReturnsNil(t *testing.T) {
	t.Parallel()

	data, err := marshalLockForCompare(nil)
	assertNoErr(t, err)

	if data != nil {
		t.Fatalf("data = %v, want nil", data)
	}
}
