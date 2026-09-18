// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	"github.com/task-otter/Taskotter/internal/testsupport"
)

const (
	emptyAfterNorm = `\\`
	linkName       = "link"
	wantErrorMsg   = "expected error, got %v"

	fmtTargetFolder = "ValidateTargetFolder() = %q"
	testFieldTasks  = "tasks"

	folderTaskfiles = "taskfiles"
	folderTask      = "task"
	pathTaskfileYML = "Taskfile.yml"
	pathGoTaskfile  = "taskfiles/go/Taskfile.yml"
	pathErrorMsgBad = "bad"

	testOutsidePath    = "../outside"
	testWindowsAbsPath = `C:\taskfiles`
	fmtErrorEquals     = "Error() = %q"
	testTaskfilesGo    = "taskfiles/go"
	taskfileVersion3   = "version: \"3\"\n"
)

// TestValidateTargetFolderAcceptsMissingWorkspace verifies an unresolvable workspace still validates.
func TestValidateTargetFolderAcceptsMissingWorkspace(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

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
	lockPathutilTestState(t)

	got, err := pathutil.ValidateTargetFolder(emptyAfterNorm, t.TempDir())
	if err == nil {
		t.Fatalf(wantErrorMsg, got)
	}
}

// TestValidateRelativePathRejectsEmptyAfterNormalization verifies separator-only input is rejected.
func TestValidateRelativePathRejectsEmptyAfterNormalization(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	got, err := pathutil.ValidateRelativePath(t.TempDir(), emptyAfterNorm)
	if err == nil {
		t.Fatalf(wantErrorMsg, got)
	}
}

// TestValidateTargetFolderFollowsInternalSymlink verifies a symlink inside the workspace is accepted.
func TestValidateTargetFolderFollowsInternalSymlink(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	workspace := t.TempDir()
	linkTargetSymlink(t, workspace)

	got, err := pathutil.ValidateTargetFolder(linkName+"/"+testFieldTasks, workspace)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if got != linkName+"/"+testFieldTasks {
		t.Fatalf(fmtTargetFolder, got)
	}
}

// TestValidateTargetFolderRejectsDanglingSymlink verifies an unresolvable symlink is rejected.
func TestValidateTargetFolderRejectsDanglingSymlink(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	workspace := t.TempDir()
	symlinkOrSkip(t, filepath.Join(workspace, "nowhere"), filepath.Join(workspace, linkName))

	got, err := pathutil.ValidateTargetFolder(linkName+"/"+testFieldTasks, workspace)
	if err == nil {
		t.Fatalf(wantErrorMsg, got)
	}
}

// TestValidateTargetFolderReportsStatFailure verifies an unreadable path component is rejected.
func TestValidateTargetFolderReportsStatFailure(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	if os.Geteuid() == consts.IndexZero {
		t.Skip("running as root: permission errors are not reproducible")
	}

	workspace := t.TempDir()
	blockDir(t, filepath.Join(workspace, "blocked"))

	got, err := pathutil.ValidateTargetFolder("blocked/"+testFieldTasks, workspace)
	if err == nil {
		t.Fatalf(wantErrorMsg, got)
	}
}

// TestReadRelativeFileRejectsUnsafePath verifies traversal is rejected before reading.
func TestReadRelativeFileRejectsUnsafePath(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	data, err := pathutil.ReadRelativeFile(t.TempDir(), testOutsidePath)
	if err == nil {
		t.Fatalf(wantErrorMsg, data)
	}
}

// TestOpenRelativeFileMissingFile verifies opening a missing file reports an error.
func TestOpenRelativeFileMissingFile(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

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
	mkdirOrFail(t, filepath.Join(target, testFieldTasks), consts.FilePerm755)
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

func lockPathutilTestState(t *testing.T) {
	t.Helper()
	t.Cleanup(testsupport.Lock())
}

// TestIsTestPath verifies test file paths are identified regardless of directory nesting.
func TestIsTestPath(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)
	assertBoolCases(
		t,
		&boolAssert{name: "IsTestPath", cases: isTestPathCases(), fn: pathutil.IsTestPath},
	)
}

func isTestPathCases() []boolCase {
	return []boolCase{
		{"go_test.go", true},
		{"eslint_test.ts", true},
		{"nested/pkg/foo_test.go", true},
		{pathTaskfileYML, false},
		{consts.ReadmeMD, false},
		{"latest.txt", false},
	}
}

// TestIsModuleMetadataPath verifies only the top-level metadata.yml path is recognized.
func TestIsModuleMetadataPath(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

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
	lockPathutilTestState(t)
	assertFolderPrefixCases(t, hasFolderPrefixCases())
}

func hasFolderPrefixCases() []folderPrefixCase {
	return []folderPrefixCase{
		{testTaskfilesGo, folderTaskfiles, true},
		{pathGoTaskfile, folderTaskfiles, true},
		{folderTask, folderTask, true},
		{"taskfiles-extra/foo", folderTask, false},
		{"task/extra", folderTaskfiles, false},
		{consts.Empty, folderTaskfiles, false},
	}
}

func assertFolderPrefixCases(t *testing.T, cases []folderPrefixCase) {
	t.Helper()

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

func assertBoolCases(t *testing.T, spec *boolAssert) {
	t.Helper()

	for i := range spec.cases {
		testCase := &spec.cases[i]
		got := spec.fn(testCase.path)

		if got != testCase.want {
			t.Fatalf("%s(%q) = %t, want %t", spec.name, testCase.path, got, testCase.want)
		}
	}
}

// TestValidateRelativePathRejectsTraversal verifies unsafe or escaping relative paths are rejected.
func TestValidateRelativePathRejectsTraversal(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

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
		path, err := pathutil.ValidateRelativePath(root, cases[i])
		if err == nil {
			t.Fatalf("ValidateRelativePath(%q) = %q, expected error", cases[i], path)
		}
	}
}

