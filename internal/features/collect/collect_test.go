// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultCollectorAppliesFilePolicy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for name, contents := range map[string]string{
		"Taskfile.yml": "version: '3'\n",
		"run.go":       "package run\n",
		"run_test.go":  "package run\n",
		"README.md":    "docs\n",
	} {
		path := filepath.Join(dir, name)

		err := os.WriteFile(path, []byte(contents), 0o755)
		if err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	withoutDocs, err := (DefaultCollector{}).Collect(Options{SourceDir: dir})
	if err != nil {
		t.Fatalf("Collect() without docs error = %v", err)
	}

	if got := fileNames(
		withoutDocs,
	); len(got) != 2 || got[0] != "Taskfile.yml" ||
		got[1] != "run.go" {

		t.Fatalf("without docs files = %#v", got)
	}

	withDocs, err := (DefaultCollector{}).Collect(Options{SourceDir: dir, IncludeDocs: true})
	if err != nil {
		t.Fatalf("Collect() with docs error = %v", err)
	}

	got := fileNames(withDocs)

	if len(got) != 3 || got[0] != "README.md" || got[1] != "Taskfile.yml" || got[2] != "run.go" {
		t.Fatalf("with docs files = %#v", got)
	}
}

func fileNames(files []File) []string {
	names := make([]string, 0, len(files))

	for _, file := range files {
		names = append(names, file.RelativePath)
	}

	return names
}
