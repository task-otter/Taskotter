package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
)

const (
	testBaseBranch  = "release/2026"
	testInvalidBool = "yes"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()

	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func baseEnv(workspace string) map[string]string {
	return map[string]string{
		"INPUT_TASKS":         "go",
		"INPUT_JS":            "",
		"INPUT_INCLUDES_DOC":  "",
		"INPUT_SYNC_ROOT":     "",
		"INPUT_STORE_VERSION": "",
		"INPUT_TARGET_FOLDER": "",
		"INPUT_GITHUB_TOKEN":  "token",
		"GITHUB_WORKSPACE":    workspace,
		"GITHUB_REPOSITORY":   "owner/repo",
		"GITHUB_REF":          "",
		"GITHUB_BASE_REF":     "",
	}
}

func TestLoadFromEnvUsesTriggerBranchAsPRBase(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["GITHUB_REF"] = "refs/heads/" + testBaseBranch
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.BaseBranch != testBaseBranch {
		t.Fatalf("BaseBranch = %q, want %s", cfg.BaseBranch, testBaseBranch)
	}
}

func TestLoadFromEnvUsesPullRequestTargetAsPRBase(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["GITHUB_REF"] = "refs/pull/42/merge"
	env["GITHUB_BASE_REF"] = testBaseBranch
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.BaseBranch != testBaseBranch {
		t.Fatalf("BaseBranch = %q, want %s", cfg.BaseBranch, testBaseBranch)
	}
}

func TestLoadFromEnvDefaults(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, baseEnv(dir))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

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

	if !cfg.IncludesDoc {
		t.Fatal("IncludesDoc should default to true")
	}

	if !cfg.SyncRoot {
		t.Fatal("SyncRoot should default to true")
	}

	if len(cfg.Tasks) != 1 || cfg.Tasks[0] != "go" {
		t.Fatalf("Tasks = %#v", cfg.Tasks)
	}
}

func TestRootTaskfileCustomPath(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_ROOT_TASKFILE"] = "build/Taskfile.yml"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.RootTaskfile != "build/Taskfile.yml" {
		t.Fatalf("RootTaskfile = %q, want build/Taskfile.yml", cfg.RootTaskfile)
	}
}

func TestRootTaskfileFollowsTargetFolder(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_TARGET_FOLDER"] = "tools/tasks"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.RootTaskfile != "tools/tasks/Taskfile.yml" {
		t.Fatalf("RootTaskfile = %q, want tools/tasks/Taskfile.yml", cfg.RootTaskfile)
	}
}

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

func TestLoadFromEnvGitHubTokenFallback(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_GITHUB_TOKEN"] = ""
	env["GITHUB_TOKEN"] = "fallback-token"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.GitHubToken != "fallback-token" {
		t.Fatalf("GitHubToken = %q, want fallback-token", cfg.GitHubToken)
	}
}

func TestLoadFromEnvDockerInputEnvNames(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_GITHUB_TOKEN"] = ""
	env["INPUT_GITHUB-TOKEN"] = "docker-token"
	env["INPUT_JS"] = "runtime: nodejs\npackage-manager: pnpm\nversion-manager: fnm\n"
	env["INPUT_INCLUDES-DOC"] = "false"
	env["INPUT_SYNC-ROOT"] = "false"
	env["INPUT_TARGET-FOLDER"] = "custom/taskfiles"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.GitHubToken != "docker-token" {
		t.Fatalf("GitHubToken = %q, want docker-token", cfg.GitHubToken)
	}

	if cfg.NodePackageManager != config.PMPnpm {
		t.Fatalf("NodePackageManager = %q, want pnpm", cfg.NodePackageManager)
	}

	if cfg.NodeVersionManager != config.VMFnm {
		t.Fatalf("NodeVersionManager = %q, want fnm", cfg.NodeVersionManager)
	}

	if cfg.IncludesDoc {
		t.Fatal("IncludesDoc = true, want false")
	}

	if cfg.SyncRoot {
		t.Fatal("SyncRoot = true, want false")
	}

	if cfg.TargetFolder != "custom/taskfiles" {
		t.Fatalf("TargetFolder = %q, want custom/taskfiles", cfg.TargetFolder)
	}
}

func TestParseTasksMultilineAndDedupe(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_TASKS"] = "eslint\nprettier,\ngo\ngo\n"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	want := []string{"eslint", "prettier", "go"}
	if len(cfg.Tasks) != len(want) {
		t.Fatalf("Tasks = %#v, want %#v", cfg.Tasks, want)
	}

	for i := range want {
		if cfg.Tasks[i] != want[i] {
			t.Fatalf("Tasks = %#v, want %#v", cfg.Tasks, want)
		}
	}
}

func TestInvalidTaskName(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_TASKS"] = "../evil"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for unsafe task name")
	}
}

func TestBunWithVersionManagerRejected(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_TASKS"] = "eslint"
	env["INPUT_JS"] = "runtime: bun\nversion-manager: fnm\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected bun+version-manager validation error")
	}
}

func TestInvalidPackageManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_JS"] = "runtime: nodejs\npackage-manager: cargo\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid package manager error")
	}
}

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

func TestFailOnChangesDefaultsFalse(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, baseEnv(dir))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.FailOnChanges {
		t.Fatal("FailOnChanges should default to false")
	}
}

func TestFailOnChangesTrue(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_FAIL-ON-CHANGES"] = "true"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if !cfg.FailOnChanges {
		t.Fatal("FailOnChanges = false, want true")
	}
}

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

func TestUnsafeStoreVersion(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_STORE_VERSION"] = "refs/heads/main"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected unsafe store-version error")
	}
}

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
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := baseEnv(dir)
			env["INPUT_TARGET_FOLDER"] = testCase.value
			setEnv(t, env)

			_, err := config.LoadFromEnv()
			if testCase.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !testCase.ok && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestTargetFolderSymlinkEscape(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(workspace, "link-out")

	err := os.Symlink(outside, link)
	if err != nil {
		t.Skip("symlink not permitted")
	}

	env := baseEnv(workspace)
	env["INPUT_TARGET_FOLDER"] = "link-out/taskfiles"
	setEnv(t, env)

	_, err = config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}
