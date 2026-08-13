// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package taskfile reads and rewrites Taskfile YAML includes and promoted root vars.
package taskfile

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/yamlfmt"
	yaml "go.yaml.in/yaml/v3"
)

type (
	// RewriteError reports Taskfile YAML rewrite failures.
	RewriteError struct {
		Message string
	}

	rootUpdateInput   = rootupd.RootUpdateInput
	generatedRootTask = rootupd.GeneratedRootTask

	// includesUpdateParams carries state for merging managed includes into the root Taskfile.
	includesUpdateParams struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		moduleVars   map[string]*yaml.Node
		input        *rootUpdateInput
	}

	// includeUpsertParams carries state for upserting one managed include entry.
	includeUpsertParams struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		moduleVars   map[string]*yaml.Node
		input        *rootUpdateInput
		task         string
	}

	// existingIncludeParams carries state for updating an existing include entry.
	existingIncludeParams struct {
		entry        *yaml.Node
		moduleVars   *yaml.Node
		path         string
		dir          string
		task         string
		managedTasks []string
	}

	// managedIncludeParams carries state for checking whether an include is managed.
	managedIncludeParams struct {
		entry        *yaml.Node
		expectedPath string
		task         string
		managedTasks []string
	}

	// pruneIncludesParams carries state for removing stale managed includes.
	pruneIncludesParams struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		managedSet   map[string]struct{}
		managedTasks []string
	}

	// promotedVarParams carries state for promoting module vars to the root.
	promotedVarParams struct {
		root         *yaml.Node
		moduleVars   map[string]*yaml.Node
		promotedVars map[string]struct{}
		tasks        []string
	}

	// addPromotedVarParams carries state for adding one promoted var to the root.
	addPromotedVarParams struct {
		rootVars   *yaml.Node
		moduleVars map[string]*yaml.Node
		existing   map[string]struct{}
		key        string
		tasks      []string
	}

	// mergeModuleVarParams carries state for merging one module var into an include.
	mergeModuleVarParams struct {
		existingVars *yaml.Node
		key          string
	}

	yamlNodeMap = map[string]*yaml.Node
	strSet      = map[string]struct{}
	rootUpdIn   = rootUpdateInput

	rewriteParams struct {
		path         string
		sourceToDest map[string]string
		fromDest     string
		dir          string
	}

	// includePathReplacement locates one include taskfile scalar in the original YAML.
	includePathReplacement struct {
		oldPath string
		newPath string
		line    int
		column  int
		style   yaml.Style
	}

	// includePathSpan is a byte range in the original content to overwrite.
	includePathSpan struct {
		value string
		start int
		end   int
	}

	// rewriteIncludesParams carries state for rewriting include paths in a Taskfile.
	rewriteIncludesParams struct {
		root         *yaml.Node
		sourceToDest map[string]string
		fromDest     string
		content      []byte
	}

	// yamlPosition locates a scalar at a 1-based YAML line/column in content.
	yamlPosition struct {
		content []byte
		line    int
		column  int
	}

	// scalarSpanParams locates the byte span of a scalar value at offset.
	scalarSpanParams struct {
		oldPath string
		content []byte
		offset  int
		style   yaml.Style
	}

	// quotedSpanParams locates the interior of a quoted scalar at offset.
	quotedSpanParams struct {
		oldPath string
		content []byte
		offset  int
		quote   byte
	}

	// replaceSpanParams overwrites content[start:end] with value.
	replaceSpanParams struct {
		value   string
		content []byte
		start   int
		end     int
	}

	// collectIncludeReplacementsParams carries state for collecting include path edits.
	collectIncludeReplacementsParams struct {
		includes     *yaml.Node
		entry        *yaml.Node
		sourceToDest map[string]string
		fromDest     string
		out          []includePathReplacement
	}
)

const (
	rootTaskfileVersion     = "3.5"
	rootTemplate            = "---\nversion: \"" + rootTaskfileVersion + "\"\n"
	yamlMappingPairKeyValue = consts.IndexTwo

	dotSlash       = "./"
	dotDotSlash    = "../"
	keyIncludes    = "includes"
	keyTaskfile    = "taskfile"
	keyDir         = "dir"
	keyVars        = "vars"
	keyTasks       = "tasks"
	taskfileSuffix = "Taskfile.yml"

	errParseTaskfileRoot  = "parse taskfile root: %w"
	errMarshalAndValidate = "marshal and validate: %w"
)

var errNoModuleVars = errors.New("module Taskfile has no vars")

// NewRootTemplate returns the minimal root Taskfile used when none exists yet.
func NewRootTemplate() []byte {
	return []byte(rootTemplate)
}

// Error implements the error interface, returning the Taskfile rewrite failure message.
func (e *RewriteError) Error() string {
	return e.Message
}

// RewriteIncludes updates include taskfile paths using sourceToDest mappings.
// fromDest is the destination module directory of the Taskfile being rewritten
// (for example "eslint" or "internal/skipfiles"), used to recompute relative
// include paths after destination normalization.
//
// Only include path scalars are edited in the original bytes; all other YAML
// formatting (including folded `>-` blocks) is preserved unchanged.
func RewriteIncludes(
	content []byte,
	sourceToDest map[string]string,
	fromDest string,
) ([]byte, error) {
	out, err := rewriteIncludesFromContent(&rewriteIncludesParams{
		content:      content,
		root:         nil,
		sourceToDest: sourceToDest,
		fromDest:     fromDest,
	})
	if err != nil {
		return nil, fmt.Errorf("rewrite includes from content: %w", err)
	}

	return out, nil
}

func rewriteIncludesFromContent(params *rewriteIncludesParams) ([]byte, error) {
	doc, root, err := parseTaskfileRoot(
		params.content,
		"parse Taskfile YAML: %v",
		"empty Taskfile YAML",
	)
	if err != nil {
		return nil, fmt.Errorf(errParseTaskfileRoot, err)
	}

	iox.Discard(doc)

	params.root = root

	out, err := applyRewriteIncludes(params)
	if err != nil {
		return nil, fmt.Errorf("apply rewrite includes: %w", err)
	}

	return out, nil
}

