// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"os"
	"path/filepath"
	"strconv"
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
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	// benchFileSpec describes one file written into the synthetic store or workspace.
	benchFileSpec struct {
		dir  string
		name string
		body string
	}
)

const (
	benchStoreModules = 16
	benchModulePrefix = "mod"
	benchDirPerm      = 0o750
	benchGuideFile    = "guide.md"
	benchDepsFileName = ".deps.yml"
	benchGuideBody    = "# guide\n"
	benchDepsBody     = "---\n"
	benchReadmePrefix = "# "
	benchRootBody     = "version: \"3\"\nincludes: {}\ntasks:\n  hello:\n    cmds:\n      - echo hello\n"
	benchTaskfileBody = "version: \"3\"\nvars:\n  TOOL_VERSION: \"1.0.0\"\ntasks:\n" +
		"  install:\n    cmds:\n      - echo install\n" +
		"  lint:\n    cmds:\n      - echo lint\n" +
		"  version:\n    cmds:\n      - echo version\n"
	benchMetadataHead = "---\nschema: taskotter.dev/taskfile-metadata/v1\nmodule: "
	benchMetadataTail = "\ntaskfile: Taskfile.yml\nexported_tasks: [install, lint, version]\n" +
		"variants: []\n"
)

func benchModuleName(idx int) string {
	return benchModulePrefix + strconv.Itoa(idx)
}

func benchWriteFile(b *testing.B, spec *benchFileSpec) {
	b.Helper()

	err := os.MkdirAll(spec.dir, benchDirPerm)
	if err != nil {
		b.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(spec.dir, spec.name), []byte(spec.body), consts.FilePerm644)
	if err != nil {
		b.Fatal(err)
	}
}

func benchWriteModule(b *testing.B, dir, name string) {
	b.Helper()

	metadata := benchMetadataHead + name + benchMetadataTail
	docs := filepath.Join(dir, docsDirName)
	readme := &benchFileSpec{dir: dir, name: testReadmeName, body: benchReadmePrefix + name}

	benchWriteFile(b, &benchFileSpec{dir: dir, name: testTaskfileName, body: benchTaskfileBody})
	benchWriteFile(b, &benchFileSpec{dir: dir, name: testMetadataFileName, body: metadata})
	benchWriteFile(b, readme)
	benchWriteFile(b, &benchFileSpec{dir: docs, name: benchGuideFile, body: benchGuideBody})
}

// benchStoreRoot writes a synthetic store tree usable by the sync planner.
func benchStoreRoot(b *testing.B) string {
	b.Helper()

	root := b.TempDir()

	for idx := range benchStoreModules {
		name := benchModuleName(idx)

		benchWriteModule(b, filepath.Join(root, config.DefaultTargetFolder, name), name)
	}

	benchWriteFile(b, &benchFileSpec{dir: root, name: benchDepsFileName, body: benchDepsBody})

	return root
}

func benchSnapshot(b *testing.B) *storedomain.Snapshot {
	b.Helper()

	snap, err := storesvc.LocalSnapshot(benchStoreRoot(b), testStoreRefInfo())
	if err != nil {
		b.Fatal(err)
	}

	return snap
}

func benchPlanTasks() []string {
	tasks := make([]string, consts.IndexZero, benchStoreModules)

	for idx := range benchStoreModules {
		tasks = append(tasks, benchModuleName(idx))
	}

	return tasks
}

func benchPlanConfig(b *testing.B) *config.Config {
	b.Helper()

	workspace := b.TempDir()
	root := &benchFileSpec{dir: workspace, name: testTaskfileName, body: benchRootBody}

	benchWriteFile(b, root)

	cfg := testConfig(workspace, nil)

	cfg.Tasks = benchPlanTasks()
	cfg.IncludesDoc = true

	return cfg
}

func benchResolutions(
	b *testing.B,
	cfg *config.Config,
	snap *storedomain.Snapshot,
) []resolvesvc.Resolution {
	b.Helper()

	resolutions, err := resolvesvc.ResolveAll(&resolvesvc.ResolveAllInput{
		Catalog:        snap.Catalog,
		PackageManager: cfg.NodePackageManager,
		Tasks:          cfg.Tasks,
	})
	if err != nil {
		b.Fatal(err)
	}

	return resolutions
}

func benchSyncInput(b *testing.B) syncdomain.SyncInput {
	b.Helper()

	snap := benchSnapshot(b)
	cfg := benchPlanConfig(b)

	input, err := syncsvc.PrepareSyncInput(&syncsvc.PrepareSyncInputArgs{
		Cfg:         cfg,
		Snapshot:    syncsvc.SnapshotPort(snap),
		TaskfileOps: synctaskfile.NewOps(),
		Resolutions: benchResolutions(b, cfg, snap),
		DepSources:  nil,
	})
	if err != nil {
		b.Fatal(err)
	}

	return input
}

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

// BenchmarkBuildPlanSynthetic measures an end-to-end sync plan build over a synthetic store.
func BenchmarkBuildPlanSynthetic(b *testing.B) {
	input := benchSyncInput(b)

	for b.Loop() {
		plan, err := syncsvc.BuildPlan(&input)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(plan)
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
