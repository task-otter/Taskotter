// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
)

// TestCleanupTempFileRunsWhenFlagged verifies cleanup closes and removes temps.
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

// TestCopyFileCopiesRelativeSource verifies CopyFile reads and writes successfully.
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

// TestCopyFileReportsMissingSource verifies missing sources fail.
func TestCopyFileReportsMissingSource(t *testing.T) {
	t.Parallel()

	err := CopyFile(&copyFileArgs{
		root: t.TempDir(), rel: fileNameTxt, dst: filepath.Join(t.TempDir(), outName),
		mode: fileModeRegular,
	})

	assertFails(t, err)
}

// TestCopyFileReportsWriteFailure verifies destination write failures surface.
//
//nolint:paralleltest // swaps the package-level mkdirAll seam
func TestCopyFileReportsWriteFailure(t *testing.T) {
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
//
//nolint:paralleltest // swaps the package-level mkdirAll seam
func TestCopyFileToReportsWriteFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	err := copyFileTo(filepath.Join(t.TempDir(), pathA, fileNameTxt), &domain.FileEntry{
		Data: []byte(byteX), Mode: fileModeRegular,
	})

	assertFails(t, err)
}

// TestCreateTempFileReportsCreateFailure verifies CreateTemp failures surface.
//
//nolint:paralleltest // swaps the package-level createTemp seam
func TestCreateTempFileReportsCreateFailure(t *testing.T) {
	swapCreateTemp(t, failingCreateTemp)

	file, err := createTempFile(t.TempDir())
	discardTemp(file)
	assertFails(t, err)
}

// TestCreateTempFileReportsMkdirFailure verifies parent mkdir failures surface.
//
//nolint:paralleltest // swaps the package-level mkdirAll seam
func TestCreateTempFileReportsMkdirFailure(t *testing.T) {
	swapMkdirAll(t, failingMkdirAll)

	file, err := createTempFile(filepath.Join(t.TempDir(), "nested"))
	discardTemp(file)
	assertFails(t, err)
}

// TestReadRelativeFileReportsOpenFailure verifies open failures surface.
//
//nolint:paralleltest // swaps the package-level openRelativeFile seam
func TestReadRelativeFileReportsOpenFailure(t *testing.T) {
	swapOpenRelative(t, failingOpenRelative)

	data, err := readRelativeFile(t.TempDir(), fileNameTxt)
	iox.Discard(data)
	assertFails(t, err)
}

// TestReadRelativeFileReportsReadFailure verifies read failures surface.
//
//nolint:paralleltest // swaps the package-level readAll seam
func TestReadRelativeFileReportsReadFailure(t *testing.T) {
	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(byteX))
	swapReadAll(t, failingReadAll)

	data, err := readRelativeFile(root, fileNameTxt)
	iox.Discard(data)
	assertFails(t, err)
}

// TestRenameTempFileReportsFailure verifies rename failures surface.
//
//nolint:paralleltest // swaps the package-level renamePath seam
func TestRenameTempFileReportsFailure(t *testing.T) {
	swapRenamePath(t, failingRename)

	assertFails(t, renameTempFile(pathA, pathB))
}

// TestWriteAndFinalizeTempReportsChmodFailure verifies chmod failures surface.
//
//nolint:paralleltest // swaps the package-level chmodFile seam
func TestWriteAndFinalizeTempReportsChmodFailure(t *testing.T) {
	swapChmodFile(t, failingChmod)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
}

// TestWriteAndFinalizeTempReportsCloseFailure verifies close failures surface.
//
//nolint:paralleltest // swaps the package-level closeFile seam
func TestWriteAndFinalizeTempReportsCloseFailure(t *testing.T) {
	swapCloseFile(t, failingClose)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
}

// TestWriteAndFinalizeTempReportsWriteFailure verifies writeFull failures surface.
//
//nolint:paralleltest // swaps the package-level writeFull seam
func TestWriteAndFinalizeTempReportsWriteFailure(t *testing.T) {
	swapWriteFull(t, failingWriteFull)

	tmp := createRealTemp(t)
	assertFails(t, writeAndFinalizeTemp(tmp, []byte(byteX), fileModeRegular))
}

// TestWriteFileAtomicReportsFinalizeFailure verifies rename failures abort atomic writes.
//
//nolint:paralleltest // swaps the package-level renamePath seam
func TestWriteFileAtomicReportsFinalizeFailure(t *testing.T) {
	swapRenamePath(t, failingRename)

	path := filepath.Join(t.TempDir(), fileNameTxt)
	assertFails(t, writeFileAtomic(path, []byte(byteX), fileModeRegular))
}

// TestWriteFullStubWriterUsedDocumentsFaults verifies StubWriter stays referenced.
func TestWriteFullStubWriterUsedDocumentsFaults(t *testing.T) {
	t.Parallel()

	writer := &faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault}
	assertFails(t, writeFull(writer, []byte(byteX)))
}

func assertFilePayload(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // test-owned path
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
