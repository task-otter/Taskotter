// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	targetFolderCase struct {
		name  string
		value string
		ok    bool
	}

	envValue struct {
		value   string
		present bool
	}
)

const (
	testFieldTasks = "tasks"

	testBaseBranch = "release/2026"

	testInvalidBool = "yes"

	testFalseValue = "false"

	testTrueValue = "true"

	testBuildTaskfile = "build/Taskfile.yml"

	testBadInputMsg = "bad input"

	testFallbackAuth = "fallback-auth"

	testDockerAuth = "docker-auth"

	testCustomTaskfiles = "custom/taskfiles"

	testTaskEslint = "eslint"

	testStoreVersionTag = "v1.2.3"

	fmtTasksWant = "Tasks = %#v, want %#v"

	fmtBaseBranchWant = "BaseBranch = %q, want %s"

	fmtJSRuntimeWantNodeJS = "JSRuntime = %q, want nodejs"

	testJSBunWithFnm = "runtime: bun\nversion-manager: fnm\n"

	inputIncludesDoc = "INPUT_INCLUDES_DOC"

	inputRootTaskfile = "INPUT_ROOT_TASKFILE"
)

var (
	testEnvMu      sync.Mutex
	testEnvStateMu sync.Mutex
	testEnvStates  = make(map[*testing.T]map[string]envValue)
)

func targetFolderCases() []targetFolderCase {
	return []targetFolderCase{
		{"default nested", "automation/taskfiles", true},
		{"absolute unix", "/taskfiles", false},
		{"windows absolute", `C:\taskfiles`, false},
		{"dot", "../taskfiles", false},
		{"dot git", ".git", false},
	}
}

// TestBunWithVersionManagerRejected verifies the removed version-manager key fails validation.
func TestBunWithVersionManagerRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputTasks] = testTaskEslint
	env[consts.InputJS] = testJSBunWithFnm
	loadEnvExpectError(t, env, "expected bun+version-manager validation error")
}

// TestFailOnChangesDefaultsFalse verifies fail-on-changes defaults to false when unset.
func TestFailOnChangesDefaultsFalse(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_FAIL-ON-CHANGES"] = testTrueValue
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
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_FAIL-ON-CHANGES"] = testInvalidBool
	loadEnvExpectError(t, env, "expected invalid fail-on-changes error")
}

// TestInvalidIncludesDoc verifies a non-boolean includes-doc input is rejected.
func TestInvalidIncludesDoc(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[inputIncludesDoc] = testInvalidBool
	loadEnvExpectError(t, env, "expected invalid includes-doc error")
}

// TestInvalidPackageManager verifies an unrecognized package manager is rejected.
func TestInvalidPackageManager(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: nodejs\npackage-manager: cargo\n"
	loadEnvExpectError(t, env, "expected invalid package manager error")
}

// TestInvalidSyncRoot verifies a non-boolean sync-root input is rejected.
func TestInvalidSyncRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env["INPUT_SYNC_ROOT"] = testInvalidBool
	loadEnvExpectError(t, env, "expected invalid sync-root error")
}

// TestInvalidTaskName verifies an unsafe task name is rejected.
func TestInvalidTaskName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputTasks] = "../evil"
	loadEnvExpectError(t, env, "expected error for unsafe task name")
}

// TestLoadFromEnvDefaults verifies default paths, includes-doc, sync-root, and tasks are set.
func TestLoadFromEnvDefaults(t *testing.T) {
	t.Parallel()

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

// TestIncludesDocFlipChangesConfigurationHash verifies flipping includes-doc changes hash and branch.
func TestIncludesDocFlipChangesConfigurationHash(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgTrue := loadIncludesDocConfig(t, dir, testTrueValue)
	cfgFalse := loadIncludesDocConfig(t, dir, testFalseValue)

	assertIncludesDocHashFlip(t, cfgTrue, cfgFalse)
}

func loadIncludesDocConfig(t *testing.T, dir, value string) *config.Config {
	t.Helper()

	env := baseEnv(dir)

	env[inputIncludesDoc] = value

	return loadEnvOK(t, env)
}

func assertIncludesDocHashFlip(t *testing.T, cfgTrue, cfgFalse *config.Config) {
	t.Helper()

	if cfgTrue.ConfigurationHash == cfgFalse.ConfigurationHash {
		t.Fatal("ConfigurationHash should change when includes_doc flips")
	}

	if cfgTrue.BranchName == cfgFalse.BranchName {
		t.Fatal("BranchName should change when includes_doc flips")
	}

	wantBranch := "taskotter/sync-" + cfgTrue.ConfigurationHash[:consts.HashPrefixLen]

	if cfgTrue.BranchName != wantBranch {
		t.Fatalf("BranchName = %q, want %q", cfgTrue.BranchName, wantBranch)
	}
}

// TestLoadFromEnvDockerInputEnvNames verifies hyphenated Docker action input env names are read.
func TestLoadFromEnvDockerInputEnvNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputGithubToken] = consts.Empty
	env["INPUT_GITHUB-TOKEN"] = testDockerAuth
	env[consts.InputJS] = "runtime: nodejs\npackage-manager: pnpm\n"
	env["INPUT_INCLUDES-DOC"] = testFalseValue
	env["INPUT_SYNC-ROOT"] = testFalseValue
	env["INPUT_TARGET-FOLDER"] = testCustomTaskfiles

	assertDockerInputs(t, loadEnvOK(t, env))
}

