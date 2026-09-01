// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
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
		t: t, dir: leafDir, rel: testMetadataFileName, content: metadata,
	})

	err := buildNestedDocsMetadataPlan(t, storeRoot)
	if err != nil {
		return fmt.Errorf("leaf metadata plan: %w", err)
	}

	return nil
}

func buildNestedDocsMetadataPlan(t *testing.T, storeRoot string) error {
	t.Helper()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	si := nestedDocsSyncInput(
		testConfig(workspace, mutateNestedDocs(true)),
		nestedDocsSnapshot(t, storeRoot),
	)

	plan, err := syncsvc.BuildPlan(&si)
	iox.Discard(plan)

	if err != nil {
		return fmt.Errorf("build plan: %w", err)
	}

	return nil
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
