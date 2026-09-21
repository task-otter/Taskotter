// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"os"
	"path/filepath"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

func createBenchmarkStore(b *testing.B) *storedomain.Snapshot {
	b.Helper()
	root := filepath.Join(
		consts.PathParent, consts.PathParent, consts.PathParent, consts.PathParent,
		dirTests, dirFixtures, dirStore,
	)

	snap, err := storesvc.LocalSnapshot(root, testStoreRefInfo())
	if err != nil {
		b.Fatal(err)
	}
	return snap
}

func prepareSyncInputBench(b *testing.B, cfg *config.Config, snap *storedomain.Snapshot) syncdomain.SyncInput {
	b.Helper()
	resolutions, err := resolvesvc.ResolveAll(&resolvesvc.ResolveAllInput{
		Tasks:          cfg.Tasks,
		Catalog:        snap.Catalog,
		PackageManager: cfg.NodePackageManager,
	})
	if err != nil {
		b.Fatal(err)
	}

	si, err := syncsvc.PrepareSyncInput(&syncsvc.PrepareSyncInputArgs{
		Cfg:         cfg,
		Snapshot:    syncsvc.SnapshotPort(snap),
		TaskfileOps: synctaskfile.NewOps(),
		Resolutions: resolutions,
		DepSources:  []string{"pnpm"},
	})
	if err != nil {
		b.Fatal(err)
	}
	return si
}

func BenchmarkBuildPlan(b *testing.B) {
	ws := b.TempDir()
	writeRootTaskfileBench(b, ws)
	snap := createBenchmarkStore(b)
	cfg := testConfig(ws, mutateEslintGoPnpm)

	si := prepareSyncInputBench(b, cfg, snap)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := syncsvc.BuildPlan(&si)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlan(b *testing.B) {
	BenchmarkBuildPlan(b)
}

func BenchmarkDiff(b *testing.B) {
	ws := b.TempDir()
	writeRootTaskfileBench(b, ws)
	snap := createBenchmarkStore(b)
	cfg := testConfig(ws, mutateEslintGoPnpm)

	si := prepareSyncInputBench(b, cfg, snap)

	// Pre-build plan and simulate workspace state
	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := syncsvc.BuildPlan(&si)
		if err != nil {
			b.Fatal(err)
		}
	}
	_ = plan
}

func BenchmarkUpdateRootTaskfile(b *testing.B) {
	template := synctaskfile.NewRootTemplate()
	input := &rootupd.RootUpdateInput{
		TargetFolder: "taskfiles",
		Tasks:        []string{"go"},
		ManagedTasks: []string{"go"},
		DestByTask:   map[string]string{"go": "go"},
		ModuleTaskfiles: map[string][]byte{
			"go": []byte("version: \"3\"\nvars:\n  GO_VER: 1.26.5\n"),
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := synctaskfile.UpdateRootTaskfile(template, input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRewriteIncludes(b *testing.B) {
	input := []byte("version: \"3\"\nincludes:\n  pnpm:\n    taskfile: \"../../../pnpm/Taskfile.yml\"\n")
	mapping := map[string]string{
		"../../../pnpm/Taskfile.yml": "../pnpm/Taskfile.yml",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := synctaskfile.RewriteIncludes(input, mapping, "eslint/node/pnpm")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func writeRootTaskfileBench(b *testing.B, workspace string) {
	b.Helper()
	content := []byte("version: \"3\"\nincludes: {}\ntasks:\n  hello:\n    cmds:\n      - echo hello\n")
	writeFileWithDirBench(b, filepath.Join(workspace, testTaskfileName), content)
}

func writeFileWithDirBench(b *testing.B, path string, content []byte) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), consts.FilePerm755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, content, consts.FilePerm644); err != nil {
		b.Fatal(err)
	}
}

func mustResolveTaskBench(b *testing.B, cfg *config.Config, snap *storedomain.Snapshot) resolvesvc.Resolution {
	b.Helper()
	res, err := resolvesvc.Resolve(&resolvesvc.ResolveInput{
		Task:           testModuleEslint,
		Catalog:        snap.Catalog,
		PackageManager: cfg.NodePackageManager,
	})
	if err != nil {
		b.Fatal(err)
	}
	return res
}
