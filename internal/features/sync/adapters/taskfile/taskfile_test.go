// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile_test

import (
	"os"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	targetFolderTaskfiles = "taskfiles"
	taskESLint            = "eslint"
	storeESLintTaskfile   = "../../../../../tests/fixtures/store/taskfiles/eslint/node/fnm/pnpm/Taskfile.yml"

	wantVersion35           = `version: "3.5"`
	fmtWantVersion35        = "expected root Taskfile version 3.5: %s"
	pathTaskfilesGoYML      = "taskfiles/go/Taskfile.yml"
	fmtMissingGoInclude     = "missing go include: %s"
	destPnpm                = "pnpm"
	srcPnpmFnm              = "pnpm/fnm"
	wantPnpmRewrite         = "../pnpm/Taskfile.yml"
	moduleInternalSkipfiles = "internal/skipfiles"
	moduleJQ                = "jq"
	srcBunLatest            = "bun-latest"
	destBun                 = "bun"
	pathUnknownTaskfile     = "../unknown/Taskfile.yml"
	fmtExpectedInRewritten  = "expected %q in rewritten Taskfile: %s"
	pathTaskfileYML         = "Taskfile.yml"
	wantGoVersionEmpty      = `GO_VERSION: ""`
	wantGoCmdUnixDefault    = `GO_CMD_UNIX: /usr/local/go/bin/go`
	wantGoVersionRef        = `GO_VERSION: '{{.GO_VERSION}}'`
	wantGoCmdUnixRef        = `GO_CMD_UNIX: '{{.GO_CMD_UNIX}}'`
)

func rewriteIncludesInput() []byte {
	return []byte(`version: "3"
includes:
  pnpm:
    taskfile: ../../../../pnpm/fnm/Taskfile.yml
tasks:
  lint:
    cmds:
      - echo ../../../../pnpm/fnm/Taskfile.yml
`)
}

