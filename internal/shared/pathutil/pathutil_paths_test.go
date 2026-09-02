// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

const (
	emptyAfterNorm = `\\`
	folderTasks    = "tasks"
	linkName       = "link"
	wantErrorMsg   = "expected error, got %v"

	fmtTargetFolder = "ValidateTargetFolder() = %q"
)

// TestValidateTargetFolderAcceptsMissingWorkspace verifies an unresolvable workspace still validates.
func TestValidateTargetFolderAcceptsMissingWorkspace(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(t.TempDir(), "missing")

	got, err := pathutil.ValidateTargetFolder(folderTaskfiles, workspace)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if got != folderTaskfiles {
		t.Fatalf("ValidateTargetFolder() = %q, want %q", got, folderTaskfiles)
	}
}

// TestValidateTargetFolderRejectsEmptyAfterNormalization verifies separator-only input is rejected.
func TestValidateTargetFolderRejectsEmptyAfterNormalization(t *testing.T) {
	t.Parallel()

	got, err := pathutil.ValidateTargetFolder(emptyAfterNorm, t.TempDir())
	if err == nil {
		t.Fatalf(wantErrorMsg, got)
	}
}

// TestValidateRelativePathRejectsEmptyAfterNormalization verifies separator-only input is rejected.
func TestValidateRelativePathRejectsEmptyAfterNormalization(t *testing.T) {
	t.Parallel()

	got, err := pathutil.ValidateRelativePath(t.TempDir(), emptyAfterNorm)
	if err == nil {
		t.Fatalf(wantErrorMsg, got)
	}
}

// TestValidateTargetFolderFollowsInternalSymlink verifies a symlink inside the workspace is accepted.
func TestValidateTargetFolderFollowsInternalSymlink(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	linkTargetSymlink(t, workspace)

	got, err := pathutil.ValidateTargetFolder(linkName+"/"+folderTasks, workspace)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if got != linkName+"/"+folderTasks {
		t.Fatalf(fmtTargetFolder, got)
	}
}

// TestValidateTargetFolderRejectsDanglingSymlink verifies an unresolvable symlink is rejected.
func TestValidateTargetFolderRejectsDanglingSymlink(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	symlinkOrSkip(t, filepath.Join(workspace, "nowhere"), filepath.Join(workspace, linkName))

	got, err := pathutil.ValidateTargetFolder(linkName+"/"+folderTasks, workspace)
	if err == nil {
		t.Fatalf(wantErrorMsg, got)
	}
}

// TestValidateTargetFolderReportsStatFailure verifies an unreadable path component is rejected.
func TestValidateTargetFolderReportsStatFailure(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == consts.IndexZero {
		t.Skip("running as root: permission errors are not reproducible")
	}

	workspace := t.TempDir()
	blockDir(t, filepath.Join(workspace, "blocked"))

	got, err := pathutil.ValidateTargetFolder("blocked/"+folderTasks, workspace)
	if err == nil {
		t.Fatalf(wantErrorMsg, got)
	}
}

// TestReadRelativeFileRejectsUnsafePath verifies traversal is rejected before reading.
func TestReadRelativeFileRejectsUnsafePath(t *testing.T) {
	t.Parallel()

	data, err := pathutil.ReadRelativeFile(t.TempDir(), testOutsidePath)
	if err == nil {
		t.Fatalf(wantErrorMsg, data)
	}
}

// TestOpenRelativeFileMissingFile verifies opening a missing file reports an error.
func TestOpenRelativeFileMissingFile(t *testing.T) {
	t.Parallel()

	file, err := pathutil.OpenRelativeFile(t.TempDir(), pathTaskfileYML)
	if err == nil {
		iox.Discard(file.Close())
		t.Fatal("expected error for missing file")
	}
}

func blockDir(t *testing.T, path string) {
	t.Helper()

	mkdirOrFail(t, path, consts.FilePerm755)
	chmodOrFail(t, path, consts.IndexZero)
	t.Cleanup(func() { iox.Discard(os.Chmod(path, consts.FilePerm755)) })
}

func chmodOrFail(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	err := os.Chmod(path, mode)
	if err != nil {
		t.Fatal(err)
	}
}

func linkTargetSymlink(t *testing.T, workspace string) {
	t.Helper()

	target := filepath.Join(workspace, "real")
	mkdirOrFail(t, filepath.Join(target, folderTasks), consts.FilePerm755)
	symlinkOrSkip(t, target, filepath.Join(workspace, linkName))
}

func mkdirOrFail(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	err := os.MkdirAll(path, mode)
	if err != nil {
		t.Fatal(err)
	}
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()

	err := os.Symlink(target, link)
	if err != nil {
		t.Skip("symlink not permitted")
	}
}