// TestReadRelativeFileMissingReturnsNotExist verifies a missing file returns an [os.ErrNotExist] error.
func TestReadRelativeFileMissingReturnsNotExist(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	root := t.TempDir()

	data, err := pathutil.ReadRelativeFile(root, "Taskfile.yml")

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadRelativeFile() err = %v, want ErrNotExist (data=%q)", err, data)
	}
}

func writeTaskfileFixture(t *testing.T, fix *taskfileFixture) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(fix.root, folderTaskfiles, consts.Go), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(fix.root, filepath.FromSlash(fix.rel)),
		fix.data,
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}
}

// TestReadRelativeFile verifies a file's contents are read via a validated relative path.
func TestReadRelativeFile(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	root := t.TempDir()
	rel := pathGoTaskfile
	want := []byte(taskfileVersion3)

	writeTaskfileFixture(t, &taskfileFixture{root: root, rel: rel, data: want})

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
	lockPathutilTestState(t)

	got := pathutil.NormalizeSlashes(` //taskfiles\\go///Taskfile.yml `)

	if got != pathGoTaskfile {
		t.Fatalf("NormalizeSlashes() = %q", got)
	}
}

// TestPathErrorString verifies Error() includes the field prefix when set and omits it otherwise.
func TestPathErrorString(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	withField := (&pathutil.PathError{Field: testFieldTasks, Value: consts.Empty, Message: pathErrorMsgBad}).Error()

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
	lockPathutilTestState(t)

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
	lockPathutilTestState(t)

	workspace := t.TempDir()

	got, err := pathutil.ValidateTargetFolder(` taskfiles\\go// `, workspace)
	if err != nil {
		t.Fatal(err)
	}

	if got != testTaskfilesGo {
		t.Fatalf(fmtTargetFolder, got)
	}
}

// TestValidateTargetFolderRejectsUnsafePaths verifies absolute, traversal, and reserved paths are rejected.
func TestValidateTargetFolderRejectsUnsafePaths(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

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
		folder, err := pathutil.ValidateTargetFolder(cases[i], workspace)
		if err == nil {
			t.Fatalf("ValidateTargetFolder(%q) = %q, expected error", cases[i], folder)
		}
	}
}

// TestValidateTargetFolderRejectsEscapingSymlink verifies a symlink escaping the workspace is rejected.
func TestValidateTargetFolderRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	workspace := t.TempDir()
	outside := t.TempDir()

	err := os.Symlink(outside, filepath.Join(workspace, "linked"))
	if err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	folder, err := pathutil.ValidateTargetFolder("linked/taskfiles", workspace)
	if err == nil {
		t.Fatalf("expected symlink escape rejection, got %q", folder)
	}
}

// TestJoinRelativeAndWorkspacePath verifies relative and workspace-absolute path joining.
func TestJoinRelativeAndWorkspacePath(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	got := pathutil.JoinRelative(folderTaskfiles, consts.Go, pathTaskfileYML)

	if got != pathGoTaskfile {
		t.Fatalf("JoinRelative() = %q", got)
	}

	workspace := t.TempDir()

	want := filepath.Join(workspace, folderTaskfiles, consts.Go)

	workspacePath := pathutil.WorkspacePath(workspace, testTaskfilesGo)

	if workspacePath != want {
		t.Fatalf("WorkspacePath() = %q, want %q", workspacePath, want)
	}
}

// TestValidateRelativePathNormalizesValidPath verifies a messy but valid relative path is normalized.
func TestValidateRelativePathNormalizesValidPath(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

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
	lockPathutilTestState(t)

	root := t.TempDir()
	rel := pathGoTaskfile

	writeTaskfileFixture(t, &taskfileFixture{root: root, rel: rel, data: []byte(taskfileVersion3)})

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
	lockPathutilTestState(t)

	file, err := pathutil.OpenRelativeFile(t.TempDir(), testOutsidePath)
	if err != nil {
		return
	}

	closeErr := file.Close()
	if closeErr != nil {
		t.Fatalf("Close() after unsafe open: %v", closeErr)
	}

	t.Fatal("expected unsafe path rejection")
}

// TestIsDocPath verifies README and docs-folder paths are identified as documentation.
func TestIsDocPath(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)
	assertBoolCases(
		t,
		&boolAssert{name: "IsDocPath", cases: isDocPathCases(), fn: pathutil.IsDocPath},
	)
}

func isDocPathCases() []boolCase {
	return []boolCase{
		{consts.ReadmeMD, true},
		{"docs/setup.md", true},
		{"docs/a/b.md", true},
		{"go/docs/setup.md", true},
		{"go/README.md", false},
		{"mydocs/x", false},
		{"docs", false},
		{"readme.md", false},
		{pathTaskfileYML, false},
	}
}

// TestPathErrorFieldNameAndDetail covers FieldName and Detail helpers.
func TestPathErrorFieldNameAndDetail(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	err := &pathutil.PathError{Field: testFieldTasks, Value: "bad", Message: "nope"}

	if err.FieldName() != testFieldTasks {
		t.Fatalf("FieldName = %q", err.FieldName())
	}

	if err.Detail() == "" {
		t.Fatal("Detail empty")
	}
}

// TestPathErrorFieldNameEmptyExtras covers FieldName when value and message are empty.
func TestPathErrorFieldNameEmptyExtras(t *testing.T) {
	t.Parallel()
	lockPathutilTestState(t)

	err := &pathutil.PathError{Field: testFieldTasks, Value: consts.Empty, Message: consts.Empty}

	if err.FieldName() != testFieldTasks {
		t.Fatalf("FieldName() = %q", err.FieldName())
	}
}
