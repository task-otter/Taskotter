// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package lockmodel_test

import (
	"strconv"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	benchManagedFiles  = 200
	benchModuleRecords = 24
	benchModulePrefix  = "mod"
	benchTargetFolder  = "taskfiles"
	benchSHA           = "6f1b0f2f9a2f4a1d8c0e7b3a5d9c1e4f6a8b0d2c4e6f8a0b2d4c6e8f0a2b4d6c"
	benchCommit        = "0f1b0f2f9a2f4a1d8c0e7b3a5d9c1e4f6a8b0d2c"
	benchFileSuffix    = ".yml"
	benchFilePrefix    = "/file"
)

func benchModuleName(idx int) string {
	return benchModulePrefix + strconv.Itoa(idx)
}

func benchModulePath(name string) string {
	return benchTargetFolder + consts.PathSepString + name
}

func benchManagedFile(idx int) managed.File {
	name := benchModuleName(idx % benchModuleRecords)
	dir := benchModulePath(name)
	rel := dir + benchFilePrefix + strconv.Itoa(idx) + benchFileSuffix

	return managed.File{
		SourceModule:      name,
		DestinationModule: name,
		SourcePath:        dir + consts.TaskfileSuffix,
		Path:              rel,
		SHA256:            benchSHA,
	}
}

func benchManagedFileList() []managed.File {
	files := make([]managed.File, consts.IndexZero, benchManagedFiles)

	for idx := range benchManagedFiles {
		files = append(files, benchManagedFile(idx))
	}

	return files
}

func benchModuleRecord(idx int) lockmodel.ModuleRecord {
	name := benchModuleName(idx)

	return lockmodel.ModuleRecord{
		SourceModule:      name,
		DestinationModule: name,
		Path:              benchModulePath(name),
	}
}

func benchRequested() lockmodel.OrderedRequested {
	requested := make(lockmodel.OrderedRequested, benchModuleRecords)

	for idx := range benchModuleRecords {
		requested[benchModuleName(idx)] = benchModuleRecord(idx)
	}

	return requested
}

func benchDependencies() []lockmodel.ModuleRecord {
	deps := make([]lockmodel.ModuleRecord, consts.IndexZero, benchModuleRecords)

	for idx := range benchModuleRecords {
		deps = append(deps, benchModuleRecord(idx))
	}

	return deps
}

func benchLockSource() lockmodel.LockSource {
	return lockmodel.LockSource{
		Repository:       "task-otter/store",
		RequestedVersion: "v1.4.0",
		SourceRef:        "refs/tags/v1.4.0",
		ResolvedCommit:   benchCommit,
		DefaultBranch:    "main",
	}
}

func benchLockConfiguration() lockmodel.LockConfiguration {
	tasks := make([]string, consts.IndexZero, benchModuleRecords)

	for idx := range benchModuleRecords {
		tasks = append(tasks, benchModuleName(idx))
	}

	return lockmodel.LockConfiguration{
		TargetFolder:       benchTargetFolder,
		NodePackageManager: "pnpm",
		Tasks:              tasks,
		IncludesDoc:        true,
		SyncRoot:           true,
	}
}

func benchLockFile() *lockmodel.LockFile {
	return &lockmodel.LockFile{
		Source:             benchLockSource(),
		Requested:          benchRequested(),
		Dependencies:       benchDependencies(),
		GeneratedRootTasks: []string{"ci", "install", "lint", "version"},
		ManagedFiles:       benchManagedFileList(),
		Configuration:      benchLockConfiguration(),
	}
}

// BenchmarkMarshalLock measures encoding a populated lock file to YAML.
func BenchmarkMarshalLock(b *testing.B) {
	lock := benchLockFile()

	for b.Loop() {
		iox.Discard(lockmodel.MarshalLock(lock))
	}
}

// BenchmarkDecodeLockFileYAML measures decoding a populated lock file from YAML.
func BenchmarkDecodeLockFileYAML(b *testing.B) {
	data := lockmodel.MarshalLock(benchLockFile())

	for b.Loop() {
		var lock lockmodel.LockFile

		err := lockmodel.DecodeLockFileYAML(data, &lock)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(lock)
	}
}
