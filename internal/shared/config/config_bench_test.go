// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	benchTaskCount    = 24
	benchModulePrefix = "mod"
	benchJSInput      = "runtime: nodejs\npackage-manager: pnpm\n"
	benchAuthValue    = "bench-auth"
	benchRepository   = "owner/repo"
	benchTargetFolder = "automation/taskfiles"
	benchSyncRootKey  = "INPUT_SYNC_ROOT"
	benchBaseRefKey   = "GITHUB_BASE_REF"
	benchRepoKey      = "GITHUB_REPOSITORY"
	benchMainRef      = "refs/heads/main"
)

func benchTaskList() string {
	var out strings.Builder

	for idx := range benchTaskCount {
		name := benchModulePrefix + strconv.Itoa(idx)

		iox.Discard(iox.WriteStringFull(&out, name+consts.Newline))
	}

	return out.String()
}

func benchEnv(workspace string) map[string]string {
	return map[string]string{
		consts.InputTasks:         benchTaskList(),
		consts.InputJS:            benchJSInput,
		inputIncludesDoc:          testTrueValue,
		benchSyncRootKey:          testTrueValue,
		consts.InputStoreVersion:  testStoreVersionTag,
		consts.InputTargetFolder:  benchTargetFolder,
		consts.InputGithubToken:   benchAuthValue,
		consts.EnvGithubWorkspace: workspace,
		benchRepoKey:              benchRepository,
		consts.GitHubRefEnv:       benchMainRef,
		benchBaseRefKey:           consts.Empty,
	}
}

// BenchmarkLoadFromEnv measures parsing, validating, and hashing the action inputs.
func BenchmarkLoadFromEnv(b *testing.B) {
	env := benchEnv(b.TempDir())

	for key := range env {
		b.Setenv(key, env[key])
	}

	for b.Loop() {
		cfg, err := config.LoadFromEnv()
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(cfg)
	}
}
