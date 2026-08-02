// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/dependency"
	"github.com/task-otter/Taskotter/internal/normalizer"
	"github.com/task-otter/Taskotter/internal/resolver"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
)

func fixtureStore(t *testing.T) *store.Snapshot {
	t.Helper()

	root := filepath.Join("..", "..", "tests", "fixtures", "store")

	snap, err := store.LocalSnapshot(root, store.RefInfo{
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

func writeRootTaskfile(t *testing.T, workspace string) {
	t.Helper()

	content := []byte(`version: "3"
includes: {}
tasks:
  hello:
    cmds:
      - echo hello
`)

	err := os.WriteFile(filepath.Join(workspace, testTaskfileName), content, 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

func dependencySources(t *testing.T, sources []string, snap *store.Snapshot) ([]string, error) {
	t.Helper()

	return dependency.ResolveTransitive(sources, snap.Deps)
}

func TestBuildPlanInitialSync(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{testModuleEslint, "go"}
		cfg.NodePackageManager = config.PMPnpm
		cfg.NodeVersionManager = config.VMFnm
		cfg.IncludesDoc = true
	})

	_, plan := preparePlan(t, workspace, cfg)

	if !plan.Changed {
		t.Fatal("expected changes on initial sync")
	}

	rootText := string(plan.RootTaskfile)

	for _, want := range []string{
		"task: go:lint",
		"task: eslint:lint",
		"task: go:lint:fix",
		"task: eslint:lint:fix",
	} {
		if !strings.Contains(rootText, want) {
			t.Fatalf("expected %q in root Taskfile:\n%s", want, rootText)
		}
	}

	if strings.Contains(rootText, "task: pnpm:lint") {
		t.Fatalf("dependency-only module should not contribute root tasks:\n%s", rootText)
	}

	wantGenerated := []string{"install", "lint", "lint:fix", "version"}

	if !slices.Equal(plan.Lock.GeneratedRootTasks, wantGenerated) {
		t.Fatalf(
			"generated root tasks = %#v, want %#v",
			plan.Lock.GeneratedRootTasks,
			wantGenerated,
		)
	}
}

func TestBuildPlanCreatesRootTaskfile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	snap := fixtureStore(t)

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
	})

	resolutions, err := resolver.ResolveAll(
		cfg.Tasks,
		snap.Catalog,
		cfg.NodePackageManager,
		cfg.NodeVersionManager,
	)
	if err != nil {
		t.Fatal(err)
	}

	sources := make([]string, 0, len(resolutions))

	for _, res := range resolutions {
		sources = append(sources, res.SourceModule)
	}

	depSources, err := dependency.ResolveTransitive(sources, snap.Deps)
	if err != nil {
		t.Fatal(err)
	}

	syncInput, err := app.PrepareSyncInput(cfg, snap, resolutions, depSources)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := syncer.BuildPlan(syncInput)
	if err != nil {
		t.Fatal(err)
	}

	if !plan.Changed {
		t.Fatal("expected changes on initial sync")
	}

	if !containsRootTaskfile(plan.Added) {
		t.Fatalf("expected root Taskfile.yml in added files, got added=%v", plan.Added)
	}
}

func containsRootTaskfile(list []string) bool {
	return slices.Contains(list, testTaskfileName)
}

// writeUnmanagedFile drops a file TaskOtter never wrote into the eslint
// destination module, so planning must refuse to take it over.
func writeUnmanagedFile(t *testing.T, workspace string) {
	t.Helper()

	err := os.MkdirAll(
		filepath.Join(workspace, config.DefaultTargetFolder, testModuleEslint),
		0o755,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(workspace, config.DefaultTargetFolder, testModuleEslint, "user.txt"),
		[]byte("keep"),
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnmanagedDestinationConflict(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	writeUnmanagedFile(t, workspace)

	snap := fixtureStore(t)
	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{testModuleEslint}
		cfg.NodePackageManager = config.PMPnpm
		cfg.NodeVersionManager = config.VMFnm
		cfg.IncludesDoc = true
	})

	res, err := resolver.Resolve(
		testModuleEslint,
		snap.Catalog,
		cfg.NodePackageManager,
		cfg.NodeVersionManager,
	)
	if err != nil {
		t.Fatal(err)
	}

	sourceToDest, err := normalizer.BuildDestinationMap([]string{res.SourceModule})
	if err != nil {
		t.Fatal(err)
	}

	eslintPath := config.DefaultTargetFolder + "/" + testModuleEslint

	_, err = syncer.BuildPlan(syncer.SyncInput{
		Config:   cfg,
		Snapshot: snap,
		Requested: map[string]syncer.ModuleRecord{
			testModuleEslint: {
				SourceModule:      res.SourceModule,
				DestinationModule: testModuleEslint,
				Path:              eslintPath,
			},
		},
		Dependencies: nil,
		SourceToDest: sourceToDest,
		DestByTask:   map[string]string{testModuleEslint: testModuleEslint},
	})
	if err == nil {
		t.Fatal("expected unmanaged destination conflict")
	}
}

func TestIncludesDocFalseSkipsReadme(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	snap := fixtureStore(t)
	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{testModuleEslint}
		cfg.NodePackageManager = config.PMPnpm
		cfg.NodeVersionManager = config.VMFnm
		cfg.IncludesDoc = false
	})

	res, err := resolver.Resolve(
		testModuleEslint,
		snap.Catalog,
		cfg.NodePackageManager,
		cfg.NodeVersionManager,
	)
	if err != nil {
		t.Fatal(err)
	}

	sourceToDest := map[string]string{res.SourceModule: testModuleEslint}

	eslintPath := config.DefaultTargetFolder + "/" + testModuleEslint

	plan, err := syncer.BuildPlan(syncer.SyncInput{
		Config:   cfg,
		Snapshot: snap,
		Requested: map[string]syncer.ModuleRecord{
			testModuleEslint: {
				SourceModule:      res.SourceModule,
				DestinationModule: testModuleEslint,
				Path:              eslintPath,
			},
		},
		Dependencies: nil,
		SourceToDest: sourceToDest,
		DestByTask:   map[string]string{testModuleEslint: testModuleEslint},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, managed := range plan.ManagedFiles {
		if filepath.Base(managed.Path) == testReadmeName {
			t.Fatal("README should be excluded when includes-doc=false")
		}
	}
}