func applyRewriteIncludes(params *rewriteIncludesParams) ([]byte, error) {
	replacements := collectIncludePathReplacements(
		params.root,
		params.sourceToDest,
		params.fromDest,
	)

	if len(replacements) == consts.IndexZero {
		return params.content, nil
	}

	out, err := applyIncludePathReplacements(params.content, replacements)
	if err != nil {
		return nil, fmt.Errorf("apply include path replacements: %w", err)
	}

	return out, nil
}

func collectIncludePathReplacements(
	root *yaml.Node,
	sourceToDest map[string]string,
	fromDest string,
) []includePathReplacement {
	includesNode := findMappingValue(root, keyIncludes)

	if includesNode == nil {
		return nil
	}

	return collectIncludePathReplacementsFromNode(includesNode, sourceToDest, fromDest)
}

func collectIncludePathReplacementsFromNode(
	includes *yaml.Node,
	sourceToDest map[string]string,
	fromDest string,
) []includePathReplacement {
	if includes.Kind != yaml.MappingNode {
		return nil
	}

	capacity := len(includes.Content)/yamlMappingPairKeyValue + consts.IndexOne
	out := make([]includePathReplacement, consts.IndexZero, capacity)

	return appendIncludePathReplacements(&collectIncludeReplacementsParams{
		out:          out,
		includes:     includes,
		entry:        nil,
		sourceToDest: sourceToDest,
		fromDest:     fromDest,
	})
}

func appendIncludePathReplacements(
	params *collectIncludeReplacementsParams,
) []includePathReplacement {
	for idx := consts.IndexZero; idx < len(params.includes.Content); idx += yamlMappingPairKeyValue {
		params.out = appendIncludeEntryReplacement(&collectIncludeReplacementsParams{
			out:          params.out,
			includes:     nil,
			entry:        params.includes.Content[idx+consts.IndexOne],
			sourceToDest: params.sourceToDest,
			fromDest:     params.fromDest,
		})
	}

	return params.out
}

func appendIncludeEntryReplacement(
	params *collectIncludeReplacementsParams,
) []includePathReplacement {
	replacement, ok := includePathReplacementForEntry(
		params.entry,
		params.sourceToDest,
		params.fromDest,
	)

	if !ok {
		return params.out
	}

	return append(params.out, replacement)
}

func includePathReplacementForEntry(
	entry *yaml.Node,
	sourceToDest map[string]string,
	fromDest string,
) (includePathReplacement, bool) {
	taskfileNode, ok := includeTaskfileScalar(entry)

	if !ok {
		return emptyIncludePathReplacement(), false
	}

	return replacementFromTaskfileNode(taskfileNode, sourceToDest, fromDest)
}

func includeTaskfileScalar(entry *yaml.Node) (*yaml.Node, bool) {
	if entry.Kind != yaml.MappingNode {
		return nil, false
	}

	taskfileNode := findMappingValue(entry, keyTaskfile)

	if taskfileNode == nil || taskfileNode.Kind != yaml.ScalarNode {
		return nil, false
	}

	return taskfileNode, true
}

func replacementFromTaskfileNode(
	taskfileNode *yaml.Node,
	sourceToDest map[string]string,
	fromDest string,
) (includePathReplacement, bool) {
	newPath := rewriteIncludePath(taskfileNode.Value, sourceToDest, fromDest)

	if newPath == taskfileNode.Value {
		return emptyIncludePathReplacement(), false
	}

	return includePathReplacement{
		line:    taskfileNode.Line,
		column:  taskfileNode.Column,
		oldPath: taskfileNode.Value,
		newPath: newPath,
		style:   taskfileNode.Style,
	}, true
}

func emptyIncludePathReplacement() includePathReplacement {
	return includePathReplacement{
		oldPath: consts.Empty,
		newPath: consts.Empty,
		line:    consts.IndexZero,
		column:  consts.IndexZero,
		style:   yaml.Style(consts.IndexZero),
	}
}

func applyIncludePathReplacements(
	content []byte,
	replacements []includePathReplacement,
) ([]byte, error) {
	spans, err := includePathSpans(content, replacements)
	if err != nil {
		return nil, fmt.Errorf("resolve include path spans: %w", err)
	}

	return replaceIncludePathSpans(content, spans), nil
}

func includePathSpans(
	content []byte,
	replacements []includePathReplacement,
) ([]includePathSpan, error) {
	spans := make([]includePathSpan, consts.IndexZero, len(replacements))

	for i := range replacements {
		span, err := includePathSpanForReplacement(content, &replacements[i])
		if err != nil {
			return nil, fmt.Errorf("resolve include path span: %w", err)
		}

		spans = append(spans, span)
	}

	return spans, nil
}

func replaceIncludePathSpans(content []byte, spans []includePathSpan) []byte {
	slices.SortFunc(spans, func(left, right includePathSpan) int {
		return cmp.Compare(right.start, left.start)
	})

	out := content

	for i := range spans {
		out = replaceByteSpan(&replaceSpanParams{
			content: out,
			start:   spans[i].start,
			end:     spans[i].end,
			value:   spans[i].value,
		})
	}

	return out
}

func includePathSpanForReplacement(
	content []byte,
	replacement *includePathReplacement,
) (includePathSpan, error) {
	offset, err := replacementContentOffset(content, replacement)
	if err != nil {
		return emptyIncludePathSpan(), fmt.Errorf("replacement content offset: %w", err)
	}

	span, err := spanFromReplacement(content, offset, replacement)
	if err != nil {
		return emptyIncludePathSpan(), fmt.Errorf("span from replacement: %w", err)
	}

	return span, nil
}

func emptyIncludePathSpan() includePathSpan {
	return includePathSpan{
		value: consts.Empty,
		start: consts.IndexZero,
		end:   consts.IndexZero,
	}
}

func replacementContentOffset(
	content []byte,
	replacement *includePathReplacement,
) (int, error) {
	offset, err := offsetAtLineColumn(&yamlPosition{
		content: content,
		line:    replacement.line,
		column:  replacement.column,
	})
	if err != nil {
		return consts.IndexZero, fmt.Errorf("offset at line/column: %w", err)
	}

	return offset, nil
}

