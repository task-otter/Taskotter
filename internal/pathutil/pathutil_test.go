// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/pathutil"
)

const (
	folderTaskfiles = "taskfiles"
	folderTask      = "task"
	pathTaskfileYML = "Taskfile.yml"
	pathGoTaskfile  = "taskfiles/go/Taskfile.yml"
	pathErrorMsgBad = "bad"
)

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
		{"README.md", false},
		{"latest.txt", false},
	}

	for _, testCase := range cases {
		got := pathutil.IsTestPath(testCase.path)

		if got != testCase.want {
			t.Fatalf("IsTestPath(%q) = %t, want %t", testCase.path, got, testCase.want)
		}
	}
}

func TestIsModuleMetadataPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{"metadata.yml", true},
		{"docs/metadata.yml", false},
		{"metadata.yaml", false},
		{"Taskfile.yml", false},
	}

	for _, testCase := range cases {
		got := pathutil.IsModuleMetadataPath(testCase.path)

		if got != testCase.want {
			t.Fatalf("IsModuleMetadataPath(%q) = %t, want %t", testCase.path, got, testCase.want)
		}
	}
}

func TestHasFolderPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path   string
		folder string
		want   bool
	}{
		{"taskfiles/go", folderTaskfiles, true},
		{pathGoTaskfile, folderTaskfiles, true},
		{folderTask, folderTask, true},
		{"taskfiles-extra/foo", folderTask, false},
		{"task/extra", folderTaskfiles, false},
		{"", folderTaskfiles, false},
	}

	for _, testCase := range cases {
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

func TestValidateRelativePathRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cases := []string{
		"",
		"///",
		"..",
		"../outside",
		"taskfiles/../../outside",
		"/etc/passwd",
		`C:\taskfiles`,
	}

	for _, rel := range cases {
		_, err := pathutil.ValidateRelativePath(root, rel)
		if err == nil {
			t.Fatalf("ValidateRelativePath(%q) expected error", rel)
		}
	}
}

func TestReadRelativeFileMissingReturnsNotExist(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	_, err := pathutil.ReadRelativeFile(root, "Taskfile.yml")

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadRelativeFile() err = %v, want ErrNotExist", err)
	}
}

func TestReadRelativeFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := pathGoTaskfile

	want := []byte("version: \"3\"\n")

	err := os.MkdirAll(filepath.Join(root, "taskfiles", "go"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), want, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	got, err := pathutil.ReadRelativeFile(root, rel)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("ReadRelativeFile() = %q, want %q", got, want)
	}
}

func TestNormalizeSlashes(t *testing.T) {
	t.Parallel()

	got := pathutil.NormalizeSlashes(` //taskfiles\\go///Taskfile.yml `)

	if got != pathGoTaskfile {
		t.Fatalf("NormalizeSlashes() = %q", got)
	}
}

func TestPathErrorString(t *testing.T) {
	t.Parallel()

	withField := (&pathutil.PathError{Field: "tasks", Value: "", Message: pathErrorMsgBad}).Error()

	if withField != "tasks: bad" {
		t.Fatalf("Error() = %q", withField)
	}

	withoutField := (&pathutil.PathError{Field: "", Value: "", Message: pathErrorMsgBad}).Error()

	if withoutField != pathErrorMsgBad {
		t.Fatalf("Error() = %q", withoutField)
	}
}

func TestValidateTaskName(t *testing.T) {
	t.Parallel()

	valid := []string{"go", "eslint-config", "a1"}

	for _, name := range valid {
		err := pathutil.ValidateTaskName(name)
		if err != nil {
			t.Fatalf("ValidateTaskName(%q) error = %v", name, err)
		}
	}

	invalid := []string{"", "Go", "go/test", `go\test`, "go..test", "-go"}

	for _, name := range invalid {
		err := pathutil.ValidateTaskName(name)
		if err == nil {
			t.Fatalf("ValidateTaskName(%q) expected error", name)
		}
	}
}

func TestValidateTargetFolder(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	got, err := pathutil.ValidateTargetFolder(` taskfiles\\go// `, workspace)
	if err != nil {
		t.Fatal(err)
	}

	if got != "taskfiles/go" {
		t.Fatalf("ValidateTargetFolder() = %q", got)
	}
}

func TestValidateTargetFolderRejectsUnsafePaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cases := []string{
		"",
		"/tmp/taskfiles",
		`C:\taskfiles`,
		"../taskfiles",
		".git",
		".git/hooks",
		".github/actions",
		".github/actions/taskotter",
	}

	for _, raw := range cases {
		_, err := pathutil.ValidateTargetFolder(raw, workspace)
		if err == nil {
			t.Fatalf("ValidateTargetFolder(%q) expected error", raw)
		}
	}
}

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

func TestJoinRelativeAndWorkspacePath(t *testing.T) {
	t.Parallel()

	got := pathutil.JoinRelative("taskfiles", "go", pathTaskfileYML)

	if got != pathGoTaskfile {
		t.Fatalf("JoinRelative() = %q", got)
	}

	workspace := t.TempDir()

	want := filepath.Join(workspace, "taskfiles", "go")

	if got := pathutil.WorkspacePath(workspace, "taskfiles/go"); got != want {
		t.Fatalf("WorkspacePath() = %q, want %q", got, want)
	}
}

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

func TestOpenRelativeFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := pathGoTaskfile

	err := os.MkdirAll(filepath.Join(root, "taskfiles", "go"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(root, filepath.FromSlash(rel)),
		[]byte("version: \"3\"\n"),
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}

	file, err := pathutil.OpenRelativeFile(root, rel)
	if err != nil {
		t.Fatal(err)
	}

	err = file.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenRelativeFileRejectsUnsafePath(t *testing.T) {
	t.Parallel()

	_, err := pathutil.OpenRelativeFile(t.TempDir(), "../outside")
	if err == nil {
		t.Fatal("expected unsafe path rejection")
	}
}

func TestIsDocPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{"README.md", true},
		{"docs/setup.md", true},
		{"go/docs/setup.md", true},
		{"go/README.md", false},
		{pathTaskfileYML, false},
	}

	for _, testCase := range cases {
		got := pathutil.IsDocPath(testCase.path)

		if got != testCase.want {
			t.Fatalf("IsDocPath(%q) = %t, want %t", testCase.path, got, testCase.want)
		}
	}
}
