// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	yaml "go.yaml.in/yaml/v3"
)

const (
	badYAML        = yamlHeader + "\tbad: [\n"
	goTask         = "go"
	goDest         = "taskfiles/go/Taskfile.yml"
	moduleWithVars = yamlHeader + "vars:\n  GO_VERSION: \"1.22\"\n"
	yamlHeader     = "version: \"3\"\n"
	fromDestESLint = "eslint"
	folderTaskfile = "taskfiles"
	valueText      = "value"
	plainText      = "plain"
	lintTask       = "lint"
	fmtDestInclude = "destinationIncludePath() = %q"
	fmtModulePath  = "moduleIncludePath() = %q"
	fmtIncludeDir  = "includeDirForRoot() = %q"
)

// TestOpsDelegatesToPackageHelpers verifies the ports adapter forwards to the helpers.
func TestOpsDelegatesToPackageHelpers(t *testing.T) {
	t.Parallel()

	ops := Ops{}

	if len(ops.NewRootTemplate()) == consts.IndexZero {
		t.Fatal("NewRootTemplate() = empty")
	}

	out, err := ops.RewriteIncludes(NewRootTemplate(), nil, consts.Empty)
	failIfErr(t, err)
	iox.Discard(out)

	out, err = ops.UpdateRootTaskfile(NewRootTemplate(), goRootInput())
	failIfErr(t, err)
	iox.Discard(out)
}

// TestOpsReportsFailures verifies malformed YAML surfaces through the ports adapter.
func TestOpsReportsFailures(t *testing.T) {
	t.Parallel()

	ops := Ops{}

	out, err := ops.RewriteIncludes([]byte(badYAML), nil, consts.Empty)
	iox.Discard(out)
	assertFails(t, err)

	out, err = ops.UpdateRootTaskfile([]byte(badYAML), goRootInput())
	iox.Discard(out)
	assertFails(t, err)
}

// TestRewriteIncludesRejectsMalformedYAML verifies parse and empty-document failures.
func TestRewriteIncludesRejectsMalformedYAML(t *testing.T) {
	t.Parallel()

	assertRewriteFails(t, []byte(badYAML))
	assertRewriteFails(t, []byte(consts.Empty))
}

// TestUpdateRootTaskfileRejectsMalformedYAML verifies parse and empty-document failures.
func TestUpdateRootTaskfileRejectsMalformedYAML(t *testing.T) {
	t.Parallel()

	assertRootUpdateFails(t, []byte(badYAML), goRootInput())
	assertRootUpdateFails(t, []byte(consts.Empty), goRootInput())
}

// TestUpdateRootTaskfileRejectsNonMappingSections verifies includes and vars must be mappings.
func TestUpdateRootTaskfileRejectsNonMappingSections(t *testing.T) {
	t.Parallel()

	assertRootUpdateFails(t, []byte(yamlHeader+"includes: []\n"), goRootInput())
	assertRootUpdateFails(t, []byte(yamlHeader+"vars: []\n"), goRootInput())
}

// TestUpdateRootTaskfileRejectsMissingDestination verifies a task without a destination fails.
func TestUpdateRootTaskfileRejectsMissingDestination(t *testing.T) {
	t.Parallel()

	input := goRootInput()

	input.DestByTask = map[string]string{}

	assertRootUpdateFails(t, NewRootTemplate(), input)
}

// TestUpdateRootTaskfileRejectsUnmanagedAlias verifies a foreign include alias fails.
func TestUpdateRootTaskfileRejectsUnmanagedAlias(t *testing.T) {
	t.Parallel()

	root := []byte(yamlHeader + "includes:\n  go:\n    taskfile: legacy/go/Taskfile.yml\n")
	assertRootUpdateFails(t, root, goRootInput())
}

// TestUpdateRootTaskfileRejectsMalformedModuleTaskfile verifies module parse failures surface.
func TestUpdateRootTaskfileRejectsMalformedModuleTaskfile(t *testing.T) {
	t.Parallel()

	input := goRootInput()

	input.ModuleTaskfiles = map[string][]byte{goTask: []byte(badYAML)}

	assertRootUpdateFails(t, NewRootTemplate(), input)
}

// TestUpdateRootTaskfileSkipsModulesWithoutVars verifies missing module vars are not fatal.
func TestUpdateRootTaskfileSkipsModulesWithoutVars(t *testing.T) {
	t.Parallel()

	cases := [][]byte{nil, []byte(yamlHeader), []byte(yamlHeader + "vars: {}\n")}

	for i := range cases {
		input := goRootInput()

		input.ModuleTaskfiles = map[string][]byte{goTask: cases[i]}

		out, err := UpdateRootTaskfile(NewRootTemplate(), input)
		if err != nil {
			t.Fatalf(consts.UnexpectedErr, err)
		}

		iox.Discard(out)
	}
}

