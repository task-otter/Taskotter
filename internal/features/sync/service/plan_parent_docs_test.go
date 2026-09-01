// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"path/filepath"
	"testing"

	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

const (
	parentDocTool       = "tool"
	parentDocNode       = "tool/node"
	parentDocLeaf       = "tool/node/pnpm"
	parentDocRootReadme = "root-readme\n"
	parentDocLeafReadme = "leaf-readme\n"
)

func writeNestedDocsStore(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFileWithDir(t, filepath.Join(root, ".deps.yml"), []byte(parentDocLeaf+": []\n"))

	writeNestedDocsToolDir(t, root)

	// Every directory on the way to a variant leaf is itself a module, so the
	// intermediate node directory needs its own Taskfile.yml to be walked.
	writeNestedDocsTaskfile(t, root, parentDocNode)
	writeNestedDocsTaskfile(t, root, parentDocLeaf)

	return root
}

func writeNestedDocsTaskfile(t *testing.T, root, module string) {
	t.Helper()

	writeModuleFile(&moduleFileInput{
		t:       t,
		dir:     filepath.Join(root, config.DefaultTargetFolder, module),
		rel:     testTaskfileName,
		content: testEmptyTaskfileYAML,
	})
}

func writeNestedDocsToolDir(t *testing.T, root string) {
	t.Helper()

	writeNestedDocsTaskfile(t, root, parentDocTool)
	writeModuleFile(&moduleFileInput{
		t:       t,
		dir:     filepath.Join(root, config.DefaultTargetFolder, parentDocTool),
		rel:     testReadmeName,
		content: parentDocRootReadme,
	})
}

func writeNestedDocsStoreWithLeafReadme(t *testing.T) string {
	t.Helper()

	root := writeNestedDocsStore(t)
	leafDir := filepath.Join(root, config.DefaultTargetFolder, parentDocLeaf)
	writeModuleFile(&moduleFileInput{
		t: t, dir: leafDir, rel: testReadmeName, content: parentDocLeafReadme,
	})

	return root
}

func nestedDocsSnapshot(t *testing.T, storeRoot string) *storedomain.Snapshot {
	t.Helper()

	snap, err := storesvc.LocalSnapshot(storeRoot, testStoreRefInfo())
	if err != nil {
		t.Fatal(err)
	}

	return snap
}

func nestedDocsSyncInput(
	cfg *config.Config,
	snap *storedomain.Snapshot,
) syncdomain.SyncInput {
	return variantModuleSyncInput(&variantModuleSyncArgs{
		cfg: cfg, snap: snap, task: parentDocTool, source: parentDocLeaf,
	})
}

func mutateNestedDocs(includesDoc bool) func(*config.Config) {
	return func(cfg *config.Config) {
		cfg.Tasks = []string{parentDocTool}
		cfg.IncludesDoc = includesDoc
	}
}

func findManagedFile(plan *syncdomain.Plan, path string) *managed.File {
	for i := range plan.ManagedFiles {
		if plan.ManagedFiles[i].Path == path {
			return &plan.ManagedFiles[i]
		}
	}

	return nil
}

func assertParentReadmePath(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	wantPath := pathutil.JoinRelative(config.DefaultTargetFolder, parentDocTool, testReadmeName)
	wantSource := pathutil.JoinRelative(config.DefaultTargetFolder, parentDocTool, testReadmeName)

	file := findManagedFile(plan, wantPath)

	if file == nil {
		t.Fatalf(errFmtExpectedManagedPath, wantPath)
	}

	if file.SourcePath != wantSource {
		t.Fatalf("SourcePath = %q, want %q", file.SourcePath, wantSource)
	}
}

func assertParentReadmeContent(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	entry, ok := plan.ModuleContents[parentDocLeaf][testReadmeName]

	if !ok {
		t.Fatal("expected README in module contents")
	}

	if string(entry.Data) != parentDocRootReadme {
		t.Fatalf("README content = %q, want %q", entry.Data, parentDocRootReadme)
	}
}

func assertParentReadmeCollected(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	assertParentReadmePath(t, plan)
	assertParentReadmeContent(t, plan)
}

func finishNestedDocsPlan(
	t *testing.T,
	mutate func(*config.Config),
	storeRoot string,
) *syncdomain.Plan {
	t.Helper()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	snap := nestedDocsSnapshot(t, storeRoot)
	cfg := testConfig(workspace, mutate)
	si := nestedDocsSyncInput(cfg, snap)

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	return plan
}

func buildNestedDocsPlan(t *testing.T, includesDoc bool) *syncdomain.Plan {
	t.Helper()

	return finishNestedDocsPlan(t, mutateNestedDocs(includesDoc), writeNestedDocsStore(t))
}

func buildNestedDocsPlanWithLeafReadme(t *testing.T) *syncdomain.Plan {
	t.Helper()

	return finishNestedDocsPlan(t, mutateNestedDocs(true), writeNestedDocsStoreWithLeafReadme(t))
}

// TestLogicalRootDocsMergedFromParent verifies parent README is collected when includes-doc is
// true and skipped when false, with logical-root docs winning over a leaf README.
func TestLogicalRootDocsMergedFromParent(t *testing.T) {
	t.Parallel()

	withDocs := buildNestedDocsPlan(t, true)
	assertParentReadmeCollected(t, withDocs)

	withoutDocs := buildNestedDocsPlan(t, false)
	assertNoReadmeManaged(t, withoutDocs)

	collision := buildNestedDocsPlanWithLeafReadme(t)
	assertParentReadmeCollected(t, collision)
}
