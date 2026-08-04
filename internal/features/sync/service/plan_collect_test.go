// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"testing"

	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
)

func writeModuleFixtureFiles(t *testing.T, dir string) {
	t.Helper()

	writeModuleFile(
		&moduleFileInput{t: t, dir: dir, rel: testTaskfileName, content: "version: \"3\"\n"},
	)
	writeModuleFile(&moduleFileInput{t: t, dir: dir, rel: testReadmeName, content: "docs\n"})
	writeModuleFile(&moduleFileInput{t: t, dir: dir, rel: docGuideMD, content: "guide\n"})
	writeModuleFile(
		&moduleFileInput{t: t, dir: dir, rel: fileGoTestGo, content: "package go_test\n"},
	)
	writeModuleFile(
		&moduleFileInput{t: t, dir: dir, rel: testMetadataFileName, content: testModuleGoMetadata},
	)
	writeModuleFile(
		&moduleFileInput{t: t, dir: dir, rel: docMetadataYML, content: testModuleGoMetadata},
	)
}

func collectOptions(dir string, policy syncsvc.DocPolicy) *syncsvc.CollectOptions {
	return &syncsvc.CollectOptions{
		TaskfileOps:  nil,
		SourceDir:    dir,
		DocPolicy:    policy,
		SourceToDest: nil,
	}
}

func assertCollectedWithoutDocs(t *testing.T, dir string) {
	t.Helper()

	withoutDocs, err := syncsvc.CollectModuleFiles(collectOptions(dir, syncsvc.DocPolicySkip))
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

func assertCollectedWithDocs(t *testing.T, dir string) {
	t.Helper()

	withDocs, err := syncsvc.CollectModuleFiles(collectOptions(dir, syncsvc.DocPolicyInclude))
	if err != nil {
		t.Fatal(err)
	}

	assertCollected(t, withDocs, map[string]bool{
		testTaskfileName:     true,
		testReadmeName:       true,
		docGuideMD:           true,
		docMetadataYML:       true,
		fileGoTestGo:         false,
		testMetadataFileName: false,
	})
}

// TestCollectModuleFilesSkipsTestsAndDocs verifies test and doc files are excluded unless requested.
func TestCollectModuleFilesSkipsTestsAndDocs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeModuleFixtureFiles(t, dir)
	assertCollectedWithDocs(t, dir)
	assertCollectedWithoutDocs(t, dir)
}

func assertCollected(t *testing.T, contents map[string]syncdomain.FileEntry, want map[string]bool) {
	t.Helper()

	for path := range want {
		_, ok := contents[path]

		if ok != want[path] {
			t.Fatalf("collected %q = %t, want %t", path, ok, want[path])
		}
	}
}
