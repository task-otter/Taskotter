// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"strconv"
	"testing"

	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prservice "github.com/task-otter/Taskotter/internal/features/pr/service"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	benchModuleCount  = 24
	benchFileCount    = 120
	benchModulePrefix = "mod"
	benchTargetFolder = "taskfiles"
	benchStoreVersion = "v1.4.0"
	benchSourceRef    = "refs/tags/v1.4.0"
	benchCommit       = "0f1b0f2f9a2f4a1d8c0e7b3a5d9c1e4f6a8b0d2c"
	benchAddedName    = "added"
	benchUpdatedName  = "updated"
	benchRemovedName  = "removed"
	benchFileSuffix   = ".yml"
	benchBranchMain   = "main"
)

func benchModuleName(idx int) string {
	return benchModulePrefix + strconv.Itoa(idx)
}

func benchModuleRecord(idx int) lockmodel.ModuleRecord {
	name := benchModuleName(idx)

	return lockmodel.ModuleRecord{
		SourceModule:      name,
		DestinationModule: name,
		Path:              benchTargetFolder + consts.PathSepString + name,
	}
}

func benchTaskNames() []string {
	tasks := make([]string, consts.IndexZero, benchModuleCount)

	for idx := range benchModuleCount {
		tasks = append(tasks, benchModuleName(idx))
	}

	return tasks
}

func benchRequestedRecords() map[string]lockmodel.ModuleRecord {
	requested := make(map[string]lockmodel.ModuleRecord, benchModuleCount)

	for idx := range benchModuleCount {
		requested[benchModuleName(idx)] = benchModuleRecord(idx)
	}

	return requested
}

func benchDependencyRecords() []lockmodel.ModuleRecord {
	deps := make([]lockmodel.ModuleRecord, consts.IndexZero, benchModuleCount)

	for idx := range benchModuleCount {
		deps = append(deps, benchModuleRecord(idx))
	}

	return deps
}

func benchFilePath(idx int, kind string) string {
	name := benchModuleName(idx % benchModuleCount)
	dir := benchTargetFolder + consts.PathSepString + name

	return dir + consts.PathSepString + kind + strconv.Itoa(idx) + benchFileSuffix
}

func benchFilePaths(kind string) []string {
	paths := make([]string, consts.IndexZero, benchFileCount)

	for idx := range benchFileCount {
		paths = append(paths, benchFilePath(idx, kind))
	}

	return paths
}

func benchBodyConfig() *config.Config {
	cfg := &config.Config{}

	cfg.Tasks = benchTaskNames()
	cfg.JSRuntime = config.JSRuntimeNodeJS
	cfg.NodePackageManager = config.PMPnpm
	cfg.TargetFolder = benchTargetFolder
	cfg.StoreVersion = benchStoreVersion
	cfg.IncludesDoc = true
	cfg.SyncRoot = true

	return cfg
}

func benchBodyPlan() *syncdomain.Plan {
	plan := &syncdomain.Plan{}

	plan.Requested = benchRequestedRecords()
	plan.Dependencies = benchDependencyRecords()
	plan.Added = benchFilePaths(benchAddedName)
	plan.Updated = benchFilePaths(benchUpdatedName)
	plan.Removed = benchFilePaths(benchRemovedName)
	plan.RootTaskfilePath = consts.Taskfile
	plan.Changed = true

	return plan
}

func benchBodyStoreRef() *prdomain.StoreRef {
	return &prdomain.StoreRef{
		SourceRef:      benchSourceRef,
		ResolvedCommit: benchCommit,
		DefaultBranch:  benchBranchMain,
	}
}

// BenchmarkBuildPRBody measures rendering the sync pull request markdown body.
func BenchmarkBuildPRBody(b *testing.B) {
	cfg := benchBodyConfig()
	plan := benchBodyPlan()
	ref := benchBodyStoreRef()

	for b.Loop() {
		iox.Discard(prservice.BuildPRBody(cfg, plan, ref))
	}
}
