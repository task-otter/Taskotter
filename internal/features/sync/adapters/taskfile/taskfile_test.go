// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	yaml "go.yaml.in/yaml/v3"
)

type (
	closingQuoteCase struct {
		content []byte
		want    int
		quote   byte
	}

	quoteStyleCase struct {
		style      yaml.Style
		wantQuote  byte
		wantQuoted bool
	}

	spanCase struct {
		start     int
		end       int
		wantStart int
	}

	positionCase struct {
		content []byte
		line    int
		column  int
	}
)

const (
	pnpmModule   = "pnpm"
	pnpmInclude  = "../../../pnpm/Taskfile.yml"
	absolutePath = "/abs/dest"
	varKeyName   = "GO_VERSION"
	goVersion122 = "1.22"

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

	wantErrText  = "expected error"
	oldPathValue = "../pnpm/Taskfile.yml"
	spanFmt      = "span = %d..%d, want %d..%d"
	twoLines     = "first\nsecond\n"
	otherLine    = "other\n"
	taskfileKey  = "taskfile: "
	oldTask      = "old"
	rawVarAA     = "AA"
	rawVarB      = "B"
)

// TestRewriteIncludesRejectsLiteralBlockPath verifies the expected behavior.
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

// TestRewriteIncludesRewritesQuotedPath verifies the expected behavior.
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

// TestRewriteIncludesSkipsNonMappingIncludes verifies the expected behavior.
func TestRewriteIncludesSkipsNonMappingIncludes(t *testing.T) {
	t.Parallel()

	assertRewriteNoop(t, []byte(yamlHeader+"includes: []\n"))
	assertRewriteNoop(t, []byte(yamlHeader+"includes:\n  pnpm: "+pnpmInclude+"\n"))
}

// TestRewriteIncludesHandlesDotSlashPrefix verifies the expected behavior.
func TestRewriteIncludesHandlesDotSlashPrefix(t *testing.T) {
	t.Parallel()

	content := []byte(yamlHeader + "includes:\n  pnpm:\n    taskfile: ./pnpm/Taskfile.yml\n")

	out, err := RewriteIncludes(content, map[string]string{pnpmModule: pnpmModule}, fromDestESLint)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	iox.Discard(out)
}

// TestRelativePathHelpersFallBackOnFailure verifies the expected behavior.
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

// TestUpdateRootTaskfileRejectsNonMappingTasks verifies the expected behavior.
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

// TestUpdateRootTaskfileKeepsStillGeneratedTasks verifies the expected behavior.
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

// TestIsManagedIncludeFallsBackToManagedTasks verifies the expected behavior.
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

// TestPromotedVarHelpersHandleMissingValues verifies the expected behavior.
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

// TestAddMissingPromotedVarSkipsUnknownKey verifies the expected behavior.
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

// TestMergeIncludeVarsSkipsNonMappingNodes verifies the expected behavior.
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
	appendMappingPair(vars, yamlScalar(varKeyName), yamlScalar(goVersion122))

	return vars
}

func pnpmMapping() map[string]string {
	return map[string]string{pnpmModule: pnpmModule}
}

// TestOverridableRootVarSkipsNilAndNonScalar verifies the expected behavior.
func TestOverridableRootVarSkipsNilAndNonScalar(t *testing.T) {
	t.Parallel()

	if overridableRootVar(varKeyName, nil) != nil {
		t.Fatal("nil value should pass through")
	}

	mapping := newYAMLMappingNode()

	if overridableRootVar(varKeyName, mapping) != mapping {
		t.Fatal("non-scalar should pass through")
	}
}

// TestOverridableRootVarPreservesExistingDefault verifies the expected behavior.
func TestOverridableRootVarPreservesExistingDefault(t *testing.T) {
	t.Parallel()

	already := yamlScalar(`{{.GO_VERSION | default "` + goVersion122 + `"}}`)

	if overridableRootVar(varKeyName, already) != already {
		t.Fatal("existing default should pass through")
	}
}

