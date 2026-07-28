package syncer_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/app"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/resolver"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/syncer"
)

var errSimulatedPromoteFailure = errors.New("simulated promote failure")

func preparePlan(t *testing.T, _ string, cfg *config.Config) (syncer.SyncInput, *syncer.Plan) {
	t.Helper()
	snap := fixtureStore(t)

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

	depSources, err := dependencySources(t, sources, snap)
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

	return syncInput, plan
}

func writeFileEntry(path string, entry syncer.FileEntry) error {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return err
	}

	return os.WriteFile(path, entry.Data, entry.Mode)
}

func TestApplyPlanWritesFiles(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
	})

	syncInput, plan := preparePlan(t, workspace, cfg)

	err := runApplyPlan(t, plan, syncInput)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(workspace, config.DefaultTargetFolder, "go/Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(workspace, cfg.MetadataPath()))
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyPlanMigratesLegacyMetadataPath(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	err := os.MkdirAll(filepath.Join(workspace, ".taskotter"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(workspace, config.LegacyMetadataPath),
		[]byte("target_folder: taskfiles\nlock_file: taskfiles/.taskotter-lock.yml\n"),
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
	})

	syncInput, plan := preparePlan(t, workspace, cfg)

	err = runApplyPlan(t, plan, syncInput)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(workspace, cfg.MetadataPath()))
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(workspace, config.LegacyMetadataPath))
	if !os.IsNotExist(err) {
		t.Fatalf("legacy metadata should be removed, stat returned: %v", err)
	}
}

func TestApplyPlanSkipsRootTaskfileWhenDisabled(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	rootPath := filepath.Join(workspace, "Taskfile.yml")
	rootContent := "this is intentionally not valid Taskfile YAML: ["

	err := os.WriteFile(rootPath, []byte(rootContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
		cfg.SyncRoot = false
	})

	syncInput, plan := preparePlan(t, workspace, cfg)

	if containsRootTaskfile(plan.Added) || containsRootTaskfile(plan.Updated) {
		t.Fatalf(
			"root Taskfile.yml should not be in the diff: added=%v updated=%v",
			plan.Added,
			plan.Updated,
		)
	}

	if containsRootTaskfile(plan.StagePaths) {
		t.Fatalf("root Taskfile.yml should not be staged: %v", plan.StagePaths)
	}

	if plan.Lock.Configuration.SyncRoot {
		t.Fatal("lock SyncRoot = true, want false")
	}

	err = runApplyPlan(t, plan, syncInput)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != rootContent {
		t.Fatalf("root Taskfile.yml changed to %q, want %q", data, rootContent)
	}

	_, err = os.Stat(filepath.Join(workspace, config.DefaultTargetFolder, "go", "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyPlanPreservesExecutableMode(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	setupPath := filepath.Join(
		"..",
		"..",
		"tests",
		"fixtures",
		"store",
		"taskfiles",
		"go",
		"setup.sh",
	)

	info, err := os.Stat(setupPath)
	if err != nil {
		t.Fatal(err)
	}

	origMode := info.Mode().Perm()

	err = os.Chmod(setupPath, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.Chmod(setupPath, origMode)
	})

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
	})

	syncInput, plan := preparePlan(t, workspace, cfg)

	err = runApplyPlan(t, plan, syncInput)
	if err != nil {
		t.Fatal(err)
	}

	info, err = os.Stat(filepath.Join(workspace, config.DefaultTargetFolder, "go/setup.sh"))
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected executable bit, got %o", info.Mode().Perm())
	}
}

func TestApplyPlanPromoteBeforeDelete(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	obsolete := filepath.Join(workspace, config.DefaultTargetFolder, "go/obsolete.txt")

	err := os.MkdirAll(filepath.Dir(obsolete), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(obsolete, []byte("old"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	writeMinimalLock(t, workspace, config.DefaultTargetFolder, []syncer.ManagedFile{
		{
			SourceModule:      "",
			DestinationModule: "go",
			SourcePath:        "",
			Path:              config.DefaultTargetFolder + "/go/obsolete.txt",
			SHA256:            "",
		},
	})

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
	})
	syncInput, plan := preparePlan(t, workspace, cfg)

	var promoted int

	withCopyFileHook(t, func(path string, entry syncer.FileEntry) error {
		if strings.Contains(path, filepath.Join(".taskotter", "staging")) {
			return writeFileEntry(path, entry)
		}

		promoted++
		if promoted == 1 {
			return errSimulatedPromoteFailure
		}

		return writeFileEntry(path, entry)
	}, func() {
		err = syncer.ApplyPlan(plan, syncInput)
		if err == nil {
			t.Fatal("expected promote failure")
		}
	})

	_, err = os.Stat(obsolete)
	if err != nil {
		t.Fatal("obsolete file should remain when promote fails before delete")
	}
}

func TestApplyPlanWriteOrder(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
	})
	syncInput, plan := preparePlan(t, workspace, cfg)

	var order []string

	stagingMarker := filepath.Join(config.DefaultTargetFolder, ".taskotter", "staging")

	withCopyFileHook(t, func(path string, entry syncer.FileEntry) error {
		if strings.Contains(path, stagingMarker) {
			return writeFileEntry(path, entry)
		}

		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}

		order = append(order, filepath.ToSlash(rel))

		return writeFileEntry(path, entry)
	}, func() {
		err := syncer.ApplyPlan(plan, syncInput)
		if err != nil {
			t.Fatal(err)
		}
	})

	lockIdx := indexOfSuffix(order, ".taskotter-lock.yml")
	metaIdx := indexOfSuffix(order, "metadata.yml")

	moduleIdx := indexOfContains(order, config.DefaultTargetFolder+"/go/")
	if lockIdx < moduleIdx || metaIdx < lockIdx {
		t.Fatalf("expected modules before lock before metadata, got %v", order)
	}
}

func indexOfSuffix(paths []string, suffix string) int {
	for i, p := range paths {
		if strings.HasSuffix(p, suffix) {
			return i
		}
	}

	return -1
}

func indexOfContains(paths []string, part string) int {
	for i, p := range paths {
		if strings.Contains(p, part) {
			return i
		}
	}

	return -1
}
