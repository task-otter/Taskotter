package taskfile_test

import (
	"os"
	"strings"
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/taskfile"
)

const (
	targetFolderTaskfiles = "taskfiles"
	taskESLint            = "eslint"
)

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
		"pnpm/fnm": "pnpm",
	}

	out, err := taskfile.RewriteIncludes(input, sourceToDest)
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)
	if !strings.Contains(text, "../../../../pnpm/Taskfile.yml") {
		t.Fatalf("include not rewritten: %s", text)
	}

	if !strings.Contains(text, "../../../../pnpm/fnm/Taskfile.yml") {
		t.Fatalf("command string should remain unchanged: %s", text)
	}
}

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
		"internal/skipfiles": "internal/skipfiles",
		"bun-latest":         "bun",
		"jq":                 "jq",
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

func TestUpdateRootTaskfileFromTemplate(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(taskfile.NewRootTemplate(), taskfile.RootUpdateInput{
		Tasks:           []string{"go"},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: "",
		DestByTask:      map[string]string{"go": "go"},
		ManagedTasks:    nil,
		ModuleTaskfiles: map[string][]byte{
			"go": []byte(`version: "3"
vars:
  GO_VERSION: ""
  GO_CMD_UNIX: /usr/local/go/bin/go
`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)
	if !strings.Contains(text, "taskfiles/go/Taskfile.yml") {
		t.Fatalf("missing go include: %s", text)
	}

	if !strings.Contains(text, "GO_VERSION") {
		t.Fatalf("missing include vars: %s", text)
	}

	if !strings.Contains(text, "GO_CMD_UNIX") {
		t.Fatalf("missing include vars: %s", text)
	}
}

func TestUpdateRootTaskfileFolderRelativeIncludes(t *testing.T) {
	t.Parallel()

	out, err := taskfile.UpdateRootTaskfile(taskfile.NewRootTemplate(), taskfile.RootUpdateInput{
		Tasks:           []string{"go"},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: targetFolderTaskfiles,
		DestByTask:      map[string]string{"go": "go"},
		ManagedTasks:    nil,
		ModuleTaskfiles: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)
	if !strings.Contains(text, "taskfile: go/Taskfile.yml") {
		t.Fatalf("expected folder-relative include, got: %s", text)
	}

	if strings.Contains(text, "taskfiles/go/Taskfile.yml") {
		t.Fatalf("include should not repeat the target folder: %s", text)
	}
}

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

	out, err := taskfile.UpdateRootTaskfile(root, taskfile.RootUpdateInput{
		Tasks:           []string{"go"},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: "",
		DestByTask:      map[string]string{"go": "go"},
		ManagedTasks:    []string{"go"},
		ModuleTaskfiles: map[string][]byte{
			"go": module,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)
	if !strings.Contains(text, "go1.22.0") {
		t.Fatalf("existing include var override removed: %s", text)
	}

	if !strings.Contains(text, "GO_CMD_UNIX") {
		t.Fatalf("missing newly added module var: %s", text)
	}
}

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

	out, err := taskfile.UpdateRootTaskfile(root, taskfile.RootUpdateInput{
		Tasks:           []string{"go", taskESLint},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: "",
		DestByTask:      map[string]string{"go": "go", taskESLint: taskESLint},
		ManagedTasks:    []string{},
		ModuleTaskfiles: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)
	if !strings.Contains(text, "taskfiles/go/Taskfile.yml") {
		t.Fatalf("missing go include: %s", text)
	}

	if !strings.Contains(text, "taskfiles/eslint/Taskfile.yml") {
		t.Fatalf("missing eslint include: %s", text)
	}

	if !strings.Contains(text, "custom/Taskfile.yml") {
		t.Fatalf("user include removed: %s", text)
	}
}

func TestUpdateRootTaskfileGeneratedTasksAndSharedVars(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
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
	out, err := taskfile.UpdateRootTaskfile(root, taskfile.RootUpdateInput{
		Tasks:           []string{"go", taskESLint},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: "",
		DestByTask:      map[string]string{"go": "go", taskESLint: taskESLint},
		ManagedTasks:    []string{"go", taskESLint},
		ModuleTaskfiles: map[string][]byte{
			"go": []byte(`version: "3"
vars:
  VERSION: ""
  GO_VERSION: ""
`),
			taskESLint: []byte(`version: "3"
vars:
  VERSION: latest
  CONFIG: ""
`),
		},
		GeneratedTasks: []taskfile.GeneratedRootTask{
			{Name: "lint", Modules: []string{"go", taskESLint}},
		},
		ManagedRootTasks: []string{"test"},
	})
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)
	for _, want := range []string{
		"VERSION: 1.2.3",
		"VERSION: '{{.VERSION}}'",
		"GO_VERSION: \"\"",
		"CONFIG: \"\"",
		"task: go:lint",
		"task: eslint:lint",
		"echo custom",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in root Taskfile:\n%s", want, text)
		}
	}

	for _, notWant := range []string{"echo user lint", "previously generated"} {
		if strings.Contains(text, notWant) {
			t.Fatalf("did not expect %q in root Taskfile:\n%s", notWant, text)
		}
	}
}

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

	_, err := taskfile.UpdateRootTaskfile(root, taskfile.RootUpdateInput{
		Tasks:           []string{taskESLint},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: "",
		DestByTask:      map[string]string{taskESLint: taskESLint},
		ManagedTasks:    []string{taskESLint},
		ModuleTaskfiles: nil,
	})
	if err == nil {
		t.Fatal("expected conflict when alias path differs from managed path")
	}
}

func TestRootTaskfileAliasConflict(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
includes:
  go:
    taskfile: legacy/go/Taskfile.yml
`)

	_, err := taskfile.UpdateRootTaskfile(root, taskfile.RootUpdateInput{
		Tasks:           []string{"go"},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: "",
		DestByTask:      map[string]string{"go": "go"},
		ManagedTasks:    []string{},
		ModuleTaskfiles: nil,
	})
	if err == nil {
		t.Fatal("expected alias conflict")
	}
}

func TestScalarIncludeWrongPathConflict(t *testing.T) {
	t.Parallel()

	root := []byte(`version: "3"
includes:
  go: legacy/go/Taskfile.yml
`)

	_, err := taskfile.UpdateRootTaskfile(root, taskfile.RootUpdateInput{
		Tasks:           []string{"go"},
		TargetFolder:    targetFolderTaskfiles,
		RootTaskfileDir: "",
		DestByTask:      map[string]string{"go": "go"},
		ManagedTasks:    []string{},
		ModuleTaskfiles: nil,
	})
	if err == nil {
		t.Fatal("expected scalar include path conflict")
	}
}

func TestRewriteUsesRealStoreSnippet(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../tests/fixtures/store/taskfiles/eslint/node/fnm/pnpm/Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}

	out, err := taskfile.RewriteIncludes(data, map[string]string{
		"pnpm/fnm": "pnpm",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), "../../../../pnpm/Taskfile.yml") {
		t.Fatalf("rewrite failed: %s", out)
	}
}