// TestOverridableRootVarWrapsScalars verifies the expected behavior.
func TestOverridableRootVarWrapsScalars(t *testing.T) {
	t.Parallel()

	plain := overridableRootVar(varKeyName, yamlScalar(goVersion122))

	if plain == nil || !strings.Contains(plain.Value, `| default "`+goVersion122+`"`) {
		t.Fatalf("plain default = %v", plain)
	}

	quoted := overridableRootVar(varKeyName, yamlScalar(`say "hi"`))

	if quoted == nil || !strings.Contains(quoted.Value, "`say \"hi\"`") {
		t.Fatalf("quoted default = %v", quoted)
	}
}

// TestOverridableDefaultArgBranches verifies the expected behavior.
func TestOverridableDefaultArgBranches(t *testing.T) {
	t.Parallel()

	if got := overridableDefaultArg("plain"); got != `"plain"` {
		t.Fatalf("plain = %q", got)
	}

	if got := overridableDefaultArg(`has "quote"`); got != "`has \"quote\"`" {
		t.Fatalf("quoted = %q", got)
	}
}

// TestOpsDelegatesToPackageHelpers verifies the expected behavior.
func TestOpsDelegatesToPackageHelpers(t *testing.T) {
	t.Parallel()

	ops := NewOps()

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

// TestOpsReportsFailures verifies the expected behavior.
func TestOpsReportsFailures(t *testing.T) {
	t.Parallel()

	ops := NewOps()

	out, err := ops.RewriteIncludes([]byte(badYAML), nil, consts.Empty)
	iox.Discard(out)
	assertFails(t, err)

	out, err = ops.UpdateRootTaskfile([]byte(badYAML), goRootInput())
	iox.Discard(out)
	assertFails(t, err)
}

// TestRewriteIncludesRejectsMalformedYAML verifies the expected behavior.
func TestRewriteIncludesRejectsMalformedYAML(t *testing.T) {
	t.Parallel()

	assertRewriteFails(t, []byte(badYAML))
	assertRewriteFails(t, []byte(consts.Empty))
}

// TestUpdateRootTaskfileRejectsMalformedYAML verifies the expected behavior.
func TestUpdateRootTaskfileRejectsMalformedYAML(t *testing.T) {
	t.Parallel()

	assertRootUpdateFails(t, []byte(badYAML), goRootInput())
	assertRootUpdateFails(t, []byte(consts.Empty), goRootInput())
}

// TestUpdateRootTaskfileRejectsNonMappingSections verifies the expected behavior.
func TestUpdateRootTaskfileRejectsNonMappingSections(t *testing.T) {
	t.Parallel()

	assertRootUpdateFails(t, []byte(yamlHeader+"includes: []\n"), goRootInput())
	assertRootUpdateFails(t, []byte(yamlHeader+"vars: []\n"), goRootInput())
}

// TestUpdateRootTaskfileRejectsMissingDestination verifies the expected behavior.
func TestUpdateRootTaskfileRejectsMissingDestination(t *testing.T) {
	t.Parallel()

	input := goRootInput()

	input.DestByTask = map[string]string{}

	assertRootUpdateFails(t, NewRootTemplate(), input)
}

// TestUpdateRootTaskfileRejectsUnmanagedAlias verifies the expected behavior.
func TestUpdateRootTaskfileRejectsUnmanagedAlias(t *testing.T) {
	t.Parallel()

	root := []byte(yamlHeader + "includes:\n  go:\n    taskfile: legacy/go/Taskfile.yml\n")
	assertRootUpdateFails(t, root, goRootInput())
}

// TestUpdateRootTaskfileRejectsMalformedModuleTaskfile verifies the expected behavior.
func TestUpdateRootTaskfileRejectsMalformedModuleTaskfile(t *testing.T) {
	t.Parallel()

	input := goRootInput()

	input.ModuleTaskfiles = map[string][]byte{goTask: []byte(badYAML)}

	assertRootUpdateFails(t, NewRootTemplate(), input)
}

// TestUpdateRootTaskfileSkipsModulesWithoutVars verifies the expected behavior.
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

// TestUpdateRootTaskfilePrunesRemovedManagedIncludes verifies the expected behavior.
func TestUpdateRootTaskfilePrunesRemovedManagedIncludes(t *testing.T) {
	t.Parallel()

	root := []byte(yamlHeader + "includes:\n  old:\n    taskfile: taskfiles/old/Taskfile.yml\n")
	input := goRootInput()

	input.ManagedTasks = []string{oldTask}

	out, err := UpdateRootTaskfile(root, input)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if bytesContain(out, "old:") {
		t.Fatalf("stale include retained: %s", out)
	}
}

// TestUpdateRootTaskfileMergesExistingIncludeVars verifies the expected behavior.
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

// TestUpdateRootTaskfileAcceptsScalarManagedInclude verifies the expected behavior.
func TestUpdateRootTaskfileAcceptsScalarManagedInclude(t *testing.T) {
	t.Parallel()

	root := []byte(yamlHeader + "includes:\n  go: " + goDest + "\n")

	out, err := UpdateRootTaskfile(root, goRootInput())
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	iox.Discard(out)
}

// TestIncludeTaskfileScalarRejectsNonMappingEntries verifies the expected behavior.
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

// TestApplyIncludePathReplacementsReportsSpanFailure verifies the expected behavior.
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

// TestRewriteIncludePathKeepsUnrelatedPaths verifies the expected behavior.
func TestRewriteIncludePathKeepsUnrelatedPaths(t *testing.T) {
	t.Parallel()

	mapping := map[string]string{pnpmModule: pnpmModule}

	assertPathUnchanged(t, "../pnpm/other.yml", mapping)
	assertPathUnchanged(t, "../../Taskfile.yml", mapping)
	assertPathUnchanged(t, "../unknown/Taskfile.yml", mapping)
}

// TestDestinationIncludePathFallsBackToOriginal verifies the expected behavior.
func TestDestinationIncludePathFallsBackToOriginal(t *testing.T) {
	t.Parallel()

	got := destinationIncludePath(consts.Empty, pnpmModule, oldPathValue)

	if got != oldPathValue {
		t.Fatalf(fmtDestInclude, got)
	}
}

// TestFinalizeRelativePrefixRejectsRemainingParent verifies the expected behavior.
func TestFinalizeRelativePrefixRejectsRemainingParent(t *testing.T) {
	t.Parallel()

	prefix, dir := finalizeRelativePrefix(dotSlash, "a/../b")

	if prefix != consts.Empty || dir != consts.Empty {
		t.Fatalf("finalizeRelativePrefix() = %q, %q", prefix, dir)
	}
}

// TestModuleIncludePathAndDirForNestedRoot verifies the expected behavior.
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

// TestExtractVarsNodeReportsMissingVars verifies the expected behavior.
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

// TestParseModuleTaskfileNodeRejectsEmptyContent verifies the expected behavior.
func TestParseModuleTaskfileNodeRejectsEmptyContent(t *testing.T) {
	t.Parallel()

	node, err := parseModuleTaskfileNode(nil)
	iox.Discard(node)
	assertFails(t, err)

	node, err = parseModuleTaskfileNode([]byte("# comment only\n"))
	iox.Discard(node)
	assertFails(t, err)
}

// TestMarshalNodeReportsEncoderFailure verifies the expected behavior.
func TestMarshalNodeReportsEncoderFailure(t *testing.T) {
	t.Parallel()

	out, err := marshalNode(emptyDocumentNode(), "marshal: %v")
	iox.Discard(out)
	assertFails(t, err)

	out, err = marshalRootTaskfile(emptyDocumentNode())
	iox.Discard(out)
	assertFails(t, err)

	out, err = marshalUpdatedRootTaskfile(&marshalRootParams{
		node: emptyDocumentNode(), root: newYAMLMappingNode(), input: goRootInput(),
	})
	iox.Discard(out)
	assertFails(t, err)
}

// TestCloneYAMLNodeCopiesNestedContent verifies the expected behavior.
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

func assertRootUpdateFails(t *testing.T, content []byte, input *ports.RootUpdateInput) {
	t.Helper()

	out, err := UpdateRootTaskfile(content, input)
	iox.Discard(out)
	assertFails(t, err)
}

func bytesContain(content []byte, want string) bool {
	return strings.Contains(string(content), want)
}

func goRootInput() *ports.RootUpdateInput {
	return &ports.RootUpdateInput{
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
	return &yaml.Node{Kind: yaml.DocumentNode}
}

func newYAMLSequenceNodeForTest() *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode}
}

