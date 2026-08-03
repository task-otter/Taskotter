// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
)

const (
	folderTaskfiles = "taskfiles"
	folderTask      = "task"
	pathTaskfileYML = "Taskfile.yml"
	pathGoTaskfile  = "taskfiles/go/Taskfile.yml"
	pathErrorMsgBad = "bad"

	testOutsidePath    = "../outside"
	testWindowsAbsPath = `C:\taskfiles`
	fmtErrorEquals     = "Error() = %q"
	testTaskfilesGo    = "taskfiles/go"
)

// TestIsTestPath verifies test file paths are identified regardless of directory nesting.
func TestIsTestPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{"go_test.go", true},
		{"eslint_test.ts", true},
		{"nested/pkg/foo_test.go", true},
		{pathTaskfileYML, false},
		{consts.ReadmeMD, false},
		{"latest.txt", false},
	}

	for i := range cases {
		testCase := &cases[i]
		got := pathutil.IsTestPath(testCase.path)

		if got != testCase.want {
			t.Fatalf("IsTestPath(%q) = %t, want %t", testCase.path, got, testCase.want)
		}
	}
}

// TestIsModuleMetadataPath verifies only the top-level metadata.yml path is recognized.
func TestIsModuleMetadataPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{"metadata.yml", true},
		{"docs/metadata.yml", false},
		{"metadata.yaml", false},
		{pathTaskfileYML, false},
	}

	for i := range cases {
		testCase := &cases[i]
		got := pathutil.IsModuleMetadataPath(testCase.path)

		if got != testCase.want {
			t.Fatalf("IsModuleMetadataPath(%q) = %t, want %t", testCase.path, got, testCase.want)
		}
	}
}

// TestHasFolderPrefix verifies exact and child-path folder prefix matching.
func TestHasFolderPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path   string
		folder string
		want   bool
	}{
		{testTaskfilesGo, folderTaskfiles, true},
		{pathGoTaskfile, folderTaskfiles, true},
		{folderTask, folderTask, true},
		{"taskfiles-extra/foo", folderTask, false},
		{"task/extra", folderTaskfiles, false},
		{consts.Empty, folderTaskfiles, false},
	}

	for i := range cases {
		testCase := &cases[i]
		got := pathutil.HasFolderPrefix(testCase.path, testCase.folder)

		if got != testCase.want {
			t.Fatalf(
				"HasFolderPrefix(%q, %q) = %t, want %t",
				testCase.path,
				testCase.folder,
				got,
				testCase.want,
			)
		}
	}
}

// TestValidateRelativePathRejectsTraversal verifies unsafe or escaping relative paths are rejected.
func TestValidateRelativePathRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cases := []string{
		consts.Empty,
		"///",
		consts.PathParent,
		testOutsidePath,
		"taskfiles/../../outside",
		"/etc/passwd",
		testWindowsAbsPath,
	}

	for i := range cases {
		_, err := pathutil.ValidateRelativePath(root, cases[i])
		if err == nil {
			t.Fatalf("ValidateRelativePath(%q) expected error", cases[i])
		}
	}
}

// TestReadRelativeFileMissingReturnsNotExist verifies a missing file returns an [os.ErrNotExist] error.
func TestReadRelativeFileMissingReturnsNotExist(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	_, err := pathutil.ReadRelativeFile(root, "Taskfile.yml")

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadRelativeFile() err = %v, want ErrNotExist", err)
	}
}

func writeTaskfileFixture(t *testing.T, root, rel string, data []byte) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(root, folderTaskfiles, consts.Go), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), data, consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}
}

// TestReadRelativeFile verifies a file's contents are read via a validated relative path.
func TestReadRelativeFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := pathGoTaskfile
	want := []byte("version: \"3\"\n")

	writeTaskfileFixture(t, root, rel, want)

	got, err := pathutil.ReadRelativeFile(root, rel)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("ReadRelativeFile() = %q, want %q", got, want)
	}
}

// TestNormalizeSlashes verifies mixed and redundant separators normalize to a clean path.
func TestNormalizeSlashes(t *testing.T) {
	t.Parallel()

	got := pathutil.NormalizeSlashes(` //taskfiles\\go///Taskfile.yml `)

	if got != pathGoTaskfile {
		t.Fatalf("NormalizeSlashes() = %q", got)
	}
}

// TestPathErrorString verifies Error() includes the field prefix when set and omits it otherwise.
func TestPathErrorString(t *testing.T) {
	t.Parallel()

	withField := (&pathutil.PathError{Field: "tasks", Value: consts.Empty, Message: pathErrorMsgBad}).Error()

	if withField != "tasks: bad" {
		t.Fatalf(fmtErrorEquals, withField)
	}

	withoutField := (&pathutil.PathError{
		Field:   consts.Empty,
		Value:   consts.Empty,
		Message: pathErrorMsgBad,
	}).Error()

	if withoutField != pathErrorMsgBad {
		t.Fatalf(fmtErrorEquals, withoutField)
	}
}

