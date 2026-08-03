// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/normalizer"
	"github.com/task-otter/Taskotter/internal/syncer"
)

func writeTaskGoManagedLock(t *testing.T, workspace string) {
	t.Helper()

	writeMinimalLock(t, workspace, targetFolderTask, []syncer.ManagedFile{
		{
			SourceModule:      consts.Empty,
			DestinationModule: consts.Go,
			SourcePath:        consts.Empty,
			Path:              taskGoTaskfilePath,
			SHA256:            consts.Empty,
		},
	})
}

func writeOldTargetFixture(t *testing.T, workspace string) (string, string) {
	t.Helper()

	oldManaged := filepath.Join(workspace, taskGoTaskfilePath)
	writeFileWithDir(t, oldManaged, []byte("version: '3'\n"), consts.FilePerm644)

	oldUser := filepath.Join(workspace, "task/go/user.txt")
	writeFileWithDir(t, oldUser, []byte(contentKeep), consts.FilePerm644)

	writeTaskGoManagedLock(t, workspace)

	return oldManaged, oldUser
}

func assertMigrationResult(t *testing.T, workspace, oldManaged, oldUser string) {
	t.Helper()

	assertFileExists(t, filepath.Join(workspace, config.DefaultTargetFolder, testGoTaskfilePath))

	_, err := os.Stat(oldManaged)
	if err == nil {
		t.Fatal("old managed file under previous target folder should be removed")
	}

	_, err = os.Stat(oldUser)
	if err != nil {
		t.Fatal("unknown files outside managed set should be preserved")
	}
}

// TestTargetFolderMigration verifies files migrate to the new target folder while unmanaged files stay.
func TestTargetFolderMigration(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	oldManaged, oldUser := writeOldTargetFixture(t, workspace)

	_, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertMigrationResult(t, workspace, oldManaged, oldUser)
}

// TestPrefixSafetyPreservesUnrelatedPaths verifies paths sharing a prefix with the target folder are untouched.
func TestPrefixSafetyPreservesUnrelatedPaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	extra := filepath.Join(workspace, "taskfiles-extra/foo.txt")
	writeFileWithDir(t, extra, []byte("stay"), consts.FilePerm644)
	writeTaskGoManagedLock(t, workspace)

	_, syncInput, plan := setupPlan(t, workspace, mutateGoWithDocs)

	err := runApplyPlan(t, plan, &syncInput)
	if err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, extra)
}

func assertNormalizesToEslint(t *testing.T, mod string) {
	t.Helper()

	sourceToDest, err := normalizer.BuildDestinationMap([]string{mod})
	if err != nil {
		t.Fatal(err)
	}

	if sourceToDest[mod] != testModuleEslint {
		t.Fatalf("%s should normalize to eslint, got %q", mod, sourceToDest[mod])
	}
}

// TestPackageManagerSwitchSameDestination verifies different package manager variants normalize to eslint.
func TestPackageManagerSwitchSameDestination(t *testing.T) {
	t.Parallel()

	for _, mod := range []string{"eslint/node/fnm/pnpm", "eslint/node/fnm/npm"} {
		t.Run(mod, func(t *testing.T) {
			t.Parallel()

			assertNormalizesToEslint(t, mod)
		})
	}
}
