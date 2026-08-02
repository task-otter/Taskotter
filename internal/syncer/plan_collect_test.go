// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/syncer"
)

func TestCollectModuleFilesSkipsTestsAndDocs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()

		path := filepath.Join(dir, rel)

		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	write(testTaskfileName, "version: \"3\"\n")
	write(testReadmeName, "docs\n")
	write("docs/guide.md", "guide\n")
	write("go_test.go", "package go_test\n")
	write("metadata.yml", "module: go\n")
	write("docs/metadata.yml", "module: go\n")

	withDocs, err := syncer.CollectModuleFiles(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The module's own metadata.yml describes it to the store; only a
	// same-named file deeper in the module is ordinary content.
	assertCollected(t, withDocs, map[string]bool{
		"Taskfile.yml":      true,
		"README.md":         true,
		"docs/guide.md":     true,
		"docs/metadata.yml": true,
		"go_test.go":        false,
		"metadata.yml":      false,
	})

	withoutDocs, err := syncer.CollectModuleFiles(dir, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	assertCollected(t, withoutDocs, map[string]bool{
		"Taskfile.yml":      true,
		"README.md":         false,
		"docs/guide.md":     false,
		"docs/metadata.yml": false,
	})
}

func assertCollected(t *testing.T, contents map[string]syncer.FileEntry, want map[string]bool) {
	t.Helper()

	for path, wantSynced := range want {
		_, ok := contents[path]

		if ok != wantSynced {
			t.Fatalf("collected %q = %t, want %t", path, ok, wantSynced)
		}
	}
}
