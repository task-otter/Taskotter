// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:exhaustruct_v5 // test fixtures only set fields exercised by the unit
package service

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	stubSnapshot struct {
		root string
	}
)

// TestAppendUniqueExportedTaskSkipsEmptyAndDupes verifies empty/dup task filtering.
func TestAppendUniqueExportedTaskSkipsEmptyAndDupes(t *testing.T) {
	t.Parallel()

	seen := map[string]struct{}{}

	out := appendUniqueExportedTask(nil, seen, consts.Empty)

	out = appendUniqueExportedTask(out, seen, taskCI)
	out = appendUniqueExportedTask(out, seen, taskCI)

	if len(out) != consts.IndexOne || out[consts.IndexZero] != taskCI {
		t.Fatalf("out = %v", out)
	}
}

// TestGeneratedTaskMetadataMissingRecord verifies absent requested records.
func TestGeneratedTaskMetadataMissingRecord(t *testing.T) {
	t.Parallel()

	meta, ok := generatedTaskMetadata(&groupModulesInput{
		requestedRecords: map[string]moduleRecord{},
		metadata:         storeTaskMetaMap{},
	}, consts.Go)
	iox.Discard(meta)

	if ok {
		t.Fatal("expected missing record")
	}
}

// TestGeneratedTaskMetadataResolvesRecord verifies known records resolve metadata.
func TestGeneratedTaskMetadataResolvesRecord(t *testing.T) {
	t.Parallel()

	var want storeTaskMetadata

	want.Schema = storeMetadataSchema
	want.Module = consts.Go
	want.ExportedTasks = []string{taskCI}

	meta, ok := generatedTaskMetadata(goTaskMetadataInput(&want), consts.Go)

	if !ok || meta.Module != consts.Go {
		t.Fatalf("ok=%t meta=%+v", ok, meta)
	}
}

// TestLoadStoreTaskMetadataReportsWalkFailure verifies load wraps walk failures.
//
//nolint:paralleltest // swaps the package-level walkDir seam
func TestLoadStoreTaskMetadataReportsWalkFailure(t *testing.T) {
	swapWalkDir(t, failingWalk)

	meta, err := loadStoreTaskMetadata(stubSnapshot{root: t.TempDir()})
	iox.Discard(meta)
	assertFails(t, err)
}

// TestModuleNameForReportsRelFailure verifies Rel failures surface.
//
//nolint:paralleltest // swaps the package-level relPath seam
func TestModuleNameForReportsRelFailure(t *testing.T) {
	swapRelPath(t, failingRelPath)

	name, err := moduleNameFor(rootDir, rootMetaPath)
	iox.Discard(name)
	assertFails(t, err)
}

// TestParseStoreTaskMetadataRejectsBadSchema verifies unsupported schemas fail.
func TestParseStoreTaskMetadataRejectsBadSchema(t *testing.T) {
	t.Parallel()

	meta, err := parseStoreTaskMetadata([]byte("schema: other\n"), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestParseStoreTaskMetadataRejectsCorruptYAML verifies corrupt metadata fails.
func TestParseStoreTaskMetadataRejectsCorruptYAML(t *testing.T) {
	t.Parallel()

	meta, err := parseStoreTaskMetadata([]byte(badYAMLText), consts.Go)
	iox.Discard(meta)
	assertFails(t, err)
}

// TestStoreMetadataWalkerReportsWalkError verifies walk errors are wrapped.
func TestStoreMetadataWalkerReportsWalkError(t *testing.T) {
	t.Parallel()

	walker := storeMetadataWalker(t.TempDir(), map[string]storeTaskMetadata{})
	assertFails(t, walker(stagingName, nil, errStub))
}

// TestUnmarshalYAMLReportsFailure verifies non-mapping nodes fail.
func TestUnmarshalYAMLReportsFailure(t *testing.T) {
	t.Parallel()

	var meta storeTaskMetadata

	assertFails(t, meta.UnmarshalYAML(scalarYAMLNode("plain")))
}

func (stubSnapshot) DefaultBranch() string { return consts.Empty }

func (snap stubSnapshot) ModuleDir(string) string { return snap.root }

func (stubSnapshot) ResolvedCommit() string { return consts.Empty }

func (stubSnapshot) SourceRef() string { return consts.Empty }

func (snap stubSnapshot) WorkspaceRoot() string { return snap.root }