// TestValidateTaskName verifies safe task names are accepted and unsafe ones are rejected.
func TestValidateTaskName(t *testing.T) {
	t.Parallel()

	valid := []string{consts.Go, "eslint-config", "a1"}
	assertAllTaskNamesValid(t, valid)

	invalid := []string{consts.Empty, "Go", "go/test", `go\test`, "go..test", "-go"}
	assertAllTaskNamesInvalid(t, invalid)
}

func assertAllTaskNamesValid(t *testing.T, names []string) {
	t.Helper()

	for i := range names {
		err := pathutil.ValidateTaskName(names[i])
		if err != nil {
			t.Fatalf("ValidateTaskName(%q) error = %v", names[i], err)
		}
	}
}

func assertAllTaskNamesInvalid(t *testing.T, names []string) {
	t.Helper()

	for i := range names {
		err := pathutil.ValidateTaskName(names[i])
		if err == nil {
			t.Fatalf("ValidateTaskName(%q) expected error", names[i])
		}
	}
}

// TestValidateTargetFolder verifies a messy target folder input normalizes to a clean path.
func TestValidateTargetFolder(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	got, err := pathutil.ValidateTargetFolder(` taskfiles\\go// `, workspace)
	if err != nil {
		t.Fatal(err)
	}

	if got != testTaskfilesGo {
		t.Fatalf("ValidateTargetFolder() = %q", got)
	}
}

// TestValidateTargetFolderRejectsUnsafePaths verifies absolute, traversal, and reserved paths are rejected.
func TestValidateTargetFolderRejectsUnsafePaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cases := []string{
		consts.Empty,
		"/tmp/taskfiles",
		testWindowsAbsPath,
		"../taskfiles",
		".git",
		".git/hooks",
		".github/actions",
		".github/actions/taskotter",
	}

	for i := range cases {
		_, err := pathutil.ValidateTargetFolder(cases[i], workspace)
		if err == nil {
			t.Fatalf("ValidateTargetFolder(%q) expected error", cases[i])
		}
	}
}

// TestValidateTargetFolderRejectsEscapingSymlink verifies a symlink escaping the workspace is rejected.
func TestValidateTargetFolderRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outside := t.TempDir()

	err := os.Symlink(outside, filepath.Join(workspace, "linked"))
	if err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err = pathutil.ValidateTargetFolder("linked/taskfiles", workspace)
	if err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}

// TestJoinRelativeAndWorkspacePath verifies relative and workspace-absolute path joining.
func TestJoinRelativeAndWorkspacePath(t *testing.T) {
	t.Parallel()

	got := pathutil.JoinRelative(folderTaskfiles, consts.Go, pathTaskfileYML)

	if got != pathGoTaskfile {
		t.Fatalf("JoinRelative() = %q", got)
	}

	workspace := t.TempDir()

	want := filepath.Join(workspace, folderTaskfiles, consts.Go)

	if got := pathutil.WorkspacePath(workspace, testTaskfilesGo); got != want {
		t.Fatalf("WorkspacePath() = %q, want %q", got, want)
	}
}

// TestValidateRelativePathNormalizesValidPath verifies a messy but valid relative path is normalized.
func TestValidateRelativePathNormalizesValidPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	got, err := pathutil.ValidateRelativePath(root, ` taskfiles\\go//Taskfile.yml `)
	if err != nil {
		t.Fatal(err)
	}

	if got != pathGoTaskfile {
		t.Fatalf("ValidateRelativePath() = %q", got)
	}
}

// TestOpenRelativeFile verifies a file is opened successfully via a validated relative path.
func TestOpenRelativeFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := pathGoTaskfile

	writeTaskfileFixture(t, root, rel, []byte("version: \"3\"\n"))

	file, err := pathutil.OpenRelativeFile(root, rel)
	if err != nil {
		t.Fatal(err)
	}

	err = file.Close()
	if err != nil {
		t.Fatal(err)
	}
}

// TestOpenRelativeFileRejectsUnsafePath verifies an unsafe relative path is rejected before opening.
func TestOpenRelativeFileRejectsUnsafePath(t *testing.T) {
	t.Parallel()

	_, err := pathutil.OpenRelativeFile(t.TempDir(), testOutsidePath)
	if err == nil {
		t.Fatal("expected unsafe path rejection")
	}
}

// TestIsDocPath verifies README and docs-folder paths are identified as documentation.
func TestIsDocPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{consts.ReadmeMD, true},
		{"docs/setup.md", true},
		{"go/docs/setup.md", true},
		{"go/README.md", false},
		{pathTaskfileYML, false},
	}

	for i := range cases {
		testCase := &cases[i]
		got := pathutil.IsDocPath(testCase.path)

		if got != testCase.want {
			t.Fatalf("IsDocPath(%q) = %t, want %t", testCase.path, got, testCase.want)
		}
	}
}
