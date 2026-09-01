// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"path/filepath"
	"strings"
	"testing"

	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	metadataFileName = "metadata.yml"

	currentSchemaMetadata = "---\nschema: taskotter.dev/taskfile-metadata/v1\n" +
		"module: tool/node/pnpm\ntaskfile: Taskfile.yml\nexported_tasks: [install]\n"

	legacySchemaMetadata = "---\nschema: taskotter.store/v1\n" +
		"module: tool/node/pnpm\ntaskfile: Taskfile.yml\nexported_tasks: [install]\n"
)

func buildPlanWithLeafMetadata(t *testing.T, metadata string) error {
	t.Helper()

	storeRoot := writeNestedDocsStore(t)
	leafDir := filepath.Join(storeRoot, config.DefaultTargetFolder, parentDocLeaf)
	writeModuleFile(&moduleFileInput{
		t: t, dir: leafDir, rel: metadataFileName, content: metadata,
	})

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	snap := nestedDocsSnapshot(t, storeRoot)
	cfg := testConfig(workspace, mutateNestedDocs(true))
	si := nestedDocsSyncInput(cfg, snap)

	plan, err := syncsvc.BuildPlan(&si)
	iox.Discard(plan)

	return err
}

// TestStoreMetadataAcceptsCurrentSchema verifies the pinned metadata.yml schema is accepted.
func TestStoreMetadataAcceptsCurrentSchema(t *testing.T) {
	t.Parallel()

	err := buildPlanWithLeafMetadata(t, currentSchemaMetadata)
	if err != nil {
		t.Fatalf("BuildPlan with current schema: %v", err)
	}
}

// TestStoreMetadataRejectsUnknownSchema verifies an unrecognized metadata.yml schema
// fails the sync rather than being silently ignored.
func TestStoreMetadataRejectsUnknownSchema(t *testing.T) {
	t.Parallel()

	err := buildPlanWithLeafMetadata(t, legacySchemaMetadata)
	if err == nil {
		t.Fatal("expected unsupported metadata schema error")
	}

	if !strings.Contains(err.Error(), "unsupported metadata.yml schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}
