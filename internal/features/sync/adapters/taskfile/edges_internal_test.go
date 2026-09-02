// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	yaml "go.yaml.in/yaml/v3"
)

const (
	pnpmModule   = "pnpm"
	pnpmInclude  = "../../../pnpm/Taskfile.yml"
	absolutePath = "/abs/dest"
	varKeyName   = "GO_VERSION"
)

// TestRewriteIncludesRejectsLiteralBlockPath verifies a literal block include path fails.
func TestRewriteIncludesRejectsLiteralBlockPath(t *testing.T) {
	t.Parallel()

	content := []byte(yamlHeader + "includes:\n  pnpm:\n    taskfile: |-\n      " +
		pnpmInclude + "\n")

	out, err := RewriteIncludes(content, pnpmMapping(), fromDestESLint)
	iox.Discard(out)

	if err == nil {
		t.Fatal("expected span resolution failure")
	}
}

// TestRewriteIncludesRewritesQuotedPath verifies a quoted include path is rewritten in place.
func TestRewriteIncludesRewritesQuotedPath(t *testing.T) {
	t.Parallel()

	content := []byte(yamlHeader + "includes:\n  pnpm:\n    taskfile: \"" + pnpmInclude + "\"\n")

	out, err := RewriteIncludes(content, pnpmMapping(), fromDestESLint)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if !strings.Contains(string(out), "\"../pnpm/Taskfile.yml\"") {
		t.Fatalf("quoted path not rewritten: %s", out)
	}
}

// TestRewriteIncludesSkipsNonMappingIncludes verifies unusable include sections are left alone.
func TestRewriteIncludesSkipsNonMappingIncludes(t *testing.T) {
	t.Parallel()

	assertRewriteNoop(t, []byte(yamlHeader+"includes: []\n"))
	assertRewriteNoop(t, []byte(yamlHeader+"includes:\n  pnpm: "+pnpmInclude+"\n"))
}

// TestRewriteIncludesHandlesDotSlashPrefix verifies "./" prefixed paths are rewritten.
func TestRewriteIncludesHandlesDotSlashPrefix(t *testing.T) {
	t.Parallel()

	content := []byte(yamlHeader + "includes:\n  pnpm:\n    taskfile: ./pnpm/Taskfile.yml\n")

	out, err := RewriteIncludes(content, map[string]string{pnpmModule: pnpmModule}, fromDestESLint)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	iox.Discard(out)
}

// TestRelativePathHelpersFallBackOnFailure verifies absolute/relative mismatches fall back.
func TestRelativePathHelpersFallBackOnFailure(t *testing.T) {
	t.Parallel()

	if got := destinationIncludePath("relative", absolutePath, pnpmInclude); got != pnpmInclude {
		t.Fatalf(fmtDestInclude, got)
	}

	if got := moduleIncludePath(absolutePath, folderTaskfile, "go"); got != goDest {
		t.Fatalf(fmtModulePath, got)
	}

	if got := includeDirForRoot(absolutePath); got != consts.PathDot {
		t.Fatalf(fmtIncludeDir, got)
	}
}

// TestUpdateRootTaskfileRejectsNonMappingTasks verifies a non-mapping tasks section fails.
func TestUpdateRootTaskfileRejectsNonMappingTasks(t *testing.T) {
	t.Parallel()

	input := goRootInput()

	input.GeneratedTasks = []rootupd.GeneratedRootTask{{Name: lintTask, Modules: []string{goTask}}}

	out, err := UpdateRootTaskfile([]byte(yamlHeader+"tasks: []\n"), input)
	iox.Discard(out)

	if err == nil {
		t.Fatal("expected tasks mapping failure")
	}
}

// TestUpdateRootTaskfileKeepsStillGeneratedTasks verifies regenerated tasks are not pruned.
func TestUpdateRootTaskfileKeepsStillGeneratedTasks(t *testing.T) {
	t.Parallel()

	input := goRootInput()

	input.GeneratedTasks = []rootupd.GeneratedRootTask{{Name: lintTask, Modules: []string{goTask}}}
	input.ManagedRootTasks = []string{lintTask}

	out, err := UpdateRootTaskfile(NewRootTemplate(), input)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if !strings.Contains(string(out), "lint:") {
		t.Fatalf("generated task missing: %s", out)
	}
}

// TestIsManagedIncludeFallsBackToManagedTasks verifies aliases without a taskfile key.
func TestIsManagedIncludeFallsBackToManagedTasks(t *testing.T) {
	t.Parallel()

	entry := newYAMLMappingNode()
	appendMappingPair(entry, yamlScalar(keyDir), yamlScalar(consts.PathDot))

	managed := &managedIncludeParams{
		entry:        entry,
		expectedPath: goDest,
		task:         goTask,
		managedTasks: []string{goTask},
	}

	if !isManagedInclude(managed) {
		t.Fatal("alias listed in managed tasks should be managed")
	}
}

// TestPromotedVarHelpersHandleMissingValues verifies missing module vars are skipped.
func TestPromotedVarHelpersHandleMissingValues(t *testing.T) {
	t.Parallel()

	if varValueIn(nil, varKeyName) != nil {
		t.Fatal("nil vars node should yield no value")
	}

	if varValueIn(yamlScalar(plainText), varKeyName) != nil {
		t.Fatal("scalar vars node should yield no value")
	}

	if firstVarValue([]string{goTask}, map[string]*yaml.Node{}, varKeyName) != nil {
		t.Fatal("missing module vars should yield no value")
	}
}

// TestAddMissingPromotedVarSkipsUnknownKey verifies a key without a value is not added.
func TestAddMissingPromotedVarSkipsUnknownKey(t *testing.T) {
	t.Parallel()

	rootVars := newYAMLMappingNode()

	addMissingPromotedVar(&addPromotedVarParams{
		rootVars:   rootVars,
		moduleVars: map[string]*yaml.Node{},
		existing:   map[string]struct{}{},
		key:        varKeyName,
		tasks:      []string{goTask},
	})

	if len(rootVars.Content) != consts.IndexZero {
		t.Fatalf("root vars = %#v", rootVars.Content)
	}
}

// TestMergeIncludeVarsSkipsNonMappingNodes verifies non-mapping vars are left untouched.
func TestMergeIncludeVarsSkipsNonMappingNodes(t *testing.T) {
	t.Parallel()

	entry := newYAMLMappingNode()
	mergeIncludeVars(entry, yamlScalar(plainText))

	if len(entry.Content) != consts.IndexZero {
		t.Fatalf("entry = %#v", entry.Content)
	}

	appendMappingPair(entry, yamlScalar(keyVars), yamlScalar(plainText))
	mergeIncludeVars(entry, moduleVarsNode())

	if entry.Content[consts.IndexOne].Value != plainText {
		t.Fatalf("entry vars = %#v", entry.Content[consts.IndexOne])
	}
}

func assertRewriteNoop(t *testing.T, content []byte) {
	t.Helper()

	out, err := RewriteIncludes(content, pnpmMapping(), fromDestESLint)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if !bytes.Equal(out, content) {
		t.Fatalf("content changed: %s", out)
	}
}

func moduleVarsNode() *yaml.Node {
	vars := newYAMLMappingNode()
	appendMappingPair(vars, yamlScalar(varKeyName), yamlScalar("1.22"))

	return vars
}

func pnpmMapping() map[string]string {
	return map[string]string{pnpmModule: pnpmModule}
}
