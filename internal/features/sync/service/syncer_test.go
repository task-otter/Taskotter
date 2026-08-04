// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

func fixtureStore(t *testing.T) *storedomain.Snapshot {
	t.Helper()

	root := filepath.Join(
		consts.PathParent, consts.PathParent, consts.PathParent, consts.PathParent,
		dirTests, dirFixtures, dirStore,
	)

	snap, err := storesvc.LocalSnapshot(root, &storedomain.RefInfo{
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

	writeFileWithDir(t, filepath.Join(workspace, testTaskfileName), content)
}

func dependencySources(
	t *testing.T,
	sources []string,
	snap *storedomain.Snapshot,
) ([]string, error) {
	t.Helper()

	deps, err := resolvesvc.ResolveTransitive(sources, snap.Deps)
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

func assertGeneratedRootTaskNames(t *testing.T, rootText string) {
	t.Helper()

	wants := []string{
		"task: go:lint",
		"task: eslint:lint",
		"task: go:lint:fix",
		"task: eslint:lint:fix",
	}

	for i := range wants {
		if !strings.Contains(rootText, wants[i]) {
			t.Fatalf("expected %q in root Taskfile:\n%s", wants[i], rootText)
		}
	}
}

func assertGeneratedRootTasks(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	rootText := string(plan.RootTaskfile)
	assertGeneratedRootTaskNames(t, rootText)

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
	plan := replan(t, workspace, cfg)

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

	resolutions, depSources := resolveModsForTest(&moduleTestInput{t: t, cfg: cfg, snap: snap})
	plan := buildPlanFrom(&buildPlanFromInput{
		t: t, cfg: cfg, snap: snap, resolutions: resolutions, depSources: depSources,
	})

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

func writeUnmanagedFile(t *testing.T, workspace string) {
	t.Helper()

	dir := filepath.Join(workspace, config.DefaultTargetFolder, testModuleEslint)

	err := os.MkdirAll(dir, consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	writeFileWithDir(
		t,
		filepath.Join(dir, testFileUserTxt),
		[]byte(contentKeep),
	)
}

func resolveEslintMod(
	t *testing.T,
	cfg *config.Config,
	snap *storedomain.Snapshot,
) resolvesvc.Resolution {
	t.Helper()

	return mustResolveTask(t, &resolvesvc.ResolveInput{
		Task: testModuleEslint, Catalog: snap.Catalog,
		PackageManager: cfg.NodePackageManager, VersionManager: cfg.NodeVersionManager,
	})
}

func buildEslintSyncInput(
	t *testing.T,
	cfg *config.Config,
	snap *storedomain.Snapshot,
) syncdomain.SyncInput {
	t.Helper()

	res := resolveEslintMod(t, cfg, snap)

	return eslintSyncInput(cfg, snap, res.SourceModule)
}

func eslintSyncInput(
	cfg *config.Config,
	snap *storedomain.Snapshot,
	source string,
) syncdomain.SyncInput {
	return syncdomain.SyncInput{
		Config:      cfg,
		TaskfileOps: synctaskfile.Ops{},
		Snapshot:    snap,
		Requested: map[string]lockmodel.ModuleRecord{
			testModuleEslint: {
				SourceModule:      source,
				DestinationModule: testModuleEslint,
				Path:              config.DefaultTargetFolder + "/" + testModuleEslint,
			},
		},
		Dependencies: nil,
		SourceToDest: map[string]string{source: testModuleEslint},
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

	expectBuildPlanError(t, &si, "expected unmanaged destination conflict")
}

func mutateEslintPnpmFnmNoDocs(cfg *config.Config) {
	cfg.Tasks = []string{testModuleEslint}
	cfg.NodePackageManager = config.PMPnpm
	cfg.NodeVersionManager = config.VMFnm
	cfg.IncludesDoc = false
}

func assertNoReadmeManaged(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	for i := range plan.ManagedFiles {
		if filepath.Base(plan.ManagedFiles[i].Path) == testReadmeName {
			t.Fatal("README should be excluded when includes-doc=false")
		}
	}
}

func goManagedPath(rel string) string {
	return pathutil.JoinRelative(config.DefaultTargetFolder, consts.Go, rel)
}

func managedPathSet(plan *syncdomain.Plan) map[string]struct{} {
	out := make(map[string]struct{}, len(plan.ManagedFiles))

	for i := range plan.ManagedFiles {
		out[plan.ManagedFiles[i].Path] = struct{}{}
	}

	return out
}

func assertManagedHasPath(t *testing.T, paths map[string]struct{}, rel string) {
	t.Helper()

	full := goManagedPath(rel)

	if _, ok := paths[full]; !ok {
		t.Fatalf("expected managed path %q", full)
	}
}

func assertManagedLacksPath(t *testing.T, paths map[string]struct{}, rel string) {
	t.Helper()

	full := goManagedPath(rel)

	if _, ok := paths[full]; ok {
		t.Fatalf("managed path %q should be excluded when includes-doc=false", full)
	}
}

func assertNoDocPathsManaged(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	paths := managedPathSet(plan)

	assertNoReadmeManaged(t, plan)
	assertManagedLacksPath(t, paths, docGuideMD)
	assertManagedLacksPath(t, paths, docNestedNoteMD)
}

func assertDocPathsManaged(t *testing.T, plan *syncdomain.Plan) {
	t.Helper()

	paths := managedPathSet(plan)

	assertManagedHasPath(t, paths, testReadmeName)
	assertManagedHasPath(t, paths, docGuideMD)
	assertManagedHasPath(t, paths, docNestedNoteMD)
}

// TestIncludesDocFalseSkipsReadme verifies README is excluded from managed files when includes-doc is false.
func TestIncludesDocFalseSkipsReadme(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	snap := fixtureStore(t)
	cfg := testConfig(workspace, mutateEslintPnpmFnmNoDocs)
	si := buildEslintSyncInput(t, cfg, snap)

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	assertNoReadmeManaged(t, plan)
}

// TestIncludesDocFalseSkipsFixtureDocs verifies go fixture README and docs/ are excluded when includes-doc is false.
func TestIncludesDocFalseSkipsFixtureDocs(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{consts.Go}
		cfg.IncludesDoc = false
	})
	si := buildSingleSyncIn(&moduleTestInput{t: t, cfg: cfg, snap: fixtureStore(t)})

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	assertNoDocPathsManaged(t, plan)
}

// TestIncludesDocTrueIncludesFixtureDocs verifies go fixture README and docs/ are managed when includes-doc is true.
func TestIncludesDocTrueIncludesFixtureDocs(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	cfg := testConfig(workspace, mutateGoWithDocs)
	si := buildSingleSyncIn(&moduleTestInput{t: t, cfg: cfg, snap: fixtureStore(t)})

	plan, err := syncsvc.BuildPlan(&si)
	if err != nil {
		t.Fatal(err)
	}

	assertDocPathsManaged(t, plan)
}
