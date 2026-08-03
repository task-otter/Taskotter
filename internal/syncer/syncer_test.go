// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/dependency"
	"github.com/task-otter/Taskotter/internal/resolver"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
)

func fixtureStore(t *testing.T) *store.Snapshot {
	t.Helper()

	root := filepath.Join(consts.PathParent, consts.PathParent, dirTests, dirFixtures, dirStore)

	snap, err := store.LocalSnapshot(root, &store.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: consts.Empty,
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

	err := os.WriteFile(filepath.Join(workspace, testTaskfileName), content, consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}
}

func dependencySources(t *testing.T, sources []string, snap *store.Snapshot) ([]string, error) {
	t.Helper()

	deps, err := dependency.ResolveTransitive(sources, snap.Deps)
	if err != nil {
		return nil, fmt.Errorf("resolve transitive deps: %w", err)
	}

	return deps, nil
}

func mutateEslintGoPnpmFnm(cfg *config.Config) {
	cfg.Tasks = []string{testModuleEslint, consts.Go}
	cfg.NodePackageManager = config.PMPnpm
	cfg.NodeVersionManager = config.VMFnm
	cfg.IncludesDoc = true
}

func assertGeneratedRootTasks(t *testing.T, plan *syncer.Plan) {
	t.Helper()

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

// TestBuildPlanInitialSync verifies an initial sync marks the plan changed and generates root tasks.
func TestBuildPlanInitialSync(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	cfg := testConfig(workspace, mutateEslintGoPnpmFnm)
	_, plan := preparePlan(t, workspace, cfg)

	if !plan.Changed {
		t.Fatal(errExpectedChangesInitial)
	}

	assertGeneratedRootTasks(t, plan)
}

// TestBuildPlanCreatesRootTaskfile verifies the root Taskfile.yml is added on initial sync.
func TestBuildPlanCreatesRootTaskfile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	snap := fixtureStore(t)
	cfg := testConfig(workspace, mutateGoWithDocs)

	resolutions, depSources := resolveModulesForTest(t, cfg, snap)
	plan := buildPlanFrom(t, cfg, snap, resolutions, depSources)

	if !plan.Changed {
		t.Fatal(errExpectedChangesInitial)
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
		consts.FilePerm755,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(workspace, config.DefaultTargetFolder, testModuleEslint, "user.txt"),
		[]byte(contentKeep),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func buildEslintSyncInput(t *testing.T, cfg *config.Config, snap *store.Snapshot) syncer.SyncInput {
	t.Helper()

	res, err := resolver.Resolve(
		testModuleEslint,
		snap.Catalog,
		cfg.NodePackageManager,
		cfg.NodeVersionManager,
	)
	if err != nil {
		t.Fatal(err)
	}

	return syncer.SyncInput{
		Config:   cfg,
		Snapshot: snap,
		Requested: map[string]syncer.ModuleRecord{
			testModuleEslint: {
				SourceModule:      res.SourceModule,
				DestinationModule: testModuleEslint,
				Path:              config.DefaultTargetFolder + "/" + testModuleEslint,
			},
		},
		Dependencies: nil,
		SourceToDest: map[string]string{res.SourceModule: testModuleEslint},
		DestByTask:   map[string]string{testModuleEslint: testModuleEslint},
	}
}

func mutateEslintPnpmFnm(cfg *config.Config) {
	cfg.Tasks = []string{testModuleEslint}
	cfg.NodePackageManager = config.PMPnpm
	cfg.NodeVersionManager = config.VMFnm
	cfg.IncludesDoc = true
}

// TestUnmanagedDestinationConflict verifies planning refuses to overwrite an unmanaged existing file.
func TestUnmanagedDestinationConflict(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	writeUnmanagedFile(t, workspace)

	snap := fixtureStore(t)
	cfg := testConfig(workspace, mutateEslintPnpmFnm)

	si := buildEslintSyncInput(t, cfg, snap)

	_, err := syncer.BuildPlan(&si)
	if err == nil {
		t.Fatal("expected unmanaged destination conflict")
	}
}

func mutateEslintPnpmFnmNoDocs(cfg *config.Config) {
	cfg.Tasks = []string{testModuleEslint}
	cfg.NodePackageManager = config.PMPnpm
	cfg.NodeVersionManager = config.VMFnm
	cfg.IncludesDoc = false
}

func assertNoReadmeManaged(t *testing.T, plan *syncer.Plan) {
	t.Helper()

	for _, managed := range plan.ManagedFiles {
		if filepath.Base(managed.Path) == testReadmeName {
			t.Fatal("README should be excluded when includes-doc=false")
		}
	}
}

// TestIncludesDocFalseSkipsReadme verifies README is excluded from managed files when includes-doc is false.
func TestIncludesDocFalseSkipsReadme(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	snap := fixtureStore(t)
	cfg := testConfig(workspace, mutateEslintPnpmFnmNoDocs)

	si := buildEslintSyncInput(t, cfg, snap)

	plan, err := syncer.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	assertNoReadmeManaged(t, plan)
}
