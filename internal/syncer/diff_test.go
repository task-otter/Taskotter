package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/app"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/resolver"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/syncer"
	"gopkg.in/yaml.v3"
)

// writeCorruptLock seeds workspace with metadata pointing at a lock file whose
// contents are not valid YAML.
func writeCorruptLock(t *testing.T, workspace string) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(workspace, config.DefaultTargetFolder), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(workspace, config.DefaultTargetFolder, ".taskotter-lock.yml")

	err = os.WriteFile(lockPath, []byte("{{not yaml"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	meta := []byte(`target_folder: taskfiles
lock_file: taskfiles/.taskotter-lock.yml
configuration_hash: abc
`)

	err = os.MkdirAll(filepath.Join(workspace, config.DefaultTargetFolder, ".taskotter"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(workspace, config.DefaultTargetFolder, ".taskotter/metadata.yml"),
		meta,
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildPlanCorruptLockFails(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	writeCorruptLock(t, workspace)

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
	})
	snap := fixtureStore(t)

	res, err := resolver.Resolve("go", snap.Catalog, "", "")
	if err != nil {
		t.Fatal(err)
	}

	syncInput := syncer.SyncInput{
		Config:   cfg,
		Snapshot: snap,
		Requested: map[string]syncer.ModuleRecord{
			"go": {
				SourceModule:      res.SourceModule,
				DestinationModule: "go",
				Path:              config.DefaultTargetFolder + "/go",
			},
		},
		Dependencies: nil,
		SourceToDest: map[string]string{res.SourceModule: "go"},
		DestByTask:   map[string]string{"go": "go"},
	}

	_, err = syncer.BuildPlan(syncInput)
	if err == nil {
		t.Fatal("expected corrupt lock error")
	}
}

func TestBuildPlanCorruptMetadataFails(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	err := os.MkdirAll(filepath.Join(workspace, config.DefaultTargetFolder, ".taskotter"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(
		filepath.Join(workspace, config.DefaultTargetFolder, ".taskotter/metadata.yml"),
		[]byte("{{not yaml"),
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
	})
	snap := fixtureStore(t)

	res, err := resolver.Resolve("go", snap.Catalog, "", "")
	if err != nil {
		t.Fatal(err)
	}

	syncInput := syncer.SyncInput{
		Config:   cfg,
		Snapshot: snap,
		Requested: map[string]syncer.ModuleRecord{
			"go": {
				SourceModule:      res.SourceModule,
				DestinationModule: "go",
				Path:              config.DefaultTargetFolder + "/go",
			},
		},
		Dependencies: nil,
		SourceToDest: map[string]string{res.SourceModule: "go"},
		DestByTask:   map[string]string{"go": "go"},
	}

	_, err = syncer.BuildPlan(syncInput)
	if err == nil {
		t.Fatal("expected corrupt metadata error")
	}
}

func TestMetadataOnlyChangeMarksChanged(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)
	cfg := testConfig(workspace, func(cfg *config.Config) {
		cfg.Tasks = []string{"go"}
		cfg.IncludesDoc = true
		cfg.ConfigurationHash = "hash-a"
	})

	syncInput, plan := preparePlan(t, workspace, cfg)

	err := runApplyPlan(t, plan, syncInput)
	if err != nil {
		t.Fatal(err)
	}

	cfg.ConfigurationHash = "hash-b"

	_, plan2 := preparePlan(t, workspace, cfg)
	if !plan2.Changed {
		t.Fatal("expected metadata-only configuration hash change to mark plan changed")
	}
}

func TestSHAOnlyLockChangeNotChanged(t *testing.T) {
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

	snap := fixtureStore(t)
	snap.Ref.ResolvedCommit = "different-sha-only"

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

	syncInput2, err := app.PrepareSyncInput(cfg, snap, resolutions, depSources)
	if err != nil {
		t.Fatal(err)
	}

	plan2, err := syncer.BuildPlan(syncInput2)
	if err != nil {
		t.Fatal(err)
	}

	if plan2.Changed {
		t.Fatalf(
			"expected no file changes when only resolved commit differs: added=%v updated=%v removed=%v",
			plan2.Added,
			plan2.Updated,
			plan2.Removed,
		)
	}
}

func TestConfigurationChangeMarksUpdated(t *testing.T) {
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

	cfg.IncludesDoc = false

	_, plan2 := preparePlan(t, workspace, cfg)
	if !plan2.Changed {
		t.Fatal("expected changes when includes-doc toggles")
	}
}

func writeMinimalLock(t *testing.T, workspace, targetFolder string, files []syncer.ManagedFile) {
	t.Helper()

	var lock syncer.LockFile

	lock.Configuration.TargetFolder = targetFolder
	lock.ManagedFiles = files

	data, err := yaml.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(workspace, targetFolder, ".taskotter-lock.yml")

	err = os.MkdirAll(filepath.Dir(lockPath), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(lockPath, data, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	meta := []byte(
		"target_folder: " + targetFolder +
			"\nlock_file: " + targetFolder + "/.taskotter-lock.yml\nconfiguration_hash: x\n",
	)

	err = os.MkdirAll(filepath.Join(workspace, targetFolder, ".taskotter"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(workspace, targetFolder, ".taskotter/metadata.yml"), meta, 0o644)
	if err != nil {
		t.Fatal(err)
	}
}