func spanFromReplacement(
	content []byte,
	offset int,
	replacement *includePathReplacement,
) (includePathSpan, error) {
	start, end, err := scalarValueSpan(&scalarSpanParams{
		content: content,
		offset:  offset,
		oldPath: replacement.oldPath,
		style:   replacement.style,
	})
	if err != nil {
		return emptyIncludePathSpan(), fmt.Errorf("scalar value span: %w", err)
	}

	return includePathSpan{
		start: start,
		end:   end,
		value: replacement.newPath,
	}, nil
}

func offsetAtLineColumn(pos *yamlPosition) (int, error) {
	if pos.line < consts.IndexOne || pos.column < consts.IndexOne {
		return consts.IndexZero, fmt.Errorf(
			"invalid yaml position: %w",
			invalidYAMLPosition(pos.line, pos.column),
		)
	}

	offset, err := offsetForValidLineColumn(pos)
	if err != nil {
		return consts.IndexZero, fmt.Errorf("offset for valid line column: %w", err)
	}

	return offset, nil
}

func invalidYAMLPosition(line, column int) error {
	return &RewriteError{
		Message: fmt.Sprintf("invalid YAML position line=%d column=%d", line, column),
	}
}

func offsetForValidLineColumn(pos *yamlPosition) (int, error) {
	offset, err := lineStartOffset(pos.content, pos.line)
	if err != nil {
		return consts.IndexZero, fmt.Errorf("line start offset: %w", err)
	}

	colOffset, err := columnOffsetInLine(pos, offset)
	if err != nil {
		return consts.IndexZero, fmt.Errorf("column offset in line: %w", err)
	}

	return colOffset, nil
}

func lineStartOffset(content []byte, line int) (int, error) {
	offset := consts.IndexZero
	currentLine := consts.IndexOne

	for currentLine < line {
		next, err := advanceToNextLine(content, offset, line)
		if err != nil {
			return consts.IndexZero, fmt.Errorf("advance to next line: %w", err)
		}

		offset = next
		currentLine++
	}

	return offset, nil
}

func advanceToNextLine(content []byte, offset, line int) (int, error) {
	newline := bytes.IndexByte(content[offset:], '\n')

	if newline < consts.IndexZero {
		return consts.IndexZero, &RewriteError{
			Message: fmt.Sprintf("YAML line %d past end of content", line),
		}
	}

	return offset + newline + consts.IndexOne, nil
}

func columnOffsetInLine(pos *yamlPosition, offset int) (int, error) {
	lineEnd := lineEndOffset(pos.content, offset)
	colOffset := pos.column - consts.IndexOne

	if colOffset > lineEnd {
		return consts.IndexZero, &RewriteError{
			Message: fmt.Sprintf("YAML column %d past end of line %d", pos.column, pos.line),
		}
	}

	return offset + colOffset, nil
}

func lineEndOffset(content []byte, offset int) int {
	lineEnd := bytes.IndexByte(content[offset:], '\n')

	if lineEnd < consts.IndexZero {
		return len(content) - offset
	}

	return lineEnd
}

func scalarValueSpan(params *scalarSpanParams) (start, end int, err error) {
	start, end, err = scalarSpanByQuote(params)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf("resolve scalar value span: %w", err)
	}

	return start, end, nil
}

func scalarSpanByQuote(params *scalarSpanParams) (start, end int, err error) {
	quote, quoted := quoteForYAMLStyle(params.style)

	start, end, err = spanForQuoteChoice(params, quote, quoted)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf(
			"scalar value span for style: %w",
			err,
		)
	}

	return start, end, nil
}

func spanForQuoteChoice(
	params *scalarSpanParams,
	quote byte,
	quoted bool,
) (start, end int, err error) {
	switch quoted {
	case true:
		start, end, err = wrapQuotedScalarSpan(params, quote)
	default:
		start, end, err = wrapPlainScalarSpan(params)
	}

	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf("span for quote choice: %w", err)
	}

	return start, end, nil
}

// quoteForYAMLStyle returns the quote byte for quoted scalar styles.
//
//nolint:exhaustive // Tagged/Literal/Folded/Flow and plain (0) use unquoted spans
func quoteForYAMLStyle(style yaml.Style) (quote byte, quoted bool) {
	switch style {
	case yaml.DoubleQuotedStyle:
		return '"', true
	case yaml.SingleQuotedStyle:
		return '\'', true
	default:
		return byte(consts.IndexZero), false
	}
}

func wrapQuotedScalarSpan(params *scalarSpanParams, quote byte) (start, end int, err error) {
	start, end, err = quotedScalarValueSpan(&quotedSpanParams{
		content: params.content,
		offset:  params.offset,
		oldPath: params.oldPath,
		quote:   quote,
	})
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf("quoted scalar value span: %w", err)
	}

	return start, end, nil
}

func wrapPlainScalarSpan(params *scalarSpanParams) (start, end int, err error) {
	start, end, err = plainScalarValueSpan(params.content, params.offset, params.oldPath)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf("plain scalar value span: %w", err)
	}

	return start, end, nil
}

func plainScalarValueSpan(content []byte, offset int, oldPath string) (start, end int, err error) {
	pathBytes := []byte(oldPath)

	if !bytes.HasPrefix(content[offset:], pathBytes) {
		return consts.IndexZero, consts.IndexZero, &RewriteError{
			Message: fmt.Sprintf("include path %q not found at YAML position", oldPath),
		}
	}

	return offset, offset + len(pathBytes), nil
}

func quotedScalarValueSpan(params *quotedSpanParams) (start, end int, err error) {
	if params.offset >= len(params.content) || params.content[params.offset] != params.quote {
		return consts.IndexZero, consts.IndexZero, &RewriteError{
			Message: fmt.Sprintf(
				"expected %q-quoted include path at YAML position",
				params.quote,
			),
		}
	}

	start, end, err = quotedScalarInteriorSpan(params)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf(
			"quoted scalar interior span: %w",
			err,
		)
	}

	return start, end, nil
}

func quotedScalarInteriorSpan(params *quotedSpanParams) (start, end int, err error) {
	closeIdx, err := findClosingQuote(params.content, params.offset, params.quote)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf("find closing quote: %w", err)
	}

	interior := params.content[params.offset+consts.IndexOne : closeIdx]

	if string(interior) != params.oldPath {
		return consts.IndexZero, consts.IndexZero, &RewriteError{
			Message: fmt.Sprintf(
				"include path %q not found inside quotes at YAML position",
				params.oldPath,
			),
		}
	}

	return params.offset + consts.IndexOne, closeIdx, nil
}