// TestQuoteForYAMLStyleMapsQuotedStyles verifies each scalar style maps to its quote byte.
func TestQuoteForYAMLStyleMapsQuotedStyles(t *testing.T) {
	t.Parallel()

	cases := []quoteStyleCase{
		{style: yaml.DoubleQuotedStyle, wantQuote: '"', wantQuoted: true},
		{style: yaml.SingleQuotedStyle, wantQuote: '\'', wantQuoted: true},
		{style: yaml.LiteralStyle, wantQuote: byte(consts.IndexZero), wantQuoted: false},
	}

	for i := range cases {
		assertQuote(t, &cases[i])
	}
}

// TestPlainScalarValueSpanFindsPath verifies the plain scalar span covers the old path.
func TestPlainScalarValueSpanFindsPath(t *testing.T) {
	t.Parallel()

	content := []byte(taskfileKey + oldPathValue + "\n")
	offset := len(taskfileKey)

	start, end, err := plainScalarValueSpan(content, offset, oldPathValue)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	assertSpan(t, &spanCase{start: start, end: end, wantStart: offset})
}

// TestPlainScalarValueSpanReportsMissingPath verifies a mismatched path is reported.
func TestPlainScalarValueSpanReportsMissingPath(t *testing.T) {
	t.Parallel()

	start, end, err := plainScalarValueSpan(
		[]byte(taskfileKey+otherLine),
		consts.IndexZero,
		oldPathValue,
	)
	iox.Discard2(start, end)
	assertFails(t, err)
}

