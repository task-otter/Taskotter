// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"strings"
	"testing"

	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	prservice "github.com/task-otter/Taskotter/internal/features/pr/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

type (
	writeOutputsArgs struct {
		Values   map[string]string
		RootDir  string
		FileName string
	}
)

const (
	taskESLint = "eslint"

	modPnpm = "pnpm"

	testEslintChain = "eslint/node/pnpm"
)

// TestBuildPRBody verifies the pull request body includes requested and dependency modules.
func TestBuildPRBody(t *testing.T) {
	t.Parallel()

	body := prservice.BuildPRBody(prBodyConfig(), buildPRBodyPlan(), buildPRBodyStoreRef())

	if !strings.Contains(body, testEslintChain) {
		t.Fatalf("missing module info: %s", body)
	}
}

// TestWriteOutputsMultilineJSON verifies multiline values are written using heredoc syntax.
func TestWriteOutputsMultilineJSON(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()

	values := map[string]string{
		"changed":        "true",
		"resolved-tasks": "{\n  \"go\": {}\n}\n",
	}

	text := writeOutputsAndRead(t, &writeOutputsArgs{
		RootDir:  rootDir,
		FileName: "output",
		Values:   values,
	})

	if !strings.Contains(text, "resolved-tasks<<EOF") {
		t.Fatalf("expected heredoc output: %s", text)
	}
}

func buildPRBodyPlan() *syncdomain.Plan {
	return &syncdomain.Plan{
		OldLock:          nil,
		CopyFileTo:       nil,
		ModuleContents:   nil,
		Requested:        prBodyRequestedModules(),
		Metadata:         emptyPRBodyMetadata(),
		OldTargetFolder:  consts.Empty,
		RootTaskfilePath: "Taskfile.yml",
		RootTaskfile:     nil,
		Updated:          nil,
		Removed:          nil,
		Added:            nil,
		ManagedFiles:     nil,
		StagePaths:       nil,
		Dependencies:     prBodyDependencyModules(),
		Lock:             emptyPRBodyLock(),
		Changed:          false,
	}
}

func prBodyRequestedModules() map[string]lockmodel.ModuleRecord {
	return map[string]lockmodel.ModuleRecord{
		taskESLint: {
			SourceModule:      testEslintChain,
			DestinationModule: taskESLint,
			Path:              "taskfiles/eslint",
		},
	}
}

func prBodyDependencyModules() []lockmodel.ModuleRecord {
	return []lockmodel.ModuleRecord{
		{SourceModule: modPnpm, DestinationModule: modPnpm, Path: "taskfiles/pnpm"},
	}
}

func emptyPRBodyMetadata() syncdomain.Metadata {
	return syncdomain.Metadata{
		TargetFolder:      consts.Empty,
		LockFile:          consts.Empty,
		ConfigurationHash: consts.Empty,
	}
}

func emptyPRBodyLock() lockmodel.LockFile {
	return lockmodel.LockFile{
		Source:             emptyPRBodyLockSource(),
		Requested:          nil,
		Dependencies:       nil,
		GeneratedRootTasks: nil,
		ManagedFiles:       nil,
		Configuration:      emptyPRBodyLockConfiguration(),
	}
}

func emptyPRBodyLockSource() lockmodel.LockSource {
	return lockmodel.LockSource{
		Repository:       consts.Empty,
		RequestedVersion: consts.Empty,
		SourceRef:        consts.Empty,
		ResolvedCommit:   consts.Empty,
		DefaultBranch:    consts.Empty,
	}
}

func emptyPRBodyLockConfiguration() lockmodel.LockConfiguration {
	return lockmodel.LockConfiguration{
		TargetFolder:       consts.Empty,
		NodePackageManager: consts.Empty,
		Tasks:              nil,
		IncludesDoc:        false,
		SyncRoot:           false,
	}
}

func buildPRBodyStoreRef() *prdomain.StoreRef {
	return prservice.StoreRefFrom(&storedomain.RefInfo{
		Repository:       "",
		RequestedVersion: "",
		SourceRef:        "refs/heads/main",
		ResolvedCommit:   "abc",
		DefaultBranch:    "main",
	})
}

func prBodyConfig() *config.Config {
	return &config.Config{
		Tasks:              []string{taskESLint},
		JSRuntime:          config.JSRuntimeNodeJS,
		NodePackageManager: config.PMPnpm,
		IncludesDoc:        true,
		SyncRoot:           true,
		FailOnChanges:      false,
		StoreVersion:       consts.Empty,
		TargetFolder:       "taskfiles",
		RootTaskfile:       "taskfiles/Taskfile.yml",
		GitHubToken:        consts.Empty,
		Workspace:          consts.Empty,
		Repository:         consts.Empty,
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}
}

func writeOutputsAndRead(t *testing.T, args *writeOutputsArgs) string {
	t.Helper()

	outputPath := args.RootDir + "/" + args.FileName

	err := iox.WriteGitHubOutputs(outputPath, args.Values)
	if err != nil {
		t.Fatal(err)
	}

	data, err := pathutil.ReadRelativeFile(args.RootDir, args.FileName)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}
