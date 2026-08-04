// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"path/filepath"
	"testing"

	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

const (
	parentDocTool       = "tool"
	parentDocLeaf       = "tool/node/fnm/pnpm"
	parentDocRootReadme = "root-readme\n"
	parentDocLeafReadme = "leaf-readme\n"
)

func writeNestedDocsStore(t *testing.T, includeLeafReadme bool) string {
	t.Helper()

	root := t.TempDir()
	writeFileWithDir(t, filepath.Join(root, ".deps.yml"), []byte(parentDocLeaf+": []\n"))

	toolDir := filepath.Join(root, config.DefaultTargetFolder, parentDocTool)
	writeModuleFile(&moduleFileInput{
		t: t, dir: toolDir, rel: testTaskfileName, content: "version: \"3\"\ntasks: {}\n",
	})
	writeModuleFile(&moduleFileInput{
		t: t, dir: toolDir, rel: testReadmeName, content: parentDocRootReadme,
	})

	leafDir := filepath.Join(root, config.DefaultTargetFolder, parentDocLeaf)
	writeModuleFile(&moduleFileInput{
		t: t, dir: leafDir, rel: testTaskfileName, content: "version: \"3\"\ntasks: {}\n",
	})

	if includeLeafReadme {
		writeModuleFile(&moduleFileInput{
			t: t, dir: leafDir, rel: testReadmeName, content: parentDocLeafReadme,
		})
	}

	return root
}

func nestedDocsSnapshot(t *testing.T, storeRoot string) *storedomain.Snapshot {
	t.Helper()

	snap, err := storesvc.LocalSnapshot(storeRoot, &storedomain.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: "",
		SourceRef:        "refs/heads/main",
		ResolvedCommit:   "abc123",
		DefaultBranch:    "main",
	})
	if err != nil {
		t.Fatal(err)
	}

	return snap
}

func nestedDocsSyncInput(
	cfg *config.Config,
	snap *storedomain.Snapshot,
) syncdomain.SyncInput {
	return syncdomain.SyncInput{
		Config:      cfg,
		TaskfileOps: synctaskfile.Ops{},
		Snapshot:    snap,
		Requested: map[string]lockmodel.ModuleRecord{
			parentDocTool: {
				SourceModule:      parentDocLeaf,
				DestinationModule: parentDocTool,
				Path:              config.DefaultTargetFolder + "/" + parentDocTool,
			},
		},
		Dependencies: nil,
		SourceToDest: map[string]string{parentDocLeaf: parentDocTool},
		DestByTask:   map[string]string{parentDocTool: parentDocTool},
	}
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

func assertParentReadmeCollected(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	wantPath := pathutil.JoinRelative(config.DefaultTargetFolder, parentDocTool, testReadmeName)
	wantSource := pathutil.JoinRelative(config.DefaultTargetFolder, parentDocTool, testReadmeName)

	file := findManagedFile(plan, wantPath)
	if file == nil {
		t.Fatalf("expected managed path %q", wantPath)
	}

	if file.SourcePath != wantSource {
		t.Fatalf("SourcePath = %q, want %q", file.SourcePath, wantSource)
	}

	entry, ok := plan.ModuleContents[parentDocLeaf][testReadmeName]
	if !ok {
		t.Fatal("expected README in module contents")
	}

	if string(entry.Data) != parentDocRootReadme {
		t.Fatalf("README content = %q, want %q", entry.Data, parentDocRootReadme)
	}
}

func buildNestedDocsPlan(t *testing.T, includesDoc, includeLeafReadme bool) *syncdomain.Plan {
	t.Helper()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	snap := nestedDocsSnapshot(t, writeNestedDocsStore(t, includeLeafReadme))
	cfg := testConfig(workspace, mutateNestedDocs(includesDoc))
	si := nestedDocsSyncInput(cfg, snap)

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	return plan
}

// TestLogicalRootDocsMergedFromParent verifies parent README is collected when includes-doc is
// true and skipped when false, with logical-root docs winning over a leaf README.
func TestLogicalRootDocsMergedFromParent(t *testing.T) {
	t.Parallel()

	withDocs := buildNestedDocsPlan(t, true, false)
	assertParentReadmeCollected(t, withDocs)

	withoutDocs := buildNestedDocsPlan(t, false, false)
	assertNoReadmeManaged(t, withoutDocs)

	collision := buildNestedDocsPlan(t, true, true)
	assertParentReadmeCollected(t, collision)
}