func findClosingQuote(content []byte, openIdx int, quote byte) (int, error) {
	idx := openIdx + consts.IndexOne

	for idx < len(content) {
		next, found := advanceQuotedIndex(content, idx, quote)

		if found {
			return next, nil
		}

		idx = next
	}

	return consts.IndexZero, &RewriteError{Message: "unterminated quoted include path"}
}

func advanceQuotedIndex(content []byte, idx int, quote byte) (next int, foundClose bool) {
	if isDoubleQuoteEscape(content, idx, quote) {
		return idx + consts.IndexTwo, false
	}

	if isSingleQuoteEscape(content, idx, quote) {
		return idx + consts.IndexTwo, false
	}

	if content[idx] == quote {
		return idx, true
	}

	return idx + consts.IndexOne, false
}

func isDoubleQuoteEscape(content []byte, idx int, quote byte) bool {
	return quote == '"' && content[idx] == '\\'
}

func isSingleQuoteEscape(content []byte, idx int, quote byte) bool {
	return content[idx] == quote && quote == '\'' &&
		idx+consts.IndexOne < len(content) && content[idx+consts.IndexOne] == '\''
}

func replaceByteSpan(params *replaceSpanParams) []byte {
	out := make(
		[]byte,
		consts.IndexZero,
		len(params.content)-params.end+params.start+len(params.value),
	)

	out = append(out, params.content[:params.start]...)
	out = append(out, params.value...)
	out = append(out, params.content[params.end:]...)

	return out
}

// parseTaskfileRoot unmarshals content into a YAML document node and returns its
// root mapping node. parseErrMsg and emptyErrMsg format the respective failures.
//
//nolint:gocritic // single-line sig for whitespace
func parseTaskfileRoot(
	content []byte,
	parseErr, emptyErr string,
) (doc *yaml.Node, docContent *yaml.Node, err error) {
	node := new(yaml.Node)

	err = yaml.Unmarshal(content, node)
	if err != nil {
		return nil, nil, &RewriteError{Message: fmt.Sprintf(parseErr, err)}
	}

	if len(node.Content) == consts.IndexZero {
		return nil, nil, &RewriteError{Message: emptyErr}
	}

	doc = node
	docContent = node.Content[consts.IndexZero]

	return doc, docContent, nil
}

// marshalAndValidate serializes node to YAML and re-parses the result to confirm
// it is well-formed. marshalErrMsg and validateErrMsg format the respective failures.
func marshalAndValidate(node *yaml.Node, marshalErrMsg, validateErrMsg string) ([]byte, error) {
	out, err := yamlfmt.Marshal(node)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf(marshalErrMsg, err)}
	}

	err = validateMarshaledYAML(out, validateErrMsg)
	if err != nil {
		return nil, fmt.Errorf("validate marshaled yaml: %w", err)
	}

	return out, nil
}

func validateMarshaledYAML(out []byte, errMsg string) error {
	var validateNode yaml.Node

	err := yaml.Unmarshal(out, &validateNode)
	if err != nil {
		return &RewriteError{Message: fmt.Sprintf(errMsg, err)}
	}

	return nil
}

func rewriteIncludePath(path string, sourceToDest map[string]string, fromDest string) string {
	normalized := filepath.ToSlash(path)

	if !strings.HasSuffix(normalized, consts.TaskfileSuffix) {
		return path
	}

	prefix, dir := splitRelativePrefix(strings.TrimSuffix(normalized, consts.TaskfileSuffix))
	iox.Discard(prefix)

	if dir == consts.Empty {
		return path
	}

	return rewriteWithDest(&rewriteParams{
		path:         path,
		sourceToDest: sourceToDest,
		fromDest:     fromDest,
		dir:          dir,
	})
}

func rewriteWithDest(params *rewriteParams) string {
	dest, ok := params.sourceToDest[params.dir]

	if !ok {
		return params.path
	}

	return destinationIncludePath(params.fromDest, dest, params.path)
}

// destinationIncludePath returns the include path from fromDest to dest's
// Taskfile.yml. On Rel failure it falls back to the original store path.
func destinationIncludePath(fromDest, dest, original string) string {
	if fromDest == consts.Empty {
		return original
	}

	target := filepath.Join(filepath.FromSlash(dest), taskfileSuffix)

	rel, err := filepath.Rel(filepath.FromSlash(fromDest), target)
	if err != nil {
		return original
	}

	return filepath.ToSlash(rel)
}

// splitRelativePrefix separates the leading ./ or ../ segments from the module
// directory. Namespaced modules such as internal/skipfiles keep their slash in
// the returned directory so it can be matched against source module names, and
// they sit one level deeper, so their siblings are reached through ../../.
func splitRelativePrefix(dir string) (prefix, moduleDir string) {
	prefix = consts.Empty

	if rest, ok := strings.CutPrefix(dir, dotSlash); ok {
		prefix += dotSlash

		dir = rest
	}

	return stripDotDotSlashes(prefix, dir)
}

func stripDotDotSlashes(prefix, dir string) (outPrefix, outDir string) {
	var prefixSb strings.Builder

	writeErr := iox.BuilderWriteString(&prefixSb, prefix)
	iox.Discard(writeErr)

	for {
		rest, ok := strings.CutPrefix(dir, dotDotSlash)

		if !ok {
			return finalizeRelativePrefix(prefixSb.String(), dir)
		}

		writeErr = iox.BuilderWriteString(&prefixSb, dotDotSlash)
		iox.Discard(writeErr)

		dir = rest
	}
}

func finalizeRelativePrefix(prefix, dir string) (outPrefix, outDir string) {
	if strings.Contains(dir, consts.PathParent) {
		return consts.Empty, consts.Empty
	}

	return prefix, dir
}

func findMappingValue(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return nil
	}

	for idx := consts.IndexZero; idx < len(mapNode.Content); idx += yamlMappingPairKeyValue {
		if isMatchingKey(mapNode.Content[idx], key) {
			return mapNode.Content[idx+consts.IndexOne]
		}
	}

	return nil
}

func isMatchingKey(keyNode *yaml.Node, key string) bool {
	return keyNode.Kind == yaml.ScalarNode && keyNode.Value == key
}