// TestLoadFromEnvGitHubTokenFallback verifies GITHUB_TOKEN is used when the input token is empty.
func TestLoadFromEnvGitHubTokenFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputGithubToken] = consts.Empty
	env["GITHUB_TOKEN"] = testFallbackAuth
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf(consts.FormatErr, err)
	}

	if cfg.GitHubToken != testFallbackAuth {
		t.Fatalf("GitHubToken = %q, want fallback-token", cfg.GitHubToken)
	}
}

// TestLoadFromEnvUsesPullRequestTargetAsPRBase verifies a pull request ref uses GITHUB_BASE_REF as the PR base.
func TestLoadFromEnvUsesPullRequestTargetAsPRBase(t *testing.T) {
	t.Parallel()

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

// TestLoadFromEnvUsesTriggerBranchAsPRBase verifies a push ref sets the PR base to the trigger branch.
func TestLoadFromEnvUsesTriggerBranchAsPRBase(t *testing.T) {
	t.Parallel()

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

// TestMissingRuntimeInputs verifies missing workspace or token inputs cause an error.
func TestMissingRuntimeInputs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	env := baseEnv(dir)

	env["GITHUB_WORKSPACE"] = consts.Empty
	loadEnvExpectError(t, env, "expected missing workspace error")

	env = baseEnv(dir)
	env[consts.InputGithubToken] = consts.Empty
	loadEnvExpectError(t, env, "expected missing token error")
}

// TestParseTasksMultilineAndDedupe verifies multiline, comma-separated task input is deduped.
func TestParseTasksMultilineAndDedupe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputTasks] = "eslint\nprettier,\ngo\ngo\n"

	cfg := loadEnvOK(t, env)

	want := []string{testTaskEslint, "prettier", consts.Go}
	assertTasksEqual(t, cfg.Tasks, want)
}

// TestRootTaskfileCustomPath verifies a custom root-taskfile input path is honored.
func TestRootTaskfileCustomPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[inputRootTaskfile] = testBuildTaskfile
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
	t.Parallel()

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

// TestEmptyTasksRejected verifies a blank tasks input fails validation.
func TestEmptyTasksRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputTasks] = consts.Empty
	loadEnvExpectError(t, env, "expected error for empty tasks")
}

// TestRootTaskfileMustStayInsideWorkspace verifies an escaping root-taskfile path is rejected.
func TestRootTaskfileMustStayInsideWorkspace(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[inputRootTaskfile] = "../outside/Taskfile.yml"
	loadEnvExpectError(t, env, "expected error for escaping root-taskfile path")
}

// TestRootTaskfileMustBeYAML verifies a non-YAML root-taskfile path is rejected.
func TestRootTaskfileMustBeYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[inputRootTaskfile] = "build/Taskfile.txt"
	loadEnvExpectError(t, env, "expected error for non-YAML root-taskfile path")
}

// TestStoreVersionAllowsSafeTag verifies a safe tag store-version value is accepted.
func TestStoreVersionAllowsSafeTag(t *testing.T) {
	t.Parallel()

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

// TestTargetFolderSymlinkEscape verifies a target folder escaping via symlink is rejected.
func TestTargetFolderSymlinkEscape(t *testing.T) {
	t.Parallel()

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

// TestTargetFolderValidation verifies target-folder values are accepted or rejected as expected.
func TestTargetFolderValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cases := targetFolderCases()

	for i := range cases {
		testCase := &cases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			runTargetFolderCase(t, dir, testCase)
		})
	}
}

