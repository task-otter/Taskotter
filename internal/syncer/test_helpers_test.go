// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"fmt"
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/syncer"
)

const (
	testModuleEslint = "eslint"
	testTaskfileName = "Taskfile.yml"
	testReadmeName   = "README.md"

	testLegacyMetaDir          = ".taskotter"
	testGoTaskfilePath         = "go/Taskfile.yml"
	testLockFileName           = ".taskotter-lock.yml"
	testMetadataFileName       = "metadata.yml"
	testStagingDir             = "staging"
	dirTests                   = "tests"
	dirFixtures                = "fixtures"
	dirStore                   = "store"
	notFoundIndex              = -1
	testMetadataRelPath        = ".taskotter/metadata.yml"
	testInvalidYAML            = "{{not yaml"
	testBadYAML                = "{{bad"
	errExpectedCorruptLock     = "expected corrupt lock error"
	errExpectedCorruptMetadata = "expected corrupt metadata error"
	taskGoTaskfilePath         = "task/go/Taskfile.yml"
	contentKeep                = "keep"
	targetFolderTask           = "task"
	docGuideMD                 = "docs/guide.md"
	fileGoTestGo               = "go_test.go"
	docMetadataYML             = "docs/metadata.yml"
	errExpectedChangesInitial  = "expected changes on initial sync"
)

func runApplyPlan(t *testing.T, plan *syncer.Plan, syncInput *syncer.SyncInput) error {
	t.Helper()

	err := syncer.ApplyPlan(plan, syncInput)
	if err != nil {
		return fmt.Errorf("apply plan: %w", err)
	}

	return nil
}

func withCopyFileHook(
	t *testing.T,
	plan *syncer.Plan,
	hook func(path string, entry syncer.FileEntry) error,
	run func(),
) {
	t.Helper()

	syncer.SetCopyFileToHookForTest(plan, hook)
	t.Cleanup(func() { syncer.SetCopyFileToHookForTest(plan, nil) })

	run()
}

func testConfig(workspace string, mutate func(*config.Config)) *config.Config {
	cfg := &config.Config{
		Tasks:              nil,
		JSRuntime:          consts.Empty,
		NodePackageManager: consts.Empty,
		NodeVersionManager: consts.Empty,
		IncludesDoc:        false,
		SyncRoot:           true,
		FailOnChanges:      false,
		StoreVersion:       consts.Empty,
		TargetFolder:       config.DefaultTargetFolder,
		RootTaskfile:       testTaskfileName,
		GitHubToken:        consts.Empty,
		Workspace:          workspace,
		Repository:         consts.Empty,
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}

	if mutate != nil {
		mutate(cfg)
	}

	return cfg
}