// moduleIncludePath returns the include taskfile path for a synced module,
// expressed relative to the directory that holds the aggregator Taskfile.
// When the aggregator sits at the workspace root the path is workspace-relative
// (for example taskfiles/go/Taskfile.yml); when it sits inside the target
// folder the path collapses to the module directory (for example go/Taskfile.yml).
func moduleIncludePath(rootDir, targetFolder, dest string) string {
	target := filepath.ToSlash(filepath.Join(targetFolder, dest, "Taskfile.yml"))

	if rootDir == consts.Empty || rootDir == consts.PathDot {
		return target
	}

	rel, err := filepath.Rel(filepath.FromSlash(rootDir), filepath.FromSlash(target))
	if err != nil {
		return target
	}

	return filepath.ToSlash(rel)
}

// includeDirForRoot returns the include-level dir so module tasks run from the
// workspace root. Empty or "." (aggregator at workspace root) yields "."; a
// nested aggregator such as "taskfiles" yields "..".
func includeDirForRoot(rootDir string) string {
	if rootDir == consts.Empty || rootDir == consts.PathDot {
		return consts.PathDot
	}

	rel, err := filepath.Rel(filepath.FromSlash(rootDir), consts.PathDot)
	if err != nil {
		return consts.PathDot
	}

	return filepath.ToSlash(rel)
}

// UpdateRootTaskfile merges managed module includes into the root Taskfile.
func UpdateRootTaskfile(content []byte, input *rootUpdateInput) ([]byte, error) {
	node, root, err := parseTaskfileRoot(
		content,
		"parse root Taskfile YAML: %v",
		"empty root Taskfile YAML",
	)
	if err != nil {
		return nil, fmt.Errorf(errParseTaskfileRoot, err)
	}

	out, err := marshalUpdatedRootTaskfile(node, root, input)
	if err != nil {
		return nil, fmt.Errorf("marshal updated root taskfile: %w", err)
	}

	return out, nil
}

func marshalUpdatedRootTaskfile(node, root *yaml.Node, input *rootUpdateInput) ([]byte, error) {
	setRootTaskfileVersion(root)

	err := applyRootUpdates(root, input)
	if err != nil {
		return nil, fmt.Errorf("apply root updates: %w", err)
	}

	out, err := marshalRootTaskfile(node)
	if err != nil {
		return nil, fmt.Errorf("marshal root taskfile: %w", err)
	}

	return out, nil
}

func marshalRootTaskfile(node *yaml.Node) ([]byte, error) {
	out, err := marshalAndValidate(
		node,
		"marshal root Taskfile YAML: %v",
		"validate root Taskfile YAML: %v",
	)
	if err != nil {
		return nil, fmt.Errorf(errMarshalAndValidate, err)
	}

	return out, nil
}

func applyRootUpdates(root *yaml.Node, input *rootUpdateInput) error {
	moduleVars, err := applyRootVars(root, input)
	if err != nil {
		return fmt.Errorf("apply root vars: %w", err)
	}

	err = applyRootIncludesAndTasks(root, &includesUpdateParams{
		includesNode: nil,
		existing:     nil,
		input:        input,
		moduleVars:   moduleVars,
	})
	if err != nil {
		return fmt.Errorf("apply root includes and tasks: %w", err)
	}

	return nil
}

