// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile_test

import (
	"os"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/taskfile"
)

const (
	targetFolderTaskfiles = "taskfiles"
	taskESLint            = "eslint"
	storeESLintTaskfile   = "../../tests/fixtures/store/taskfiles/eslint/node/fnm/pnpm/Taskfile.yml"

	wantVersion35           = `version: "3.5"`
	fmtWantVersion35        = "expected root Taskfile version 3.5: %s"
	pathTaskfilesGoYML      = "taskfiles/go/Taskfile.yml"
	fmtMissingGoInclude     = "missing go include: %s"
	fmtMissingIncludeVars   = "missing include vars: %s"
	varGoCmdUnix            = "GO_CMD_UNIX"
	destPnpm                = "pnpm"
	srcPnpmFnm              = "pnpm/fnm"
	wantPnpmRewrite         = "../../../../pnpm/Taskfile.yml"
	moduleInternalSkipfiles = "internal/skipfiles"
	moduleJQ                = "jq"
)

func rootUpdateInput(input taskfile.RootUpdateInput) taskfile.RootUpdateInput {
	input.GeneratedTasks = append([]taskfile.GeneratedRootTask{}, input.GeneratedTasks...)
	input.ManagedRootTasks = append([]string{}, input.ManagedRootTasks...)

	return input
}

func assertRewriteIncludesOutput(t *testing.T, text string) {
	t.Helper()

	if !strings.Contains(text, wantPnpmRewrite) {
		t.Fatalf("include not rewritten: %s", text)
	}

	if !strings.Contains(text, "../../../../pnpm/fnm/Taskfile.yml") {
		t.Fatalf("command string should remain unchanged: %s", text)
	}
}

// TestRewriteIncludes verifies include taskfile paths are rewritten to destination paths.
func TestRewriteIncludes(t *testing.T) {
	t.Parallel()

	input := []byte(`version: "3"
includes:
  pnpm:
    taskfile: ../../../../pnpm/fnm/Taskfile.yml
tasks:
  lint:
    cmds:
      - echo ../../../../pnpm/fnm/Taskfile.yml
`)
	sourceToDest := map[string]string{
		srcPnpmFnm: destPnpm,
	}

	out, err := taskfile.RewriteIncludes(input, sourceToDest)
	if err != nil {
		t.Fatal(err)
	}

	assertRewriteIncludesOutput(t, string(out))
}

// TestRewriteIncludesNamespacedModule verifies namespaced and unmapped includes rewrite correctly.
func TestRewriteIncludesNamespacedModule(t *testing.T) {
	t.Parallel()

	input := []byte(`version: "3"
includes:
  skipfiles:
    taskfile: ../internal/skipfiles/Taskfile.yml
  bun:
    taskfile: ../bun-latest/Taskfile.yml
  sibling:
    taskfile: ../../jq/Taskfile.yml
  unmapped:
    taskfile: ../unknown/Taskfile.yml
`)
	sourceToDest := map[string]string{
		moduleInternalSkipfiles: moduleInternalSkipfiles,
		"bun-latest":            "bun",
		moduleJQ:                moduleJQ,
	}

	out, err := taskfile.RewriteIncludes(input, sourceToDest)
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)

	for _, want := range []string{
		"../internal/skipfiles/Taskfile.yml",
		"../bun/Taskfile.yml",
		"../../jq/Taskfile.yml",
		"../unknown/Taskfile.yml",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in rewritten Taskfile: %s", want, text)
		}
	}
}

func assertTemplateOutput(t *testing.T, text string) {
	t.Helper()

	if !strings.Contains(text, wantVersion35) {
		t.Fatalf(fmtWantVersion35, text)
	}

	if !strings.Contains(text, pathTaskfilesGoYML) {
		t.Fatalf(fmtMissingGoInclude, text)
	}

	if !strings.Contains(text, "GO_VERSION") {
		t.Fatalf(fmtMissingIncludeVars, text)
	}

	if !strings.Contains(text, varGoCmdUnix) {
		t.Fatalf(fmtMissingIncludeVars, text)
	}
}

