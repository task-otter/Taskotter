package pathutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/pathutil"
)

const (
	folderTaskfiles = "taskfiles"
	folderTask      = "task"
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
		{"Taskfile.yml", false},
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
		{"taskfiles/go/Taskfile.yml", folderTaskfiles, true},
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
		"..",
		"../outside",
		"taskfiles/../../outside",
		"/etc/passwd",
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
	rel := "taskfiles/go/Taskfile.yml"

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

	if string(got) != string(want) {
		t.Fatalf("ReadRelativeFile() = %q, want %q", got, want)
	}
}
