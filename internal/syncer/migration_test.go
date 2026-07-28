package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/normalizer"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/syncer"
)

func TestTargetFolderMigration(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	oldManaged := filepath.Join(workspace, "task/go/Taskfile.yml")

	err := os.MkdirAll(filepath.Dir(oldManaged), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(oldManaged, []byte("version: '3'\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	oldUser := filepath.Join(workspace, "task/go/user.txt")

	err = os.WriteFile(oldUser, []byte("keep"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	writeMinimalLock(t, workspace, "task", []syncer.ManagedFile{
		{
			SourceModule:      "",
			DestinationModule: "go",
			SourcePath:        "",
			Path:              "task/go/Taskfile.yml",
			SHA256:            "",
		},
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

	_, err = os.Stat(filepath.Join(workspace, config.DefaultTargetFolder, "go/Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(oldManaged)
	if err == nil {
		t.Fatal("old managed file under previous target folder should be removed")
	}

	_, err = os.Stat(oldUser)
	if err != nil {
		t.Fatal("unknown files outside managed set should be preserved")
	}
}

func TestPrefixSafetyPreservesUnrelatedPaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeRootTaskfile(t, workspace)

	extra := filepath.Join(workspace, "taskfiles-extra/foo.txt")

	err := os.MkdirAll(filepath.Dir(extra), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(extra, []byte("stay"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	writeMinimalLock(t, workspace, "task", []syncer.ManagedFile{
		{
			SourceModule:      "",
			DestinationModule: "go",
			SourcePath:        "",
			Path:              "task/go/Taskfile.yml",
			SHA256:            "",
		},
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

	_, err = os.Stat(extra)
	if err != nil {
		t.Fatal("taskfiles-extra must not match old folder prefix task")
	}
}

func TestPackageManagerSwitchSameDestination(t *testing.T) {
	t.Parallel()

	for _, mod := range []string{"eslint/node/fnm/pnpm", "eslint/node/fnm/npm"} {
		t.Run(mod, func(t *testing.T) {
			t.Parallel()

			sourceToDest, err := normalizer.BuildDestinationMap([]string{mod})
			if err != nil {
				t.Fatal(err)
			}

			if sourceToDest[mod] != testModuleEslint {
				t.Fatalf("%s should normalize to eslint, got %q", mod, sourceToDest[mod])
			}
		})
	}
}