// TestQuotedScalarValueSpanFindsInterior verifies double and single quoted spans.
func TestQuotedScalarValueSpanFindsInterior(t *testing.T) {
	t.Parallel()

	assertQuotedSpan(t, '"')
	assertQuotedSpan(t, '\'')
}

// TestQuotedScalarValueSpanRejectsUnquotedScalar verifies a missing opening quote fails.
func TestQuotedScalarValueSpanRejectsUnquotedScalar(t *testing.T) {
	t.Parallel()

	assertQuotedSpanFails(t, []byte(oldPathValue))
	assertQuotedSpanFails(t, []byte(consts.Empty))
}

// TestQuotedScalarValueSpanReportsUnterminatedQuote verifies a missing closing quote fails.
func TestQuotedScalarValueSpanReportsUnterminatedQuote(t *testing.T) {
	t.Parallel()
	assertQuotedSpanFails(t, []byte(`"`+oldPathValue))
}

// TestQuotedScalarValueSpanReportsPathMismatch verifies a different quoted value fails.
func TestQuotedScalarValueSpanReportsPathMismatch(t *testing.T) {
	t.Parallel()
	assertQuotedSpanFails(t, []byte(`"other"`))
}

// TestFindClosingQuoteSkipsEscapes verifies escaped quotes do not terminate the scalar.
func TestFindClosingQuoteSkipsEscapes(t *testing.T) {
	t.Parallel()

	cases := []closingQuoteCase{
		{content: []byte(`"a\"b"`), quote: '"', want: consts.IndexOne + len(`a\"b`)},
		{content: []byte(`'a''b'`), quote: '\'', want: consts.IndexOne + len(`a''b`)},
	}

	for i := range cases {
		assertClosingQuote(t, &cases[i])
	}
}

// TestOffsetAtLineColumnRejectsInvalidPositions verifies out-of-range positions fail.
func TestOffsetAtLineColumnRejectsInvalidPositions(t *testing.T) {
	t.Parallel()

	content := []byte(twoLines)

	cases := []positionCase{
		{content: content, line: consts.IndexZero, column: consts.IndexOne},
		{content: content, line: consts.IndexOne, column: consts.IndexZero},
		{content: content, line: consts.Index99, column: consts.IndexOne},
		{content: content, line: consts.IndexOne, column: consts.Index99},
	}

	for i := range cases {
		assertOffsetFails(t, &cases[i])
	}
}

// TestOffsetAtLineColumnResolvesPosition verifies a valid position maps to a byte offset.
func TestOffsetAtLineColumnResolvesPosition(t *testing.T) {
	t.Parallel()

	offset, err := offsetAtLineColumn(&yamlPosition{
		content: []byte(twoLines),
		line:    consts.IndexTwo,
		column:  consts.IndexOne,
	})
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	wantOffset := len(twoLines) - len("second\n")

	if offset != wantOffset {
		t.Fatalf("offset = %d, want %d", offset, wantOffset)
	}
}

