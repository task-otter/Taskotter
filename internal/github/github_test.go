// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/github"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
)

const (
	taskESLint      = "eslint"
	testEslintChain = "eslint/node/fnm/pnpm"
)

func writeOutputsAndRead(t *testing.T, path string, values map[string]string) string {
	t.Helper()

	err := github.WriteOutputs(path, values)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

// TestWriteOutputsMultilineJSON verifies multiline values are written using heredoc syntax.
func TestWriteOutputsMultilineJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "output")

	values := map[string]string{
		"changed":        "true",
		"resolved-tasks": "{\n  \"go\": {}\n}\n",
	}

	text := writeOutputsAndRead(t, path, values)

	if !strings.Contains(text, "resolved-tasks<<EOF") {
		t.Fatalf("expected heredoc output: %s", text)
	}
}

func prBodyConfig() *config.Config {
	return &config.Config{
		Tasks:              []string{taskESLint},
		JSRuntime:          config.JSRuntimeNodeJS,
		NodePackageManager: config.PMPnpm,
		NodeVersionManager: config.VMFnm,
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

// TestBuildPRBody verifies the pull request body includes requested and dependency modules.
func TestBuildPRBody(t *testing.T) {
	t.Parallel()

	var lock syncer.LockFile

	var metadata syncer.Metadata

	cfg := prBodyConfig()
	plan := &syncer.Plan{
		Requested: map[string]syncer.ModuleRecord{
			taskESLint: {
				SourceModule:      testEslintChain,
				DestinationModule: taskESLint,
				Path:              "taskfiles/eslint",
			},
		},
		Dependencies: []syncer.ModuleRecord{
			{SourceModule: "pnpm/fnm", DestinationModule: "pnpm", Path: "taskfiles/pnpm"},
		},
		ManagedFiles:     nil,
		ModuleContents:   nil,
		RootTaskfile:     nil,
		RootTaskfilePath: "Taskfile.yml",
		Lock:             lock,
		Metadata:         metadata,
		Added:            nil,
		Updated:          nil,
		Removed:          nil,
		Changed:          false,
		OldLock:          nil,
		OldTargetFolder:  "",
		StagePaths:       nil,
	}

	body := github.BuildPRBody(cfg, plan, github.StoreRefFrom(&store.RefInfo{
		Repository:       "",
		RequestedVersion: "",
		SourceRef:        "refs/heads/main",
		ResolvedCommit:   "abc",
		DefaultBranch:    "main",
	}))

	if !strings.Contains(body, "eslint/node/fnm/pnpm") {
		t.Fatalf("missing module info: %s", body)
	}
}