func applyRootVars(root *yaml.Node, input *rootUpdIn) (yamlNodeMap, error) {
	moduleVars, err := moduleVarsByTask(input)
	if err != nil {
		return nil, fmt.Errorf("module vars by task: %w", err)
	}

	promotedVars := promotedModuleVarNames(input.Tasks, moduleVars)

	err = upsertRootPromotedVars(&promotedVarParams{
		root: root, tasks: input.Tasks, moduleVars: moduleVars, promotedVars: promotedVars,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert root promoted vars: %w", err)
	}

	return moduleVars, nil
}

func applyRootIncludesAndTasks(root *yaml.Node, params *includesUpdateParams) error {
	err := populateIncludesState(root, params)
	if err != nil {
		return fmt.Errorf("populate includes state: %w", err)
	}

	err = upsertManagedIncludes(params)
	if err != nil {
		return fmt.Errorf("upsert managed includes: %w", err)
	}

	err = updateGeneratedRootTasks(root, params.input)
	if err != nil {
		return fmt.Errorf("update generated root tasks: %w", err)
	}

	return nil
}

func populateIncludesState(root *yaml.Node, params *includesUpdateParams) error {
	includesNode, existing, err := prepareIncludesNode(root, params.input)
	if err != nil {
		return fmt.Errorf("prepare includes node: %w", err)
	}

	params.includesNode = includesNode
	params.existing = existing

	return nil
}

// findOrCreateMappingNode returns the mapping node under key on root, creating an
// empty one when absent. It errors when the node exists but isn't a mapping.
func findOrCreateMappingNode(root *yaml.Node, key, errMsg string) (*yaml.Node, error) {
	node := findMappingValue(root, key)

	if node == nil {
		node = newYAMLMappingNode()
		appendMappingPair(root, yamlScalar(key), node)
	}

	if node.Kind != yaml.MappingNode {
		return nil, &RewriteError{Message: errMsg}
	}

	return node, nil
}

func prepareIncludesNode(root *yaml.Node, input *rootUpdIn) (*yaml.Node, yamlNodeMap, error) {
	includesNode, err := findOrCreateMappingNode(
		root,
		keyIncludes,
		"root Taskfile includes must be a mapping",
	)
	if err != nil {
		return nil, nil, fmt.Errorf("find or create includes mapping node: %w", err)
	}

	existing := existingIncludeEntries(includesNode)
	pruneStaleIncludes(includesNode, existing, input)

	return includesNode, existing, nil
}

func pruneStaleIncludes(incNode *yaml.Node, exist yamlNodeMap, input *rootUpdIn) {
	pruneRemovedManagedIncludes(&pruneIncludesParams{
		includesNode: incNode,
		existing:     exist,
		managedSet:   managedTaskSet(input.Tasks),
		managedTasks: input.ManagedTasks,
	})
}

func managedTaskSet(tasks []string) map[string]struct{} {
	managedSet := make(map[string]struct{}, len(tasks))

	for i := range tasks {
		managedSet[tasks[i]] = struct{}{}
	}

	return managedSet
}

func existingIncludeEntries(includesNode *yaml.Node) map[string]*yaml.Node {
	existing := make(map[string]*yaml.Node)

	for idx := consts.IndexZero; idx < len(includesNode.Content); idx += yamlMappingPairKeyValue {
		keyNode := includesNode.Content[idx]

		existing[keyNode.Value] = includesNode.Content[idx+consts.IndexOne]
	}

	return existing
}

func pruneRemovedManagedIncludes(params *pruneIncludesParams) {
	for alias := range params.existing {
		managedVal, managed := params.managedSet[alias]
		iox.Discard(managedVal)

		if managed {
			continue
		}

		if containsString(params.managedTasks, alias) {
			deleteMappingKey(params.includesNode, alias)
		}
	}
}

func upsertManagedIncludes(params *includesUpdateParams) error {
	for i := range params.input.Tasks {
		task := params.input.Tasks[i]

		err := upsertOneInclude(&includeUpsertParams{
			includesNode: params.includesNode,
			input:        params.input,
			existing:     params.existing,
			moduleVars:   params.moduleVars,
			task:         task,
		})
		if err != nil {
			return fmt.Errorf("upsert include for task %q: %w", task, err)
		}
	}

	return nil
}

func upsertOneInclude(params *includeUpsertParams) error {
	path, err := includePathForTask(params)
	if err != nil {
		return fmt.Errorf("include path for task %q: %w", params.task, err)
	}

	err = upsertIncludeAtPath(params, path)
	if err != nil {
		return fmt.Errorf("upsert include at path: %w", err)
	}

	return nil
}

func upsertIncludeAtPath(params *includeUpsertParams, path string) error {
	entry, found := params.existing[params.task]

	if !found {
		appendNewInclude(params, path, params.moduleVars[params.task])

		return nil
	}

	err := updateExistingIncludeEntry(params, entry, path)
	if err != nil {
		return fmt.Errorf("update existing include entry for task %q: %w", params.task, err)
	}

	return nil
}

func includePathForTask(params *includeUpsertParams) (string, error) {
	dest, ok := params.input.DestByTask[params.task]

	if !ok {
		return consts.Empty, &RewriteError{
			Message: fmt.Sprintf("missing destination for task %q", params.task),
		}
	}

	path := moduleIncludePath(params.input.RootTaskfileDir, params.input.TargetFolder, dest)

	return path, nil
}

func updateExistingIncludeEntry(params *includeUpsertParams, entry *yaml.Node, path string) error {
	err := updateExistingInclude(&existingIncludeParams{
		entry:        entry,
		path:         path,
		dir:          includeDirForRoot(params.input.RootTaskfileDir),
		moduleVars:   params.moduleVars[params.task],
		managedTasks: params.input.ManagedTasks,
		task:         params.task,
	})
	if err != nil {
		return fmt.Errorf("update existing include for task %q: %w", params.task, err)
	}

	return nil
}

func appendNewInclude(params *includeUpsertParams, path string, moduleVars *yaml.Node) {
	entry := newIncludeEntry(path, includeDirForRoot(params.input.RootTaskfileDir), moduleVars)
	appendMappingPair(params.includesNode, yamlScalar(params.task), entry)
}

func updateExistingInclude(params *existingIncludeParams) error {
	err := ensureManagedInclude(params)
	if err != nil {
		return fmt.Errorf("ensure managed include for task %q: %w", params.task, err)
	}

	setIncludePath(params.entry, params.path, params.dir)
	mergeIncludeVars(params.entry, params.moduleVars)

	return nil
}

func ensureManagedInclude(params *existingIncludeParams) error {
	managed := &managedIncludeParams{
		entry:        params.entry,
		expectedPath: params.path,
		managedTasks: params.managedTasks,
		task:         params.task,
	}

	if isManagedInclude(managed) {
		return nil
	}

	return &RewriteError{
		Message: fmt.Sprintf(
			"include alias %q already exists and is not managed by TaskOtter",
			params.task,
		),
	}
}

func isManagedInclude(params *managedIncludeParams) bool {
	taskfileNode := findMappingValue(params.entry, keyTaskfile)

	if taskfileNode != nil {
		return taskfileNode.Value == params.expectedPath
	}

	if params.entry.Kind == yaml.ScalarNode {
		return params.entry.Value == params.expectedPath
	}

	return containsString(params.managedTasks, params.task)
}

func setIncludePath(entry *yaml.Node, path, dir string) {
	setOrAppendMappingScalar(entry, keyTaskfile, path)
	setOrAppendMappingScalar(entry, keyDir, dir)
}

func setOrAppendMappingScalar(entry *yaml.Node, key, value string) {
	node := findMappingValue(entry, key)

	if node == nil {
		appendMappingPair(entry, yamlScalar(key), yamlScalar(value))

		return
	}

	node.Value = value
}

func extractVarsNode(content []byte) (*yaml.Node, error) {
	root, err := parseModuleTaskfileNode(content)
	if err != nil {
		return nil, fmt.Errorf("parse module taskfile node: %w", err)
	}

	varsNode := findMappingValue(root, keyVars)

	if !hasModuleVars(varsNode) {
		return nil, errNoModuleVars
	}

	return cloneYAMLNode(varsNode), nil
}

func hasModuleVars(varsNode *yaml.Node) bool {
	return varsNode != nil && varsNode.Kind == yaml.MappingNode &&
		len(varsNode.Content) != consts.IndexZero
}

func parseModuleTaskfileNode(content []byte) (*yaml.Node, error) {
	if len(content) == consts.IndexZero {
		return nil, errNoModuleVars
	}

	var node yaml.Node

	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf("parse module Taskfile YAML: %v", err)}
	}

	if len(node.Content) == consts.IndexZero {
		return nil, errNoModuleVars
	}

	return node.Content[consts.IndexZero], nil
}

func moduleVarsByTask(input *rootUpdateInput) (map[string]*yaml.Node, error) {
	out := make(map[string]*yaml.Node, len(input.Tasks))

	for i := range input.Tasks {
		task := input.Tasks[i]

		err := addTaskModuleVar(out, input, task)
		if err != nil {
			return nil, fmt.Errorf("add task module var for %q: %w", task, err)
		}
	}

	return out, nil
}

