package syncer_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/syncer"
)

const (
	testModuleEslint = "eslint"
	testTaskfileName = "Taskfile.yml"
	testReadmeName   = "README.md"
)

func runApplyPlan(t *testing.T, plan *syncer.Plan, syncInput syncer.SyncInput) error {
	t.Helper()

	return syncer.ApplyPlan(plan, syncInput)
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
		JSRuntime:          "",
		NodePackageManager: "",
		NodeVersionManager: "",
		IncludesDoc:        false,
		SyncRoot:           true,
		FailOnChanges:      false,
		StoreVersion:       "",
		TargetFolder:       config.DefaultTargetFolder,
		RootTaskfile:       testTaskfileName,
		GitHubToken:        "",
		Workspace:          workspace,
		Repository:         "",
		GitHubOutput:       "",
		BaseBranch:         "",
		ConfigurationHash:  "",
		BranchName:         "",
	}
	if mutate != nil {
		mutate(cfg)
	}

	return cfg
}
