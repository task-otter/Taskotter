// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/syncer"
)

func writeModuleFile(t *testing.T, dir, rel, content string) {
	t.Helper()

	writeFileWithDir(t, filepath.Join(dir, rel), []byte(content), consts.FilePerm644)
}

func writeModuleFixtureFiles(t *testing.T, dir string) {
	t.Helper()

	writeModuleFile(t, dir, testTaskfileName, "version: \"3\"\n")
	writeModuleFile(t, dir, testReadmeName, "docs\n")
	writeModuleFile(t, dir, docGuideMD, "guide\n")
	writeModuleFile(t, dir, fileGoTestGo, "package go_test\n")
	writeModuleFile(t, dir, testMetadataFileName, "module: go\n")
	writeModuleFile(t, dir, docMetadataYML, "module: go\n")
}

func assertCollectedWithoutDocs(t *testing.T, dir string) {
	t.Helper()

	withoutDocs, err := syncer.CollectModuleFiles(dir, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	assertCollected(t, withoutDocs, map[string]bool{
		testTaskfileName: true,
		testReadmeName:   false,
		docGuideMD:       false,
		docMetadataYML:   false,
	})
}

// TestCollectModuleFilesSkipsTestsAndDocs verifies test and doc files are excluded unless requested.
func TestCollectModuleFilesSkipsTestsAndDocs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeModuleFixtureFiles(t, dir)

	withDocs, err := syncer.CollectModuleFiles(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The module's own metadata.yml describes it to the store; only a
	// same-named file deeper in the module is ordinary content.
	assertCollected(t, withDocs, map[string]bool{
		testTaskfileName:     true,
		testReadmeName:       true,
		docGuideMD:           true,
		docMetadataYML:       true,
		fileGoTestGo:         false,
		testMetadataFileName: false,
	})

	assertCollectedWithoutDocs(t, dir)
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