// TestLineEndOffsetHandlesMissingNewline verifies content without a trailing newline.
func TestLineEndOffsetHandlesMissingNewline(t *testing.T) {
	t.Parallel()

	content := []byte("no newline")

	if got := lineEndOffset(content, consts.IndexZero); got != len(content) {
		t.Fatalf("lineEndOffset() = %d, want %d", got, len(content))
	}
}

// TestScalarValueSpanReportsQuotedFailure verifies quoted span failures propagate.
func TestScalarValueSpanReportsQuotedFailure(t *testing.T) {
	t.Parallel()

	start, end, err := scalarValueSpan(&scalarSpanParams{
		content: []byte(oldPathValue),
		offset:  consts.IndexZero,
		oldPath: oldPathValue,
		style:   yaml.DoubleQuotedStyle,
	})
	iox.Discard2(start, end)
	assertFails(t, err)
}

// TestSpanFromReplacementReportsFailure verifies span resolution failures propagate.
func TestSpanFromReplacementReportsFailure(t *testing.T) {
	t.Parallel()

	span, err := spanFromReplacement([]byte(otherLine), consts.IndexZero, newReplacement())
	iox.Discard(span)
	assertFails(t, err)
}

// TestIncludePathSpanForReplacementReportsFailures verifies offset and span failures propagate.
func TestIncludePathSpanForReplacementReportsFailures(t *testing.T) {
	t.Parallel()

	replacement := newReplacement()

	replacement.line = consts.IndexZero

	span, err := includePathSpanForReplacement([]byte(otherLine), replacement)
	iox.Discard(span)
	assertFails(t, err)

	span, err = includePathSpanForReplacement([]byte(otherLine), newReplacement())
	iox.Discard(span)
	assertFails(t, err)
}