// TestUnsafeStoreVersion verifies an unsafe store-version value is rejected.
func TestUnsafeStoreVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputStoreVersion] = "refs/heads/main"
	loadEnvExpectError(t, env, "expected unsafe store-version error")
}

// TestValidationErrorWithoutField verifies Error() returns just the message when Field is empty.
func TestValidationErrorWithoutField(t *testing.T) {
	t.Parallel()

	err := (&config.ValidationError{Field: consts.Empty, Message: testBadInputMsg}).Error()

	if err != testBadInputMsg {
		t.Fatalf("Error() = %q", err)
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

	if config.LockFilePath(cfg) != "taskfiles/.taskotter-lock.yml" {
		t.Fatalf("LockFilePath = %q, want taskfiles/.taskotter-lock.yml", config.LockFilePath(cfg))
	}

	if config.MetadataPath(cfg) != "taskfiles/.taskotter/metadata.yml" {
		t.Fatalf(
			"MetadataPath = %q, want taskfiles/.taskotter/metadata.yml",
			config.MetadataPath(cfg),
		)
	}
}

func assertDockerAuth(t *testing.T, cfg *config.Config) {
	t.Helper()

	if cfg.GitHubToken != testDockerAuth {
		t.Fatalf("GitHubToken = %q, want docker-token", cfg.GitHubToken)
	}

	if cfg.TargetFolder != testCustomTaskfiles {
		t.Fatalf("TargetFolder = %q, want custom/taskfiles", cfg.TargetFolder)
	}
}

func assertDockerInputs(t *testing.T, cfg *config.Config) {
	t.Helper()

	assertDockerAuth(t, cfg)
	assertDockerJSSettings(t, cfg)
}

func assertDockerJSSettings(t *testing.T, cfg *config.Config) {
	t.Helper()

	if cfg.NodePackageManager != config.PMPnpm {
		t.Fatalf("NodePackageManager = %q, want pnpm", cfg.NodePackageManager)
	}

	if cfg.IncludesDoc {
		t.Fatal("IncludesDoc = true, want false")
	}

	if cfg.SyncRoot {
		t.Fatal("SyncRoot = true, want false")
	}
}

func assertTargetFolderAccepted(t *testing.T, env map[string]string) {
	t.Helper()

	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	iox.Discard(cfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertTargetFolderRejected(t *testing.T, env map[string]string) {
	t.Helper()

	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	iox.Discard(cfg)

	if err == nil {
		t.Fatal(consts.ExpectedValidErr)
	}
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

func baseEnv(workspace string) map[string]string {
	return map[string]string{
		consts.InputTasks:        consts.Go,
		consts.InputJS:           consts.Empty,
		inputIncludesDoc:         consts.Empty,
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

func loadEnvExpectError(t *testing.T, env map[string]string, msg string) {
	t.Helper()

	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	iox.Discard(cfg)

	if err == nil {
		t.Fatal(msg)
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

func runTargetFolderCase(t *testing.T, dir string, testCase *targetFolderCase) {
	t.Helper()

	env := baseEnv(dir)

	env[consts.InputTargetFolder] = testCase.value

	if !testCase.ok {
		assertTargetFolderRejected(t, env)

		return
	}

	assertTargetFolderAccepted(t, env)
}

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()

	testEnvStateMu.Lock()

	state, exists := testEnvStates[t]
	testEnvStateMu.Unlock()

	if !exists {
		testEnvMu.Lock()

		state = make(map[string]envValue)

		testEnvStateMu.Lock()

		testEnvStates[t] = state
		testEnvStateMu.Unlock()

		t.Cleanup(func() {
			testEnvStateMu.Lock()
			delete(testEnvStates, t)
			testEnvStateMu.Unlock()

			for key, original := range state {
				if original.present {
					err := os.Setenv(key, original.value)
					if err != nil {
						t.Errorf("restore %s: %v", key, err)
					}

					continue
				}

				err := os.Unsetenv(key)
				if err != nil {
					t.Errorf("unset %s: %v", key, err)
				}
			}

			testEnvMu.Unlock()
		})
	}

	for k := range kv {
		testEnvStateMu.Lock()

		if _, recorded := state[k]; !recorded {
			value, present := os.LookupEnv(k)

			state[k] = envValue{value: value, present: present}
		}

		testEnvStateMu.Unlock()

		err := os.Setenv(k, kv[k])
		if err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
}

// TestValidationErrorFieldName covers FieldName.
func TestValidationErrorFieldName(t *testing.T) {
	t.Parallel()

	err := &config.ValidationError{Field: testFieldTasks, Message: "required"}

	if err.FieldName() != testFieldTasks {
		t.Fatalf("FieldName = %q", err.FieldName())
	}
}

// TestValidationErrorFieldNameEmptyMessage covers FieldName when Message is empty.
func TestValidationErrorFieldNameEmptyMessage(t *testing.T) {
	t.Parallel()

	err := &config.ValidationError{Field: testFieldTasks, Message: consts.Empty}

	if err.FieldName() != testFieldTasks {
		t.Fatalf("FieldName() = %q", err.FieldName())
	}
}

func jsEnv(dir, jsValue string) map[string]string {
	env := baseEnv(dir)

	env[consts.InputJS] = jsValue

	return env
}

// TestParseJSNodeJSDefaults verifies nodejs runtime defaults to the npm package manager.
func TestParseJSNodeJSDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := loadEnvOK(t, jsEnv(dir, "runtime: nodejs\n"))

	if cfg.JSRuntime != config.JSRuntimeNodeJS {
		t.Fatalf(fmtJSRuntimeWantNodeJS, cfg.JSRuntime)
	}

	if cfg.NodePackageManager != config.PMNPM {
		t.Fatalf("NodePackageManager = %q, want npm", cfg.NodePackageManager)
	}
}

// TestParseJSBun verifies bun runtime sets the bun package manager.
func TestParseJSBun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := loadEnvOK(t, jsEnv(dir, "runtime: bun\n"))

	if cfg.JSRuntime != config.JSRuntimeBun {
		t.Fatalf("JSRuntime = %q, want bun", cfg.JSRuntime)
	}

	if cfg.NodePackageManager != config.JSRuntimeBun {
		t.Fatalf("NodePackageManager = %q, want bun", cfg.NodePackageManager)
	}
}

// TestParseJSBunRejectsVersionManager verifies the removed version-manager key fails under bun.
func TestParseJSBunRejectsVersionManager(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = testJSBunWithFnm
	loadEnvExpectError(t, env, consts.ExpectedValidErr)
}

// TestParseJSBunRejectsPackageManager verifies bun runtime with an explicit package-manager fails.
func TestParseJSBunRejectsPackageManager(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: bun\npackage-manager: pnpm\n"
	loadEnvExpectError(t, env, consts.ExpectedValidErr)
}

// TestParseJSNodeJSRejectsBunPackageManager verifies nodejs runtime rejects the bun package manager.
func TestParseJSNodeJSRejectsBunPackageManager(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: nodejs\npackage-manager: bun\n"
	loadEnvExpectError(t, env, consts.ExpectedValidErr)
}

// TestParseJSEmpty verifies an empty js input leaves runtime and package manager unset.
func TestParseJSEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setEnv(t, baseEnv(dir))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JSRuntime != consts.Empty {
		t.Fatalf("JSRuntime = %q, want empty", cfg.JSRuntime)
	}

	if cfg.NodePackageManager != consts.Empty {
		t.Fatalf("NodePackageManager = %q, want empty", cfg.NodePackageManager)
	}
}

// TestParseJSDefaultsRuntimeToNodeJS verifies the runtime defaults to nodejs when unset.
func TestParseJSDefaultsRuntimeToNodeJS(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "package-manager: yarn\n"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JSRuntime != config.JSRuntimeNodeJS {
		t.Fatalf(fmtJSRuntimeWantNodeJS, cfg.JSRuntime)
	}
}

// TestParseJSRejectsInvalidYAML verifies malformed js YAML input is rejected.
func TestParseJSRejectsInvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = ":"
	loadEnvExpectError(t, env, "expected invalid YAML error")
}

// TestParseJSRejectsInvalidRuntime verifies an unrecognized runtime value is rejected.
func TestParseJSRejectsInvalidRuntime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: deno\n"
	loadEnvExpectError(t, env, "expected invalid runtime error")
}

// TestParseJSRejectsVersionManager verifies the removed version-manager key is rejected outright,
// including values that were valid before the store dropped its fnm and nvm variants.
func TestParseJSRejectsVersionManager(t *testing.T) {
	t.Parallel()

	removed := []string{"fnm", "nvm", "volta"}

	for i := range removed {
		dir := t.TempDir()
		env := baseEnv(dir)

		env[consts.InputJS] = "runtime: nodejs\nversion-manager: " + removed[i] + "\n"
		loadEnvExpectError(t, env, "expected removed version-manager error")
	}
}