// TestUpdateRootTaskfilePrunesRemovedManagedIncludes verifies stale managed aliases are dropped.
func TestUpdateRootTaskfilePrunesRemovedManagedIncludes(t *testing.T) {
	t.Parallel()

	root := []byte(yamlHeader + "includes:\n  old:\n    taskfile: taskfiles/old/Taskfile.yml\n")
	input := goRootInput()

	input.ManagedTasks = []string{"old"}

	out, err := UpdateRootTaskfile(root, input)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if bytesContain(out, "old:") {
		t.Fatalf("stale include retained: %s", out)
	}
}

// TestUpdateRootTaskfileMergesExistingIncludeVars verifies an existing managed entry is updated.
func TestUpdateRootTaskfileMergesExistingIncludeVars(t *testing.T) {
	t.Parallel()

	root := []byte(
		yamlHeader + "includes:\n  go:\n    taskfile: " + goDest +
			"\n    vars:\n      GO_VERSION: old\n",
	)
	input := goRootInput()

	out, err := UpdateRootTaskfile(root, input)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if !bytesContain(out, "GO_VERSION") {
		t.Fatalf("promoted var missing: %s", out)
	}
}

// TestUpdateRootTaskfileAcceptsScalarManagedInclude verifies a scalar include entry is managed.
func TestUpdateRootTaskfileAcceptsScalarManagedInclude(t *testing.T) {
	t.Parallel()

	root := []byte(yamlHeader + "includes:\n  go: " + goDest + "\n")

	out, err := UpdateRootTaskfile(root, goRootInput())
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	iox.Discard(out)
}

// TestIncludeTaskfileScalarRejectsNonMappingEntries verifies non-mapping include entries are skipped.
func TestIncludeTaskfileScalarRejectsNonMappingEntries(t *testing.T) {
	t.Parallel()

	node, ok := includeTaskfileScalar(yamlScalar(valueText))
	iox.Discard(node)

	if ok {
		t.Fatal("scalar entry should not yield a taskfile scalar")
	}

	node, ok = includeTaskfileScalar(mappingWithSequenceTaskfile())
	iox.Discard(node)

	if ok {
		t.Fatal("non-scalar taskfile should not yield a taskfile scalar")
	}
}

// TestApplyIncludePathReplacementsReportsSpanFailure verifies unresolvable spans fail.
func TestApplyIncludePathReplacementsReportsSpanFailure(t *testing.T) {
	t.Parallel()

	replacements := []includePathReplacement{{
		oldPath: "missing",
		newPath: "other",
		line:    consts.Index99,
		column:  consts.IndexOne,
		style:   yaml.Style(consts.IndexZero),
	}}

	out, err := applyIncludePathReplacements([]byte(yamlHeader), replacements)
	iox.Discard(out)
	assertFails(t, err)
}

// TestRewriteIncludePathKeepsUnrelatedPaths verifies non-Taskfile and unmapped paths are kept.
func TestRewriteIncludePathKeepsUnrelatedPaths(t *testing.T) {
	t.Parallel()

	mapping := map[string]string{pnpmModule: pnpmModule}

	assertPathUnchanged(t, "../pnpm/other.yml", mapping)
	assertPathUnchanged(t, "../../Taskfile.yml", mapping)
	assertPathUnchanged(t, "../unknown/Taskfile.yml", mapping)
}

// TestDestinationIncludePathFallsBackToOriginal verifies an empty fromDest keeps the path.
func TestDestinationIncludePathFallsBackToOriginal(t *testing.T) {
	t.Parallel()

	got := destinationIncludePath(consts.Empty, pnpmModule, oldPathValue)

	if got != oldPathValue {
		t.Fatalf(fmtDestInclude, got)
	}
}

// TestFinalizeRelativePrefixRejectsRemainingParent verifies a residual ".." clears the split.
func TestFinalizeRelativePrefixRejectsRemainingParent(t *testing.T) {
	t.Parallel()

	prefix, dir := finalizeRelativePrefix(dotSlash, "a/../b")

	if prefix != consts.Empty || dir != consts.Empty {
		t.Fatalf("finalizeRelativePrefix() = %q, %q", prefix, dir)
	}
}