func assertClosingQuote(t *testing.T, testCase *closingQuoteCase) {
	t.Helper()

	idx, err := findClosingQuote(testCase.content, consts.IndexZero, testCase.quote)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if idx != testCase.want {
		t.Fatalf("closing quote = %d, want %d", idx, testCase.want)
	}
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func assertOffsetFails(t *testing.T, testCase *positionCase) {
	t.Helper()

	offset, err := offsetAtLineColumn(&yamlPosition{
		content: testCase.content,
		line:    testCase.line,
		column:  testCase.column,
	})
	iox.Discard(offset)
	assertFails(t, err)
}

func assertQuote(t *testing.T, testCase *quoteStyleCase) {
	t.Helper()

	quote, quoted := quoteForYAMLStyle(testCase.style)

	if quote != testCase.wantQuote || quoted != testCase.wantQuoted {
		t.Fatalf("quoteForYAMLStyle(%v) = %q, %t", testCase.style, quote, quoted)
	}
}

func assertQuotedSpan(t *testing.T, quote byte) {
	t.Helper()

	content := []byte(string(quote) + oldPathValue + string(quote))

	start, end, err := quotedScalarValueSpan(&quotedSpanParams{
		content: content,
		offset:  consts.IndexZero,
		oldPath: oldPathValue,
		quote:   quote,
	})
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	assertSpan(t, &spanCase{start: start, end: end, wantStart: consts.IndexOne})
}

func assertQuotedSpanFails(t *testing.T, content []byte) {
	t.Helper()

	start, end, err := quotedScalarValueSpan(&quotedSpanParams{
		content: content,
		offset:  consts.IndexZero,
		oldPath: oldPathValue,
		quote:   '"',
	})
	iox.Discard2(start, end)
	assertFails(t, err)
}

func assertSpan(t *testing.T, testCase *spanCase) {
	t.Helper()

	wantEnd := testCase.wantStart + len(oldPathValue)

	if testCase.start != testCase.wantStart || testCase.end != wantEnd {
		t.Fatalf(spanFmt, testCase.start, testCase.end, testCase.wantStart, wantEnd)
	}
}

func newReplacement() *includePathReplacement {
	return &includePathReplacement{
		oldPath: oldPathValue,
		newPath: "../npm/Taskfile.yml",
		line:    consts.IndexOne,
		column:  consts.IndexOne,
		style:   yaml.Style(consts.IndexZero),
	}
}

// TestRawVarHelpersCoverReplacementBranches verifies the behavior covered by this test.
func TestRawVarHelpersCoverReplacementBranches(t *testing.T) {
	t.Parallel()

	raw := map[string]string{
		rawVarA:  "one",
		rawVarAA: "two",
		rawVarB:  consts.Empty,
	}
	assertRawVarReplacement(t, raw)
}

func assertRawVarReplacement(t *testing.T, raw map[string]string) {
	t.Helper()

	assertRawVarKeys(t, raw)
	assertRawVarSplice(t, raw)
}

func assertRawVarKeys(t *testing.T, raw map[string]string) {
	t.Helper()

	keys := rawVarKeysLongestFirst(raw)

	if len(keys) != consts.IndexThree || keys[consts.IndexZero] != rawVarAA {
		t.Fatalf("keys = %#v", keys)
	}
}

func assertRawVarSplice(t *testing.T, raw map[string]string) {
	t.Helper()

	mapNode := &yaml.Node{Kind: yaml.MappingNode}
	replaceOrAppendScalar(mapNode, rawVarA, oldTask)
	replaceOrAppendScalar(mapNode, rawVarA, "new")
	placeholderOneRootVar(mapNode, rawVarB, consts.Empty)

	out := spliceRawPromotedVars(
		[]byte(rawVarPlaceholder(rawVarAA)+" "+rawVarPlaceholder(rawVarA)),
		raw,
	)

	if !bytes.Contains(out, []byte("two one")) {
		t.Fatalf("spliced = %q", out)
	}
}

// TestRawBlockOffsetHelpersCoverBounds verifies the behavior covered by this test.
func TestRawBlockOffsetHelpersCoverBounds(t *testing.T) {
	t.Parallel()

	content := []byte("vars:\n  A: |\n    one\n")

	assertRawBlockOffsets(t, content)
	assertRawBlockLineTraversal(t)
}

const (
	rawVarA      = "A"
	rawBlockLast = "last"
)

func assertRawBlockOffsets(t *testing.T, content []byte) {
	t.Helper()

	assertInt(t, intAssertion{
		got:   len(trimRawBlock(content, len(content), consts.IndexOne)),
		want:  consts.IndexZero,
		label: "trimRawBlock",
	})
	assertInt(t, intAssertion{
		got:   offsetOfLine(content, consts.Index99),
		want:  len(content),
		label: "offsetOfLine",
	})
	assertInt(t, intAssertion{
		got:   offsetOfLine(content, consts.IndexOne),
		want:  consts.IndexZero,
		label: "offsetOfLine(first)",
	})
	assertInt(t, intAssertion{
		got: addColumnOffset(
			content,
			consts.IndexOne,
			consts.IndexZero,
		),
		want:  consts.IndexOne,
		label: "addColumnOffset",
	})
	assertInt(t, intAssertion{
		got:   clampOffset(consts.IndexOne, consts.IndexTwo),
		want:  consts.IndexOne,
		label: "clampOffset",
	})
}

func assertRawBlockLineTraversal(t *testing.T) {
	t.Helper()

	next, done := advanceBlockLine(
		[]byte(rawBlockLast),
		consts.IndexZero,
		consts.IndexOne,
	)

	if next != len(rawBlockLast) || !done {
		t.Fatalf("advanceBlockLine() = %d, %t", next, done)
	}

	if next, ok := nextLineStart(
		[]byte(rawBlockLast),
		consts.IndexZero,
	); next != consts.IndexZero ||
		ok {

		t.Fatalf("nextLineStart() = %d, %t", next, ok)
	}
}

type intAssertion struct {
	label string
	got   int
	want  int
}

func assertInt(t *testing.T, assertion intAssertion) {
	t.Helper()

	if assertion.got != assertion.want {
		t.Fatalf("%s = %d, want %d", assertion.label, assertion.got, assertion.want)
	}
}

// TestStoreExtractedVarsSkipsNilAndNotOK verifies the behavior covered by this test.
func TestStoreExtractedVarsSkipsNilAndNotOK(t *testing.T) {
	t.Parallel()

	result := &rootVarsResult{byTask: map[string]*yaml.Node{}, raw: map[string]string{}}

	storeExtractedVars(result, "nil", nil)
	storeExtractedVars(result, "not-ok", &extractedVars{})

	if len(result.byTask) != consts.IndexZero {
		t.Fatalf("byTask = %#v", result.byTask)
	}
}
