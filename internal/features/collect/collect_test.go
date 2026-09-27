// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package collect

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	fileMode       = 0o755
	taskfileName   = "Taskfile.yml"
	runFileName    = "run.go"
	readmeFileName = "README.md"
)

func TestDefaultCollectorAppliesFilePolicy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for name, contents := range map[string]string{
		taskfileName:   "version: '3'\n",
		runFileName:    "package run\n",
		"run_test.go":  "package run\n",
		readmeFileName: "docs\n",
	} {
		path := filepath.Join(dir, name)

		err := os.WriteFile(path, []byte(contents), fileMode)
		if err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	withoutDocs, err := (DefaultCollector{}).Collect(Options{SourceDir: dir})
	if err != nil {
		t.Fatalf("Collect() without docs error = %v", err)
	}

	got := fileNames(withoutDocs)

	assertFileNames(t, got, []string{taskfileName, runFileName})

	withDocs, err := (DefaultCollector{}).Collect(Options{SourceDir: dir, IncludeDocs: true})
	if err != nil {
		t.Fatalf("Collect() with docs error = %v", err)
	}

	got = fileNames(withDocs)

	assertFileNames(t, got, []string{readmeFileName, taskfileName, runFileName})
}

func assertFileNames(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}

	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("files = %#v, want %#v", got, want)
		}
	}
}

func fileNames(files []File) []string {
	names := make([]string, 0, len(files))

	for _, file := range files {
		names = append(names, file.RelativePath)
	}

	return names
}