// TestUpdateRootTaskfileFromTemplate verifies a fresh template gains the expected includes and vars.
func TestUpdateRootTaskfileFromTemplate(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(
		taskfile.NewRootTemplate(),
		rootUpdateInput(taskfile.RootUpdateInput{
			Tasks:           []string{consts.Go},
			TargetFolder:    targetFolderTaskfiles,
			RootTaskfileDir: consts.Empty,
			DestByTask:      map[string]string{consts.Go: consts.Go},
			ManagedTasks:    nil,
			ModuleTaskfiles: map[string][]byte{
				consts.Go: []byte(`version: "3"
vars:
  GO_VERSION: ""
  GO_CMD_UNIX: /usr/local/go/bin/go
`),
			},
			GeneratedTasks:   nil,
			ManagedRootTasks: nil,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	assertTemplateOutput(t, string(out))
}

// TestUpdateRootTaskfileFolderRelativeIncludes verifies includes are folder-relative when nested.
func TestUpdateRootTaskfileFolderRelativeIncludes(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(
		taskfile.NewRootTemplate(),
		rootUpdateInput(taskfile.RootUpdateInput{
			Tasks:            []string{consts.Go},
			TargetFolder:     targetFolderTaskfiles,
			RootTaskfileDir:  targetFolderTaskfiles,
			DestByTask:       map[string]string{consts.Go: consts.Go},
			ManagedTasks:     nil,
			ModuleTaskfiles:  nil,
			GeneratedTasks:   nil,
			ManagedRootTasks: nil,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)

	if !strings.Contains(text, "taskfile: go/Taskfile.yml") {
		t.Fatalf("expected folder-relative include, got: %s", text)
	}

	if strings.Contains(text, pathTaskfilesGoYML) {
		t.Fatalf("include should not repeat the target folder: %s", text)
	}
}

// TestUpdateRootTaskfileUpdatesVersion verifies the root Taskfile version is updated to the current version.
func TestUpdateRootTaskfileUpdatesVersion(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
includes:
  custom:
    taskfile: custom/Taskfile.yml
`)

	out, err := taskfile.UpdateRootTaskfile(root, rootUpdateInput(taskfile.RootUpdateInput{
		Tasks:            []string{consts.Go},
		TargetFolder:     targetFolderTaskfiles,
		RootTaskfileDir:  "",
		DestByTask:       map[string]string{consts.Go: consts.Go},
		ManagedTasks:     nil,
		ModuleTaskfiles:  nil,
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	}))
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)

	if !strings.Contains(text, wantVersion35) {
		t.Fatalf(fmtWantVersion35, text)
	}
}

// TestUpdateRootTaskfilePreservesExistingIncludeVars verifies user-set include vars survive an update.
func TestUpdateRootTaskfilePreservesExistingIncludeVars(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
includes:
  go:
    taskfile: taskfiles/go/Taskfile.yml
    vars:
      GO_VERSION: go1.22.0
`)
	module := []byte(`version: "3"
vars:
  GO_VERSION: ""
  GO_CMD_UNIX: /usr/local/go/bin/go
`)

	out, err := taskfile.UpdateRootTaskfile(root, rootUpdateInput(taskfile.RootUpdateInput{
		Tasks:           []string{consts.Go},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: consts.Empty,
		DestByTask:      map[string]string{consts.Go: consts.Go},
		ManagedTasks:    []string{consts.Go},
		ModuleTaskfiles: map[string][]byte{
			consts.Go: module,
		},
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	}))
	if err != nil {
		t.Fatal(err)
	}

	assertPreservedIncludeVars(t, string(out))
}

func assertPreservedIncludeVars(t *testing.T, text string) {
	t.Helper()

	if !strings.Contains(text, "go1.22.0") {
		t.Fatalf("existing include var override removed: %s", text)
	}

	if !strings.Contains(text, varGoCmdUnix) {
		t.Fatalf("missing newly added module var: %s", text)
	}
}

// TestUpdateRootTaskfile verifies managed includes are added while user includes are preserved.
func TestUpdateRootTaskfile(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
includes:
  custom:
    taskfile: custom/Taskfile.yml
tasks:
  hello:
    cmds:
      - echo hi
`)

	out, err := taskfile.UpdateRootTaskfile(root, rootUpdateInput(taskfile.RootUpdateInput{
		Tasks:            []string{consts.Go, taskESLint},
		TargetFolder:     targetFolderTaskfiles,
		RootTaskfileDir:  "",
		DestByTask:       map[string]string{consts.Go: consts.Go, taskESLint: taskESLint},
		ManagedTasks:     []string{},
		ModuleTaskfiles:  nil,
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	}))
	if err != nil {
		t.Fatal(err)
	}

	assertUpdateRootTaskfileOutput(t, string(out))
}

func assertUpdateRootTaskfileOutput(t *testing.T, text string) {
	t.Helper()

	if !strings.Contains(text, pathTaskfilesGoYML) {
		t.Fatalf(fmtMissingGoInclude, text)
	}

	if !strings.Contains(text, "taskfiles/eslint/Taskfile.yml") {
		t.Fatalf("missing eslint include: %s", text)
	}

	if !strings.Contains(text, "custom/Taskfile.yml") {
		t.Fatalf("user include removed: %s", text)
	}
}

// TestUpdateRootTaskfileGeneratedTasksAndSharedVars verifies generated tasks and shared vars are merged in.
func TestUpdateRootTaskfileGeneratedTasksAndSharedVars(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(
		sharedVarsRootFixture(),
		rootUpdateInput(taskfile.RootUpdateInput{
			Tasks:           []string{consts.Go, taskESLint},
			TargetFolder:    targetFolderTaskfiles,
			RootTaskfileDir: consts.Empty,
			DestByTask:      map[string]string{consts.Go: consts.Go, taskESLint: taskESLint},
			ManagedTasks:    []string{consts.Go, taskESLint},
			ModuleTaskfiles: sharedVarsModuleTaskfiles(),
			GeneratedTasks: []taskfile.GeneratedRootTask{
				{Name: "lint", Modules: []string{consts.Go, taskESLint}},
			},
			ManagedRootTasks: []string{"test"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)

	assertContainsAll(t, text, []string{
		"VERSION: 1.2.3",
		"VERSION: '{{.VERSION}}'",
		"GO_VERSION: \"\"",
		"CONFIG: \"\"",
		"task: go:lint",
		"task: eslint:lint",
		"echo custom",
	})

	assertContainsNone(t, text, []string{"echo user lint", "previously generated"})
}

func assertContainsAll(t *testing.T, text string, wants []string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in root Taskfile:\n%s", want, text)
		}
	}
}

func assertContainsNone(t *testing.T, text string, notWants []string) {
	t.Helper()

	for _, notWant := range notWants {
		if strings.Contains(text, notWant) {
			t.Fatalf("did not expect %q in root Taskfile:\n%s", notWant, text)
		}
	}
}

func sharedVarsRootFixture() []byte {
	return []byte(`version: "3"
vars:
  VERSION: 1.2.3
includes:
  custom:
    taskfile: custom/Taskfile.yml
tasks:
  lint:
    cmds:
      - echo user lint
  test:
    cmds:
      - echo previously generated
  custom:
    cmds:
      - echo custom
`)
}

func sharedVarsModuleTaskfiles() map[string][]byte {
	return map[string][]byte{
		consts.Go: []byte(`version: "3"
vars:
  VERSION: ""
  GO_VERSION: ""
`),
		taskESLint: []byte(`version: "3"
vars:
  VERSION: latest
  CONFIG: ""
`),
	}
}

// TestManagedIncludeDifferentPathConflict verifies a managed alias with a mismatched path errors.
func TestManagedIncludeDifferentPathConflict(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
includes:
  eslint:
    taskfile: custom/eslint/Taskfile.yml
tasks:
  hello:
    cmds:
      - echo hi
`)

	_, err := taskfile.UpdateRootTaskfile(root, rootUpdateInput(taskfile.RootUpdateInput{
		Tasks:            []string{taskESLint},
		TargetFolder:     targetFolderTaskfiles,
		RootTaskfileDir:  "",
		DestByTask:       map[string]string{taskESLint: taskESLint},
		ManagedTasks:     []string{taskESLint},
		ModuleTaskfiles:  nil,
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	}))
	if err == nil {
		t.Fatal("expected conflict when alias path differs from managed path")
	}
}

// TestRootTaskfileAliasConflict verifies a conflicting legacy alias path returns an error.
func TestRootTaskfileAliasConflict(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
includes:
  go:
    taskfile: legacy/go/Taskfile.yml
`)

	_, err := taskfile.UpdateRootTaskfile(root, rootUpdateInput(taskfile.RootUpdateInput{
		Tasks:            []string{consts.Go},
		TargetFolder:     targetFolderTaskfiles,
		RootTaskfileDir:  "",
		DestByTask:       map[string]string{consts.Go: consts.Go},
		ManagedTasks:     []string{},
		ModuleTaskfiles:  nil,
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	}))
	if err == nil {
		t.Fatal("expected alias conflict")
	}
}

// TestScalarIncludeWrongPathConflict verifies a scalar include with the wrong path returns an error.
func TestScalarIncludeWrongPathConflict(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
includes:
  go: legacy/go/Taskfile.yml
`)

	_, err := taskfile.UpdateRootTaskfile(root, rootUpdateInput(taskfile.RootUpdateInput{
		Tasks:            []string{consts.Go},
		TargetFolder:     targetFolderTaskfiles,
		RootTaskfileDir:  "",
		DestByTask:       map[string]string{consts.Go: consts.Go},
		ManagedTasks:     []string{},
		ModuleTaskfiles:  nil,
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	}))
	if err == nil {
		t.Fatal("expected scalar include path conflict")
	}
}

// TestRewriteUsesRealStoreSnippet verifies rewriting works against a real store fixture Taskfile.
func TestRewriteUsesRealStoreSnippet(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(storeESLintTaskfile)
	if err != nil {
		t.Fatal(err)
	}

	out, err := taskfile.RewriteIncludes(data, map[string]string{
		srcPnpmFnm: destPnpm,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), wantPnpmRewrite) {
		t.Fatalf("rewrite failed: %s", out)
	}
}
