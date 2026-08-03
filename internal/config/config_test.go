// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
)

const (
	testBaseBranch         = "release/2026"
	testInvalidBool        = "yes"
	testFalseValue         = "false"
	testBuildTaskfile      = "build/Taskfile.yml"
	testBadInputMsg        = "bad input"
	testFallbackToken      = "fallback-token"
	testDockerToken        = "docker-token"
	testCustomTaskfiles    = "custom/taskfiles"
	testTaskEslint         = "eslint"
	testStoreVersionTag    = "v1.2.3"
	fmtWantFnm             = "NodeVersionManager = %q, want fnm"
	fmtTasksWant           = "Tasks = %#v, want %#v"
	fmtBaseBranchWant      = "BaseBranch = %q, want %s"
	fmtJSRuntimeWantNodeJS = "JSRuntime = %q, want nodejs"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()

	for k := range kv {
		t.Setenv(k, kv[k])
	}
}

func loadEnvOK(t *testing.T, env map[string]string) *config.Config {
	t.Helper()

	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	return cfg
}

func loadEnvExpectError(t *testing.T, env map[string]string, msg string) {
	t.Helper()

	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal(msg)
	}
}

func baseEnv(workspace string) map[string]string {
	return map[string]string{
		consts.InputTasks:        consts.Go,
		consts.InputJS:           consts.Empty,
		"INPUT_INCLUDES_DOC":     consts.Empty,
		"INPUT_SYNC_ROOT":        consts.Empty,
		consts.InputStoreVersion: consts.Empty,
		consts.InputTargetFolder: consts.Empty,
		consts.InputGithubToken:  "token",
		"GITHUB_WORKSPACE":       workspace,
		"GITHUB_REPOSITORY":      "owner/repo",
		consts.GitHubRefEnv:      consts.Empty,
		"GITHUB_BASE_REF":        consts.Empty,
	}
}

// TestLoadFromEnvUsesTriggerBranchAsPRBase verifies a push ref sets the PR base to the trigger branch.
func TestLoadFromEnvUsesTriggerBranchAsPRBase(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.GitHubRefEnv] = "refs/heads/" + testBaseBranch
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	if cfg.BaseBranch != testBaseBranch {
		t.Fatalf(fmtBaseBranchWant, cfg.BaseBranch, testBaseBranch)
	}
}

// TestLoadFromEnvUsesPullRequestTargetAsPRBase verifies a pull request ref uses GITHUB_BASE_REF as the PR base.
func TestLoadFromEnvUsesPullRequestTargetAsPRBase(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.GitHubRefEnv] = "refs/pull/42/merge"
	env["GITHUB_BASE_REF"] = testBaseBranch
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	if cfg.BaseBranch != testBaseBranch {
		t.Fatalf(fmtBaseBranchWant, cfg.BaseBranch, testBaseBranch)
	}
}

// TestLoadFromEnvDefaults verifies default paths, includes-doc, sync-root, and tasks are set.
func TestLoadFromEnvDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg := loadEnvOK(t, baseEnv(dir))

	assertDefaultPaths(t, cfg)

	if !cfg.IncludesDoc {
		t.Fatal("IncludesDoc should default to true")
	}

	if !cfg.SyncRoot {
		t.Fatal("SyncRoot should default to true")
	}

	if len(cfg.Tasks) != consts.IndexOne || cfg.Tasks[consts.IndexZero] != consts.Go {
		t.Fatalf("Tasks = %#v", cfg.Tasks)
	}
}

func assertDefaultPaths(t *testing.T, cfg *config.Config) {
	t.Helper()

	if cfg.TargetFolder != "taskfiles" {
		t.Fatalf("TargetFolder = %q, want taskfiles", cfg.TargetFolder)
	}

	if cfg.RootTaskfile != "taskfiles/Taskfile.yml" {
		t.Fatalf("RootTaskfile = %q, want taskfiles/Taskfile.yml", cfg.RootTaskfile)
	}

	if cfg.LockFilePath() != "taskfiles/.taskotter-lock.yml" {
		t.Fatalf("LockFilePath = %q, want taskfiles/.taskotter-lock.yml", cfg.LockFilePath())
	}

	if cfg.MetadataPath() != "taskfiles/.taskotter/metadata.yml" {
		t.Fatalf("MetadataPath = %q, want taskfiles/.taskotter/metadata.yml", cfg.MetadataPath())
	}
}