func addTaskModuleVar(out map[string]*yaml.Node, input *rootUpdateInput, task string) error {
	moduleVars, include, err := moduleVarForTask(input.ModuleTaskfiles[task], task)

	if errors.Is(err, errNoModuleVars) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("module var for task %q: %w", task, err)
	}

	if include {
		out[task] = moduleVars
	}

	return nil
}

func moduleVarForTask(content []byte, task string) (*yaml.Node, bool, error) {
	moduleVars, ok, err := tryExtractVarsNode(content)

	if errors.Is(err, errNoModuleVars) {
		return nil, false, errNoModuleVars
	}

	if err != nil {
		return nil, false, fmt.Errorf("try extract vars node for task %q: %w", task, err)
	}

	return moduleVars, ok, nil
}

func tryExtractVarsNode(content []byte) (*yaml.Node, bool, error) {
	moduleVars, err := extractVarsNode(content)
	if err == nil {
		return moduleVars, true, nil
	}

	if errors.Is(err, errNoModuleVars) {
		return nil, false, errNoModuleVars
	}

	return nil, false, fmt.Errorf("extract vars node: %w", err)
}

func promotedModuleVarNames(tasks []string, modVars yamlNodeMap) strSet {
	promoted := make(map[string]struct{})

	for i := range tasks {
		for key := range varKeySet(modVars[tasks[i]]) {
			promoted[key] = struct{}{}
		}
	}

	return promoted
}

func varKeySet(varsNode *yaml.Node) map[string]struct{} {
	keys := make(map[string]struct{})

	if varsNode == nil || varsNode.Kind != yaml.MappingNode {
		return keys
	}

	for idx := consts.IndexZero; idx < len(varsNode.Content); idx += yamlMappingPairKeyValue {
		keys[varsNode.Content[idx].Value] = struct{}{}
	}

	return keys
}

func upsertRootPromotedVars(params *promotedVarParams) error {
	if len(params.promotedVars) == consts.IndexZero {
		return nil
	}

	rootVars, err := rootVarsMappingNode(params.root)
	if err != nil {
		return fmt.Errorf("root vars mapping node: %w", err)
	}

	addPromotedVarsToRoot(rootVars, params)

	return nil
}

func rootVarsMappingNode(root *yaml.Node) (*yaml.Node, error) {
	rootVars, err := findOrCreateMappingNode(root, keyVars, "root Taskfile vars must be a mapping")
	if err != nil {
		return nil, fmt.Errorf("find or create vars mapping node: %w", err)
	}

	return rootVars, nil
}

func addPromotedVarsToRoot(rootVars *yaml.Node, params *promotedVarParams) {
	existing := mappingKeys(rootVars)

	keys := sortedKeys(params.promotedVars)

	for i := range keys {
		addMissingPromotedVar(&addPromotedVarParams{
			rootVars: rootVars, tasks: params.tasks, moduleVars: params.moduleVars,
			key: keys[i], existing: existing,
		})
	}
}

func addMissingPromotedVar(params *addPromotedVarParams) {
	existingVal, ok := params.existing[params.key]
	iox.Discard(existingVal)

	if ok {
		return
	}

	value := firstVarValue(params.tasks, params.moduleVars, params.key)

	if value == nil {
		return
	}

	appendMappingPair(params.rootVars, yamlScalar(params.key), value)
}

func firstVarValue(tasks []string, moduleVarsByTask map[string]*yaml.Node, key string) *yaml.Node {
	for i := range tasks {
		value := varValueIn(moduleVarsByTask[tasks[i]], key)

		if value != nil {
			return value
		}
	}

	return nil
}

func varValueIn(varsNode *yaml.Node, key string) *yaml.Node {
	if varsNode == nil || varsNode.Kind != yaml.MappingNode {
		return nil
	}

	for idx := consts.IndexZero; idx < len(varsNode.Content); idx += yamlMappingPairKeyValue {
		if varsNode.Content[idx].Value == key {
			return cloneYAMLNode(varsNode.Content[idx+consts.IndexOne])
		}
	}

	return nil
}

func mergeIncludeVars(entry, moduleVars *yaml.Node) {
	if moduleVars == nil || moduleVars.Kind != yaml.MappingNode {
		return
	}

	existingVars, ok := resolveExistingVars(entry, moduleVars)

	if !ok {
		return
	}

	mergeModuleVarsInto(existingVars, moduleVars)
}

func mergeModuleVarsInto(existVars, modVars *yaml.Node) {
	for idx := consts.IndexZero; idx < len(modVars.Content); idx += yamlMappingPairKeyValue {
		mergeOneModuleVar(&mergeModuleVarParams{
			existingVars: existVars,
			key:          modVars.Content[idx].Value,
		})
	}
}

// resolveExistingVars returns the entry's existing vars mapping, creating one from
// moduleVars when absent. ok is false when there is nothing left to merge (either a
// fresh vars node was just created, or the existing one isn't a mapping).
func resolveExistingVars(entry, modVars *yaml.Node) (*yaml.Node, bool) {
	existingVars := findMappingValue(entry, keyVars)

	if existingVars == nil {
		appendMappingPair(entry, yamlScalar(keyVars), includeVarsNode(modVars))

		return nil, false
	}

	if existingVars.Kind != yaml.MappingNode {
		return nil, false
	}

	return existingVars, true
}

func mergeOneModuleVar(params *mergeModuleVarParams) {
	setMappingValue(params.existingVars, params.key, rootVarReference(params.key))
}

func includeVarsNode(moduleVars *yaml.Node) *yaml.Node {
	out := newYAMLMappingNode()

	for idx := consts.IndexZero; idx < len(moduleVars.Content); idx += yamlMappingPairKeyValue {
		key := moduleVars.Content[idx].Value

		appendMappingPair(
			out,
			cloneYAMLNode(moduleVars.Content[idx]),
			rootVarReference(key),
		)
	}

	return out
}

func rootVarReference(key string) *yaml.Node {
	return yamlScalar("{{." + key + "}}")
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	data, err := yaml.Marshal(node)
	if err != nil {
		return nil
	}

	var out yaml.Node

	err = yaml.Unmarshal(data, &out)

	if err != nil || len(out.Content) == consts.IndexZero {
		return nil
	}

	return out.Content[consts.IndexZero]
}