func rewriteNamespacedInput() []byte {
	return []byte(`version: "3"
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
}

func goModuleTaskfile() []byte {
	return []byte(`version: "3"
vars:
  GO_VERSION: ""
  GO_CMD_UNIX: /usr/local/go/bin/go
`)
}

func rootWithCustomInclude() []byte {
	return []byte(`version: "3"
includes:
  custom:
    taskfile: custom/Taskfile.yml
`)
}

func rootWithGoInclude() []byte {
	return []byte(`version: "3"
includes:
  go:
    taskfile: taskfiles/go/Taskfile.yml
    vars:
      GO_VERSION: go1.22.0
`)
}

func rootWithHelloTask() []byte {
	return []byte(`version: "3"
includes:
  custom:
    taskfile: custom/Taskfile.yml
tasks:
  hello:
    cmds:
      - echo hi
`)
}

func rootWithESLintConflict() []byte {
	return []byte(`version: "3"
includes:
  eslint:
    taskfile: custom/eslint/Taskfile.yml
tasks:
  hello:
    cmds:
      - echo hi
`)
}

func rootWithGoAliasConflict() []byte {
	return []byte(`version: "3"
includes:
  go:
    taskfile: legacy/go/Taskfile.yml
`)
}

func rootWithScalarGoConflict() []byte {
	return []byte(`version: "3"
includes:
  go: legacy/go/Taskfile.yml
`)
}

func rootUpdateInput(input *rootupd.RootUpdateInput) *rootupd.RootUpdateInput {
	input.GeneratedTasks = append([]rootupd.GeneratedRootTask{}, input.GeneratedTasks...)
	input.ManagedRootTasks = append([]string{}, input.ManagedRootTasks...)

	return input
}

func goOnlyRootInput() *rootupd.RootUpdateInput {
	return rootUpdateInput(&rootupd.RootUpdateInput{
		Tasks:            []string{consts.Go},
		TargetFolder:     targetFolderTaskfiles,
		RootTaskfileDir:  consts.Empty,
		DestByTask:       map[string]string{consts.Go: consts.Go},
		ManagedTasks:     nil,
		ModuleTaskfiles:  map[string][]byte{consts.Go: goModuleTaskfile()},
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	})
}

func rewriteNamespacedSourceToDest() map[string]string {
	return map[string]string{
		moduleInternalSkipfiles: moduleInternalSkipfiles,
		srcBunLatest:            destBun,
		moduleJQ:                moduleJQ,
	}
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

	out, err := taskfile.RewriteIncludes(
		rewriteIncludesInput(),
		map[string]string{srcPnpmFnm: destPnpm},
		taskESLint,
	)
	if err != nil {
		t.Fatal(err)
	}

	assertRewriteIncludesOutput(t, string(out))
}

// TestRewriteIncludesNamespacedModule verifies namespaced and unmapped includes rewrite correctly.
func TestRewriteIncludesNamespacedModule(t *testing.T) {
	t.Parallel()

	out, err := taskfile.RewriteIncludes(
		rewriteNamespacedInput(),
		rewriteNamespacedSourceToDest(),
		moduleInternalSkipfiles,
	)
	if err != nil {
		t.Fatal(err)
	}

	assertRewriteOutput(t, string(out), []string{
		"taskfile: " + pathTaskfileYML,
		"../../bun/Taskfile.yml",
		"../../jq/Taskfile.yml",
		pathUnknownTaskfile,
	})
}

// TestRewriteIncludesFlatDestination verifies sibling deps use a single ../ after normalize.
func TestRewriteIncludesFlatDestination(t *testing.T) {
	t.Parallel()

	out, err := taskfile.RewriteIncludes(
		rewriteNamespacedInput(),
		rewriteNamespacedSourceToDest(),
		taskESLint,
	)
	if err != nil {
		t.Fatal(err)
	}

	assertRewriteOutput(t, string(out), []string{
		"../internal/skipfiles/Taskfile.yml",
		"../bun/Taskfile.yml",
		"../jq/Taskfile.yml",
		pathUnknownTaskfile,
	})
}

func assertRewriteOutput(t *testing.T, text string, wants []string) {
	t.Helper()

	for i := range wants {
		if !strings.Contains(text, wants[i]) {
			t.Fatalf(fmtExpectedInRewritten, wants[i], text)
		}
	}
}

func assertTemplateOutput(t *testing.T, text string) {
	t.Helper()

	assertContainsAll(t, text, []string{
		wantVersion35,
		pathTaskfilesGoYML,
	})
	assertGoModulePromotedVars(t, text)
}

// TestUpdateRootTaskfileFromTemplate verifies a fresh template gains the expected includes and vars.
func TestUpdateRootTaskfileFromTemplate(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(taskfile.NewRootTemplate(), goOnlyRootInput())
	if err != nil {
		t.Fatal(err)
	}

	assertTemplateOutput(t, string(out))
}

// TestUpdateRootTaskfileFolderRelativeIncludes verifies includes are folder-relative when nested.
func TestUpdateRootTaskfileFolderRelativeIncludes(t *testing.T) {
	t.Parallel()

	input := goOnlyRootInput()

	input.RootTaskfileDir = targetFolderTaskfiles
	input.ModuleTaskfiles = nil

	out, err := taskfile.UpdateRootTaskfile(taskfile.NewRootTemplate(), input)
	if err != nil {
		t.Fatal(err)
	}

	assertFolderRelativeInclude(t, string(out))
}

func assertFolderRelativeInclude(t *testing.T, text string) {
	t.Helper()

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

	out, err := taskfile.UpdateRootTaskfile(rootWithCustomInclude(), goOnlyRootInput())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), wantVersion35) {
		t.Fatalf(fmtWantVersion35, out)
	}
}

// TestUpdateRootTaskfileForcesManagedIncludeVarsToRootRefs verifies include-level
// literals for managed module keys are rewritten to {{.KEY}} root references.
func TestUpdateRootTaskfileForcesManagedIncludeVarsToRootRefs(t *testing.T) {
	t.Parallel()

	input := goOnlyRootInput()

	input.ManagedTasks = []string{consts.Go}
	input.ModuleTaskfiles = map[string][]byte{consts.Go: goModuleTaskfile()}

	out, err := taskfile.UpdateRootTaskfile(rootWithGoInclude(), input)
	if err != nil {
		t.Fatal(err)
	}

	assertForcedIncludeRootRefs(t, string(out))
}

func assertForcedIncludeRootRefs(t *testing.T, text string) {
	t.Helper()

	assertGoModulePromotedVars(t, text)
	assertContainsNone(t, text, []string{"go1.22.0"})
}

func assertGoModulePromotedVars(t *testing.T, text string) {
	t.Helper()

	assertContainsAll(t, text, []string{
		wantGoVersionEmpty,
		wantGoCmdUnixDefault,
		wantGoVersionRef,
		wantGoCmdUnixRef,
	})
}

// TestUpdateRootTaskfile verifies managed includes are added while user includes are preserved.
func TestUpdateRootTaskfile(t *testing.T) {
	t.Parallel()

	input := rootUpdateInput(&rootupd.RootUpdateInput{
		Tasks:            []string{consts.Go, taskESLint},
		TargetFolder:     targetFolderTaskfiles,
		RootTaskfileDir:  consts.Empty,
		DestByTask:       map[string]string{consts.Go: consts.Go, taskESLint: taskESLint},
		ManagedTasks:     []string{},
		ModuleTaskfiles:  nil,
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	})

	out, err := taskfile.UpdateRootTaskfile(rootWithHelloTask(), input)
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

// TestUpdateRootTaskfileGeneratedTasksAndPromotedVars verifies generated tasks and
// all module vars are promoted to root with include {{.KEY}} references.
func TestUpdateRootTaskfileGeneratedTasksAndPromotedVars(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(promotedVarsRootFixture(), generatedTasksInput())
	if err != nil {
		t.Fatal(err)
	}

	assertGeneratedTasksAndPromotedVars(t, string(out))
}

func generatedTasksInput() *rootupd.RootUpdateInput {
	return rootUpdateInput(&rootupd.RootUpdateInput{
		Tasks:           []string{consts.Go, taskESLint},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: consts.Empty,
		DestByTask:      map[string]string{consts.Go: consts.Go, taskESLint: taskESLint},
		ManagedTasks:    []string{consts.Go, taskESLint},
		ModuleTaskfiles: promotedVarsModuleTaskfiles(),
		GeneratedTasks: []rootupd.GeneratedRootTask{
			{Name: "lint", Modules: []string{consts.Go, taskESLint}},
		},
		ManagedRootTasks: []string{"test"},
	})
}

func assertGeneratedTasksAndPromotedVars(t *testing.T, text string) {
	t.Helper()

	assertContainsAll(t, text, []string{
		"VERSION: 1.2.3",
		"VERSION: '{{.VERSION}}'",
		wantGoVersionEmpty,
		wantGoVersionRef,
		"CONFIG: \"\"",
		"CONFIG: '{{.CONFIG}}'",
		"task: go:lint",
		"task: eslint:lint",
		"echo custom",
	})

	assertContainsNone(t, text, []string{"echo user lint", "previously generated"})
}

func assertContainsAll(t *testing.T, text string, wants []string) {
	t.Helper()

	for i := range wants {
		if !strings.Contains(text, wants[i]) {
			t.Fatalf("expected %q in root Taskfile:\n%s", wants[i], text)
		}
	}
}

func assertContainsNone(t *testing.T, text string, notWants []string) {
	t.Helper()

	for i := range notWants {
		if strings.Contains(text, notWants[i]) {
			t.Fatalf("did not expect %q in root Taskfile:\n%s", notWants[i], text)
		}
	}
}

func promotedVarsRootFixture() []byte {
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

func promotedVarsModuleTaskfiles() map[string][]byte {
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

// TestUpdateRootTaskfilePromotesSingleModuleVars verifies a var present in only one
// module is still promoted to root and referenced from that include.
func TestUpdateRootTaskfilePromotesSingleModuleVars(t *testing.T) {
	t.Parallel()

	input := goOnlyRootInput()

	input.ManagedTasks = []string{consts.Go}

	out, err := taskfile.UpdateRootTaskfile(taskfile.NewRootTemplate(), input)
	if err != nil {
		t.Fatal(err)
	}

	assertGoModulePromotedVars(t, string(out))
}

// TestManagedIncludeDifferentPathConflict verifies a managed alias with a mismatched path errors.
func TestManagedIncludeDifferentPathConflict(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(
		rootWithESLintConflict(),
		rootUpdateInput(&rootupd.RootUpdateInput{
			Tasks:            []string{taskESLint},
			TargetFolder:     targetFolderTaskfiles,
			RootTaskfileDir:  consts.Empty,
			DestByTask:       map[string]string{taskESLint: taskESLint},
			ManagedTasks:     []string{taskESLint},
			ModuleTaskfiles:  nil,
			GeneratedTasks:   nil,
			ManagedRootTasks: nil,
		}),
	)
	iox.Discard(out)

	if err == nil {
		t.Fatal("expected conflict when alias path differs from managed path")
	}
}

// TestRootTaskfileAliasConflict verifies a conflicting legacy alias path returns an error.
func TestRootTaskfileAliasConflict(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(rootWithGoAliasConflict(), goOnlyRootInput())
	iox.Discard(out)

	if err == nil {
		t.Fatal("expected alias conflict")
	}
}

// TestScalarIncludeWrongPathConflict verifies a scalar include with the wrong path returns an error.
func TestScalarIncludeWrongPathConflict(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(rootWithScalarGoConflict(), goOnlyRootInput())
	iox.Discard(out)

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

	out, err := taskfile.RewriteIncludes(data, map[string]string{srcPnpmFnm: destPnpm}, taskESLint)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), wantPnpmRewrite) {
		t.Fatalf("rewrite failed: %s", out)
	}
}