// TestRootTaskfileCustomPath verifies a custom root-taskfile input path is honored.
func TestRootTaskfileCustomPath(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_ROOT_TASKFILE"] = testBuildTaskfile
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	if cfg.RootTaskfile != testBuildTaskfile {
		t.Fatalf("RootTaskfile = %q, want build/Taskfile.yml", cfg.RootTaskfile)
	}
}

// TestRootTaskfileFollowsTargetFolder verifies the default root taskfile path tracks target-folder.
func TestRootTaskfileFollowsTargetFolder(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputTargetFolder] = "tools/tasks"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	if cfg.RootTaskfile != "tools/tasks/Taskfile.yml" {
		t.Fatalf("RootTaskfile = %q, want tools/tasks/Taskfile.yml", cfg.RootTaskfile)
	}
}

// TestRootTaskfileMustBeYAML verifies a non-YAML root-taskfile path is rejected.
func TestRootTaskfileMustBeYAML(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_ROOT_TASKFILE"] = "build/Taskfile.txt"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for non-YAML root-taskfile path")
	}
}

// TestValidationErrorWithoutField verifies Error() returns just the message when Field is empty.
func TestValidationErrorWithoutField(t *testing.T) {
	t.Parallel()

	err := (&config.ValidationError{Field: consts.Empty, Message: testBadInputMsg}).Error()

	if err != testBadInputMsg {
		t.Fatalf("Error() = %q", err)
	}
}

// TestMissingRuntimeInputs verifies missing workspace or token inputs cause an error.
func TestMissingRuntimeInputs(t *testing.T) {
	dir := t.TempDir()

	env := baseEnv(dir)

	env["GITHUB_WORKSPACE"] = consts.Empty
	loadEnvExpectError(t, env, "expected missing workspace error")

	env = baseEnv(dir)
	env[consts.InputGithubToken] = consts.Empty
	loadEnvExpectError(t, env, "expected missing token error")
}

// TestLoadFromEnvGitHubTokenFallback verifies GITHUB_TOKEN is used when the input token is empty.
func TestLoadFromEnvGitHubTokenFallback(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputGithubToken] = consts.Empty
	env["GITHUB_TOKEN"] = testFallbackToken
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	if cfg.GitHubToken != testFallbackToken {
		t.Fatalf("GitHubToken = %q, want fallback-token", cfg.GitHubToken)
	}
}

// TestLoadFromEnvDockerInputEnvNames verifies hyphenated Docker action input env names are read.
func TestLoadFromEnvDockerInputEnvNames(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputGithubToken] = consts.Empty
	env["INPUT_GITHUB-TOKEN"] = testDockerToken
	env[consts.InputJS] = "runtime: nodejs\npackage-manager: pnpm\nversion-manager: fnm\n"
	env["INPUT_INCLUDES-DOC"] = testFalseValue
	env["INPUT_SYNC-ROOT"] = testFalseValue
	env["INPUT_TARGET-FOLDER"] = testCustomTaskfiles

	assertDockerInputs(t, loadEnvOK(t, env))
}

func assertDockerInputs(t *testing.T, cfg *config.Config) {
	t.Helper()

	assertDockerAuth(t, cfg)
	assertDockerJSSettings(t, cfg)
}

func assertDockerAuth(t *testing.T, cfg *config.Config) {
	t.Helper()

	if cfg.GitHubToken != testDockerToken {
		t.Fatalf("GitHubToken = %q, want docker-token", cfg.GitHubToken)
	}

	if cfg.TargetFolder != testCustomTaskfiles {
		t.Fatalf("TargetFolder = %q, want custom/taskfiles", cfg.TargetFolder)
	}
}

func assertDockerJSSettings(t *testing.T, cfg *config.Config) {
	t.Helper()

	if cfg.NodePackageManager != config.PMPnpm {
		t.Fatalf("NodePackageManager = %q, want pnpm", cfg.NodePackageManager)
	}

	if cfg.NodeVersionManager != config.VMFnm {
		t.Fatalf(fmtWantFnm, cfg.NodeVersionManager)
	}

	if cfg.IncludesDoc {
		t.Fatal("IncludesDoc = true, want false")
	}

	if cfg.SyncRoot {
		t.Fatal("SyncRoot = true, want false")
	}
}