func newIncludeEntry(path, dir string, modVars *yaml.Node) *yaml.Node {
	entry := newYAMLMappingNode()
	appendMappingPair(entry, yamlScalar(keyTaskfile), yamlScalar(path))
	appendMappingPair(entry, yamlScalar(keyDir), yamlScalar(dir))

	if modVars != nil {
		appendMappingPair(entry, yamlScalar(keyVars), includeVarsNode(modVars))
	}

	return entry
}

func updateGeneratedRootTasks(root *yaml.Node, input *rootUpdateInput) error {
	if !hasGeneratedRootTaskUpdates(input) {
		return nil
	}

	tasksNode, err := findOrCreateMappingNode(
		root,
		keyTasks,
		"root Taskfile tasks must be a mapping",
	)
	if err != nil {
		return fmt.Errorf("find or create tasks mapping node: %w", err)
	}

	applyGeneratedRootTaskUpdates(tasksNode, input)

	return nil
}

func hasGeneratedRootTaskUpdates(input *rootUpdateInput) bool {
	return len(input.GeneratedTasks) != consts.IndexZero ||
		len(input.ManagedRootTasks) != consts.IndexZero
}

func applyGeneratedRootTaskUpdates(tasksNode *yaml.Node, input *rootUpdateInput) {
	generatedSet := generatedTaskSet(input.GeneratedTasks)

	removeStaleGenTasks(tasksNode, input.ManagedRootTasks, generatedSet)
	applyGeneratedTasks(tasksNode, input.GeneratedTasks)
}

func generatedTaskSet(generatedTasks []generatedRootTask) map[string]struct{} {
	generatedSet := make(map[string]struct{}, len(generatedTasks))

	for i := range generatedTasks {
		generatedSet[generatedTasks[i].Name] = struct{}{}
	}

	return generatedSet
}

func removeStaleGenTasks(tskNode *yaml.Node, mngTasks []string, genSet strSet) {
	for i := range mngTasks {
		old := mngTasks[i]

		generatedVal, stillGenerated := genSet[old]
		iox.Discard(generatedVal)

		if stillGenerated {
			continue
		}

		deleteMappingKey(tskNode, old)
	}
}

func applyGeneratedTasks(tasksNode *yaml.Node, generatedTasks []generatedRootTask) {
	for i := range generatedTasks {
		generated := &generatedTasks[i]

		deleteMappingKey(tasksNode, generated.Name)
		appendMappingPair(tasksNode, yamlScalar(generated.Name), newGeneratedTaskEntry(generated))
	}
}

func newGeneratedTaskEntry(generated *generatedRootTask) *yaml.Node {
	entry := newYAMLMappingNode()
	appendMappingPair(
		entry,
		yamlScalar("desc"),
		yamlScalar("Run "+generated.Name+" for synced TaskOtter modules"),
	)
	appendMappingPair(entry, yamlScalar("cmds"), generatedTaskCommands(generated))

	return entry
}

func generatedTaskCommands(generated *generatedRootTask) *yaml.Node {
	cmds := newYAMLSequenceNode()

	for i := range generated.Modules {
		module := generated.Modules[i]

		cmd := newYAMLMappingNode()
		appendMappingPair(cmd, yamlScalar("task"), yamlScalar(module+":"+generated.Name))

		cmds.Content = append(cmds.Content, cmd)
	}

	return cmds
}

func newYAMLNode(kind yaml.Kind) *yaml.Node {
	return &yaml.Node{
		Kind:        kind,
		Style:       consts.IndexZero,
		Tag:         consts.Empty,
		Value:       consts.Empty,
		Anchor:      consts.Empty,
		Alias:       nil,
		Content:     nil,
		HeadComment: consts.Empty,
		LineComment: consts.Empty,
		FootComment: consts.Empty,
		Line:        consts.IndexZero,
		Column:      consts.IndexZero,
	}
}

func newYAMLMappingNode() *yaml.Node {
	return newYAMLNode(yaml.MappingNode)
}

func newYAMLSequenceNode() *yaml.Node {
	return newYAMLNode(yaml.SequenceNode)
}

func yamlScalar(value string) *yaml.Node {
	return &yaml.Node{
		Kind:        yaml.ScalarNode,
		Style:       consts.IndexZero,
		Tag:         consts.Empty,
		Value:       value,
		Anchor:      consts.Empty,
		Alias:       nil,
		Content:     nil,
		HeadComment: consts.Empty,
		LineComment: consts.Empty,
		FootComment: consts.Empty,
		Line:        consts.IndexZero,
		Column:      consts.IndexZero,
	}
}

func appendMappingPair(mapNode, key, value *yaml.Node) {
	mapNode.Content = append(mapNode.Content, key, value)
}

func mappingKeys(mapNode *yaml.Node) map[string]struct{} {
	keys := make(map[string]struct{}, len(mapNode.Content)/yamlMappingPairKeyValue)

	for idx := consts.IndexZero; idx < len(mapNode.Content); idx += yamlMappingPairKeyValue {
		keys[mapNode.Content[idx].Value] = struct{}{}
	}

	return keys
}

func setMappingValue(mapNode *yaml.Node, key string, value *yaml.Node) {
	for idx := consts.IndexZero; idx < len(mapNode.Content); idx += yamlMappingPairKeyValue {
		if mapNode.Content[idx].Value == key {
			mapNode.Content[idx+consts.IndexOne] = value

			return
		}
	}

	appendMappingPair(mapNode, yamlScalar(key), value)
}

func setRootTaskfileVersion(root *yaml.Node) {
	setMappingValue(root, "version", taskfileVersionScalar(rootTaskfileVersion))
}

func taskfileVersionScalar(value string) *yaml.Node {
	node := yamlScalar(value)

	node.Style = yaml.DoubleQuotedStyle

	return node
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, consts.IndexZero, len(m))

	for key := range m {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

func deleteMappingKey(mapNode *yaml.Node, key string) {
	for idx := consts.IndexZero; idx < len(mapNode.Content); idx += yamlMappingPairKeyValue {
		if mapNode.Content[idx].Value == key {
			mapNode.Content = append(
				mapNode.Content[:idx],
				mapNode.Content[idx+yamlMappingPairKeyValue:]...,
			)

			return
		}
	}
}

func containsString(list []string, target string) bool {
	return slices.Contains(list, target)
}
