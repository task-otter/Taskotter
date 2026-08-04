// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"os"
	"path/filepath"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	migrationAssertInput struct {
		t          *testing.T
		workspace  string
		oldManaged string
		oldUser    string
	}

	oldTargetFixture struct {
		oldManaged string
		oldUser    string
	}
)

// TestPackageManagerSwitchSameDestination verifies different package manager variants normalize to eslint.
func TestPackageManagerSwitchSameDestination(t *testing.T) {
	t.Parallel()

	mods := []string{"eslint/node/fnm/pnpm", "eslint/node/fnm/npm"}

	for i := range mods {
		t.Run(mods[i], func(t *testing.T) {
			t.Parallel()

			assertNormalizesToEslint(t, mods[i])
		})
	}
}

// TestPrefixSafetyPreservesUnrelatedPaths verifies paths sharing a prefix with the target folder are untouched.
func TestPrefixSafetyPreservesUnrelatedPaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	extra := filepath.Join(workspace, "taskfiles-extra", "foo.txt")
	writeFileWithDir(t, extra, []byte("stay"))
	writeTaskGoManagedLock(t, workspace)

	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)
	discardCfg(cfg)

	assertApplyPlanPreservesPath(&preservePathInput{
		t: t, plan: plan, syncInput: &syncInput, path: extra,
	})
}

// TestTargetFolderMigration verifies files migrate to the new target folder while unmanaged files stay.
func TestTargetFolderMigration(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	fixture := writeOldTargetFixture(t, workspace)

	cfg, syncInput, plan := setupPlan(
		&setupPlanArgs{t: t, workspace: workspace, mutate: mutateGoWithDocs},
	)
	discardCfg(cfg)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertMigrationResult(&migrationAssertInput{
		t: t, workspace: workspace, oldManaged: fixture.oldManaged, oldUser: fixture.oldUser,
	})
}

func assertMigrationResult(input *migrationAssertInput) {
	input.t.Helper()

	assertFileExists(
		input.t,
		filepath.Join(input.workspace, config.DefaultTargetFolder, testGoTaskfilePath),
	)

	stat, err := os.Stat(input.oldManaged)
	iox.Discard(stat)

	if err == nil {
		input.t.Fatal("old managed file under previous target folder should be removed")
	}

	stat, err = os.Stat(input.oldUser)
	iox.Discard(stat)

	if err != nil {
		input.t.Fatal("unknown files outside managed set should be preserved")
	}
}

func assertNormalizesToEslint(t *testing.T, mod string) {
	t.Helper()

	sourceToDest, err := resolvesvc.BuildDestinationMap([]string{mod})
	if err != nil {
		t.Fatal(err)
	}

	if sourceToDest[mod] != testModuleEslint {
		t.Fatalf("%s should normalize to eslint, got %q", mod, sourceToDest[mod])
	}
}

func writeOldTargetFixture(t *testing.T, workspace string) oldTargetFixture {
	t.Helper()

	oldManaged := filepath.Join(workspace, taskGoTaskfilePath)
	writeFileWithDir(t, oldManaged, []byte("version: '3'\n"))

	oldUser := filepath.Join(workspace, targetFolderTask, consts.Go, testFileUserTxt)
	writeFileWithDir(t, oldUser, []byte(contentKeep))

	writeTaskGoManagedLock(t, workspace)

	return oldTargetFixture{oldManaged: oldManaged, oldUser: oldUser}
}

func writeTaskGoManagedLock(t *testing.T, workspace string) {
	t.Helper()

	writeMinimalLock(&lockWriteInput{
		t: t, workspace: workspace, targetFolder: targetFolderTask,
		files: []managed.File{{
			SourceModule:      consts.Empty,
			DestinationModule: consts.Go,
			SourcePath:        consts.Empty,
			Path:              taskGoTaskfilePath,
			SHA256:            consts.Empty,
		}},
	})
}