// TestModuleIncludePathAndDirForNestedRoot verifies nested aggregator paths are relative.
func TestModuleIncludePathAndDirForNestedRoot(t *testing.T) {
	t.Parallel()

	path := moduleIncludePath(folderTaskfile, folderTaskfile, goTask)

	if path != "go/Taskfile.yml" {
		t.Fatalf("moduleIncludePath() = %q", path)
	}

	if dir := includeDirForRoot(folderTaskfile); dir != consts.PathParent {
		t.Fatalf("includeDirForRoot() = %q", dir)
	}
}

// TestExtractVarsNodeReportsMissingVars verifies module Taskfiles without vars are reported.
func TestExtractVarsNodeReportsMissingVars(t *testing.T) {
	t.Parallel()

	node, err := extractVarsNode([]byte(yamlHeader))
	iox.Discard(node)
	assertFails(t, err)

	node, err = extractVarsNode([]byte(moduleWithVars))
	iox.Discard(node)

	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

// TestParseModuleTaskfileNodeRejectsEmptyContent verifies empty and blank documents fail.
func TestParseModuleTaskfileNodeRejectsEmptyContent(t *testing.T) {
	t.Parallel()

	node, err := parseModuleTaskfileNode(nil)
	iox.Discard(node)
	assertFails(t, err)

	node, err = parseModuleTaskfileNode([]byte("# comment only\n"))
	iox.Discard(node)
	assertFails(t, err)
}

// TestMarshalNodeReportsEncoderFailure verifies an unencodable node is reported.
func TestMarshalNodeReportsEncoderFailure(t *testing.T) {
	t.Parallel()

	out, err := marshalNode(emptyDocumentNode(), "marshal: %v")
	iox.Discard(out)
	assertFails(t, err)

	out, err = marshalRootTaskfile(emptyDocumentNode())
	iox.Discard(out)
	assertFails(t, err)

	out, err = marshalUpdatedRootTaskfile(emptyDocumentNode(), newYAMLMappingNode(), goRootInput())
	iox.Discard(out)
	assertFails(t, err)
}

// TestCloneYAMLNodeCopiesNestedContent verifies clones are independent of the source.
func TestCloneYAMLNodeCopiesNestedContent(t *testing.T) {
	t.Parallel()

	if cloneYAMLNode(nil) != nil {
		t.Fatal("cloneYAMLNode(nil) should be nil")
	}

	source := newYAMLMappingNode()
	appendMappingPair(source, yamlScalar("key"), yamlScalar(valueText))

	clone := cloneYAMLNode(source)

	clone.Content[consts.IndexOne].Value = "changed"

	if source.Content[consts.IndexOne].Value != valueText {
		t.Fatal("clone shares nodes with the source")
	}
}

func assertPathUnchanged(t *testing.T, path string, mapping map[string]string) {
	t.Helper()

	if got := rewriteIncludePath(path, mapping, fromDestESLint); got != path {
		t.Fatalf("rewriteIncludePath(%q) = %q", path, got)
	}
}

func assertRewriteFails(t *testing.T, content []byte) {
	t.Helper()

	out, err := RewriteIncludes(content, map[string]string{pnpmModule: pnpmModule}, fromDestESLint)
	iox.Discard(out)
	assertFails(t, err)
}

func assertRootUpdateFails(t *testing.T, content []byte, input *rootupd.RootUpdateInput) {
	t.Helper()

	out, err := UpdateRootTaskfile(content, input)
	iox.Discard(out)
	assertFails(t, err)
}

func bytesContain(content []byte, want string) bool {
	return strings.Contains(string(content), want)
}

func goRootInput() *rootupd.RootUpdateInput {
	return &rootupd.RootUpdateInput{
		Tasks:            []string{goTask},
		TargetFolder:     folderTaskfile,
		RootTaskfileDir:  consts.Empty,
		DestByTask:       map[string]string{goTask: goTask},
		ManagedTasks:     nil,
		ModuleTaskfiles:  map[string][]byte{goTask: []byte(moduleWithVars)},
		GeneratedTasks:   nil,
		ManagedRootTasks: nil,
	}
}

func mappingWithSequenceTaskfile() *yaml.Node {
	entry := newYAMLMappingNode()
	appendMappingPair(entry, yamlScalar(keyTaskfile), newYAMLSequenceNodeForTest())

	return entry
}

func failIfErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

func emptyDocumentNode() *yaml.Node {
	//nolint:exhaustruct_v5 // an empty document node fails to encode, which is the point
	return &yaml.Node{Kind: yaml.DocumentNode}
}

func newYAMLSequenceNodeForTest() *yaml.Node {
	//nolint:exhaustruct_v5 // only the kind matters for this fixture
	return &yaml.Node{Kind: yaml.SequenceNode}
}