// TestParseTasksMultilineAndDedupe verifies multiline, comma-separated task input is deduped.
func TestParseTasksMultilineAndDedupe(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputTasks] = "eslint\nprettier,\ngo\ngo\n"

	cfg := loadEnvOK(t, env)

	want := []string{testTaskEslint, "prettier", consts.Go}
	assertTasksEqual(t, cfg.Tasks, want)
}

func assertTasksEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf(fmtTasksWant, got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf(fmtTasksWant, got, want)
		}
	}
}

// TestInvalidTaskName verifies an unsafe task name is rejected.
func TestInvalidTaskName(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputTasks] = "../evil"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for unsafe task name")
	}
}

// TestBunWithVersionManagerRejected verifies bun runtime with a version-manager fails validation.
func TestBunWithVersionManagerRejected(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputTasks] = testTaskEslint
	env[consts.InputJS] = "runtime: bun\nversion-manager: fnm\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected bun+version-manager validation error")
	}
}

// TestInvalidPackageManager verifies an unrecognized package manager is rejected.
func TestInvalidPackageManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: nodejs\npackage-manager: cargo\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid package manager error")
	}
}

// TestInvalidIncludesDoc verifies a non-boolean includes-doc input is rejected.
func TestInvalidIncludesDoc(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_INCLUDES_DOC"] = testInvalidBool
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid includes-doc error")
	}
}

// TestInvalidSyncRoot verifies a non-boolean sync-root input is rejected.
func TestInvalidSyncRoot(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_SYNC_ROOT"] = testInvalidBool
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid sync-root error")
	}
}

// TestFailOnChangesDefaultsFalse verifies fail-on-changes defaults to false when unset.
func TestFailOnChangesDefaultsFalse(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, baseEnv(dir))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	if cfg.FailOnChanges {
		t.Fatal("FailOnChanges should default to false")
	}
}

// TestFailOnChangesTrue verifies fail-on-changes is set to true when the input is "true".
func TestFailOnChangesTrue(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_FAIL-ON-CHANGES"] = "true"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	if !cfg.FailOnChanges {
		t.Fatal("FailOnChanges = false, want true")
	}
}

// TestInvalidFailOnChanges verifies a non-boolean fail-on-changes input is rejected.
func TestInvalidFailOnChanges(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_FAIL-ON-CHANGES"] = testInvalidBool
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid fail-on-changes error")
	}
}

// TestUnsafeStoreVersion verifies an unsafe store-version value is rejected.
func TestUnsafeStoreVersion(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputStoreVersion] = "refs/heads/main"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected unsafe store-version error")
	}
}

// TestStoreVersionAllowsSafeTag verifies a safe tag store-version value is accepted.
func TestStoreVersionAllowsSafeTag(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputStoreVersion] = testStoreVersionTag
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.StoreVersion != testStoreVersionTag {
		t.Fatalf("StoreVersion = %q", cfg.StoreVersion)
	}
}

// TestTargetFolderValidation verifies target-folder values are accepted or rejected as expected.
func TestTargetFolderValidation(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{"default nested", "automation/taskfiles", true},
		{"absolute unix", "/taskfiles", false},
		{"windows absolute", `C:\taskfiles`, false},
		{"dot", "../taskfiles", false},
		{"dot git", ".git", false},
	}

	for i := range cases {
		testCase := &cases[i]
		t.Run(testCase.name, func(t *testing.T) {
			env := baseEnv(dir)

			env[consts.InputTargetFolder] = testCase.value

			assertTargetFolderResult(t, env, testCase.ok)
		})
	}
}

func assertTargetFolderResult(t *testing.T, env map[string]string, wantOK bool) {
	t.Helper()

	setEnv(t, env)

	_, err := config.LoadFromEnv()

	if wantOK && err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wantOK && err == nil {
		t.Fatal(consts.ExpectedValidErr)
	}
}

// TestTargetFolderSymlinkEscape verifies a target folder escaping via symlink is rejected.
func TestTargetFolderSymlinkEscape(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(workspace, "link-out")

	err := os.Symlink(outside, link)
	if err != nil {
		t.Skip("symlink not permitted")
	}

	env := baseEnv(workspace)

	env[consts.InputTargetFolder] = "link-out/taskfiles"

	loadEnvExpectError(t, env, "expected symlink escape rejection")
}
