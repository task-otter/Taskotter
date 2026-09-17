// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	yaml "go.yaml.in/yaml/v3"
)

type (
	rootTextEdit struct {
		replacement []byte
		start       int
		end         int
		order       int
	}

	rawModuleVars = map[string]map[string][]byte

	rootPatchParams struct {
		originalRoot *yaml.Node
		desiredRoot  *yaml.Node
		input        *rootUpdateInput
		rawVars      rawModuleVars
		content      []byte
	}

	mappingPair struct {
		key   *yaml.Node
		value *yaml.Node
	}
)

func rawModuleVarsByTask(input *rootUpdateInput) (rawModuleVars, error) {
	out := make(rawModuleVars, len(input.Tasks))

	for i := range input.Tasks {
		task := input.Tasks[i]
		content := input.ModuleTaskfiles[task]

		root, err := parseModuleTaskfileNode(content)
		if err != nil {
			if errors.Is(err, errNoModuleVars) {
				continue
			}

			return nil, fmt.Errorf("parse module Taskfile for %q: %w", task, err)
		}

		varsNode := findMappingValue(root, keyVars)

		if !hasModuleVars(varsNode) {
			continue
		}

		values, err := rawVarsFromNode(content, root, varsNode)
		if err != nil {
			return nil, fmt.Errorf("raw vars for %q: %w", task, err)
		}

		out[task] = values
	}

	return out, nil
}

func rawVarsFromNode(content []byte, root, varsNode *yaml.Node) (map[string][]byte, error) {
	rootEnd := topLevelNodeEnd(content, root, varsNode)
	out := make(map[string][]byte, len(varsNode.Content)/yamlMappingPairKeyValue)

	for idx := consts.IndexZero; idx < len(varsNode.Content); idx += yamlMappingPairKeyValue {
		key := varsNode.Content[idx]

		start, end, err := sourceNodeSpan(content, varsNode, idx, rootEnd)
		if err != nil {
			return nil, fmt.Errorf("source span for %q: %w", key.Value, err)
		}

		out[key.Value] = bytes.Clone(content[start:end])
	}

	return out, nil
}

func patchRootTaskfile(
	content []byte,
	originalRoot, desiredRoot *yaml.Node,
	input *rootUpdateInput,
	rawVars rawModuleVars,
) ([]byte, error) {
	params := &rootPatchParams{
		content: content, originalRoot: originalRoot, desiredRoot: desiredRoot,
		input: input, rawVars: rawVars,
	}

	edits, err := rootTextEdits(params)
	if err != nil {
		return nil, err
	}

	normalizeRootEditLineEndings(edits, rootLineEnding(content))

	out := applyRootTextEdits(content, edits)

	_, _, err = parseTaskfileRoot(
		out,
		"validate patched root Taskfile YAML: %v",
		"empty patched root Taskfile YAML",
	)
	if err != nil {
		return nil, fmt.Errorf("validate patched root Taskfile: %w", err)
	}

	return out, nil
}

func normalizeRootEditLineEndings(edits []rootTextEdit, ending []byte) {
	if bytes.Equal(ending, []byte("\n")) {
		return
	}

	for i := range edits {
		replacement := bytes.ReplaceAll(edits[i].replacement, []byte("\r\n"), []byte("\n"))

		edits[i].replacement = bytes.ReplaceAll(replacement, []byte("\n"), ending)
	}
}

func rootLineEnding(content []byte) []byte {
	if bytes.Contains(content, []byte("\r\n")) {
		return []byte("\r\n")
	}

	return []byte("\n")
}

func rootTextEdits(params *rootPatchParams) ([]rootTextEdit, error) {
	edits := make([]rootTextEdit, consts.IndexZero)

	edits, err := appendVersionEdit(edits, params)
	if err != nil {
		return nil, fmt.Errorf("version edit: %w", err)
	}

	for _, section := range []string{keyVars, keyIncludes, keyTasks} {
		sectionEdits, err := editsForRootSection(params, section)
		if err != nil {
			return nil, fmt.Errorf("%s edits: %w", section, err)
		}

		edits = append(edits, sectionEdits...)
	}

	for i := range edits {
		edits[i].order = i
	}

	return edits, nil
}

func appendVersionEdit(edits []rootTextEdit, params *rootPatchParams) ([]rootTextEdit, error) {
	original := findMappingPair(params.originalRoot, "version")
	desired := findMappingPair(params.desiredRoot, "version")

	if desired.value == nil {
		return edits, nil
	}

	if original.value == nil {
		return append(edits, rootTextEdit{
			start:       documentBodyStart(params.content),
			end:         documentBodyStart(params.content),
			replacement: []byte("version: \"" + desired.value.Value + "\"\n"),
		}), nil
	}

	if original.value.Value == desired.value.Value {
		return edits, nil
	}

	start, end, err := scalarSourceSpan(params.content, original.key, original.value)
	if err != nil {
		return nil, err
	}

	return append(edits, rootTextEdit{
		start: start, end: end,
		replacement: []byte(`"` + desired.value.Value + `"`),
	}), nil
}

func editsForRootSection(params *rootPatchParams, section string) ([]rootTextEdit, error) {
	original := findMappingPair(params.originalRoot, section)
	desired := findMappingPair(params.desiredRoot, section)

	if desired.value == nil {
		return nil, nil
	}

	if original.value == nil {
		replacement, err := renderRootSection(
			section,
			desired.value,
			params.rawVars,
			params.input.Tasks,
		)
		if err != nil {
			return nil, err
		}

		start := len(params.content)

		if start != consts.IndexZero && params.content[start-consts.IndexOne] != '\n' {
			replacement = append([]byte{'\n'}, replacement...)
		}

		return []rootTextEdit{{start: start, end: start, replacement: replacement}}, nil
	}

	if !isBlockMapping(params.content, original.key, original.value) {
		start, end := rootEntrySpan(params.content, params.originalRoot, original.key)

		replacement, err := renderRootSection(
			section,
			desired.value,
			params.rawVars,
			params.input.Tasks,
		)
		if err != nil {
			return nil, err
		}

		return []rootTextEdit{{start: start, end: end, replacement: replacement}}, nil
	}

	switch section {
	case keyVars:
		return editsForRootVars(params, original.value, desired.value)
	case keyIncludes:
		return editsForManagedMapping(
			params,
			original.value,
			desired.value,
			managedIncludeNames(params.input),
		)
	case keyTasks:
		return editsForManagedMapping(
			params,
			original.value,
			desired.value,
			managedRootTaskNames(params.input),
		)
	default:
		return nil, nil
	}
}

func editsForRootVars(
	params *rootPatchParams,
	original, desired *yaml.Node,
) ([]rootTextEdit, error) {
	originalKeys := mappingKeys(original)
	newPairs := make([]mappingPair, consts.IndexZero)

	for idx := consts.IndexZero; idx < len(desired.Content); idx += yamlMappingPairKeyValue {
		key := desired.Content[idx].Value

		if _, exists := originalKeys[key]; exists {
			continue
		}

		newPairs = append(newPairs, mappingPair{
			key: desired.Content[idx], value: desired.Content[idx+consts.IndexOne],
		})
	}

	if len(newPairs) == consts.IndexZero {
		return nil, nil
	}

	sectionEnd := topLevelNodeEnd(params.content, params.originalRoot, original)
	insertAt := trimBlankLinesBackward(params.content, sectionEnd)

	text, err := renderRootVarPairs(newPairs, params.rawVars, params.input.Tasks)
	if err != nil {
		return nil, err
	}

	return []rootTextEdit{{start: insertAt, end: insertAt, replacement: text}}, nil
}

func editsForManagedMapping(
	params *rootPatchParams,
	original, desired *yaml.Node,
	managed map[string]struct{},
) ([]rootTextEdit, error) {
	edits := make([]rootTextEdit, consts.IndexZero)
	sectionEnd := topLevelNodeEnd(params.content, params.originalRoot, original)
	originalKeys := mappingKeys(original)

	for idx := consts.IndexZero; idx < len(original.Content); idx += yamlMappingPairKeyValue {
		key := original.Content[idx].Value

		if _, isManaged := managed[key]; !isManaged {
			continue
		}

		newPair := desiredPair(desired, key)
		start, end := mappingEntrySpan(params.content, original, idx, sectionEnd)

		if newPair.value == nil {
			edits = append(edits, rootTextEdit{start: start, end: end})

			continue
		}

		replacement, err := renderMappingPair(newPair)
		if err != nil {
			return nil, fmt.Errorf("render managed entry %q: %w", key, err)
		}

		if bytes.Equal(params.content[start:end], replacement) {
			continue
		}

		edits = append(edits, rootTextEdit{start: start, end: end, replacement: replacement})
	}

	newEntries := make([]mappingPair, consts.IndexZero)

	for idx := consts.IndexZero; idx < len(desired.Content); idx += yamlMappingPairKeyValue {
		key := desired.Content[idx].Value

		if _, already := originalKeys[key]; already {
			continue
		}

		if _, isManaged := managed[key]; !isManaged {
			continue
		}

		newEntries = append(newEntries, mappingPair{
			key: desired.Content[idx], value: desired.Content[idx+consts.IndexOne],
		})
	}

	if len(newEntries) != consts.IndexZero {
		insertAt := trimBlankLinesBackward(params.content, sectionEnd)
		text := make([]byte, consts.IndexZero)

		for i := range newEntries {
			entry, err := renderMappingPair(newEntries[i])
			if err != nil {
				return nil, fmt.Errorf(
					"render new managed entry %q: %w",
					newEntries[i].key.Value,
					err,
				)
			}

			text = append(text, entry...)
		}

		edits = append(edits, rootTextEdit{start: insertAt, end: insertAt, replacement: text})
	}

	return edits, nil
}

func managedIncludeNames(input *rootUpdateInput) map[string]struct{} {
	return unionNames(input.Tasks, input.ManagedTasks)
}

func managedRootTaskNames(input *rootUpdateInput) map[string]struct{} {
	names := unionNames(input.ManagedRootTasks, nil)

	for i := range input.GeneratedTasks {
		names[input.GeneratedTasks[i].Name] = struct{}{}
	}

	return names
}

func unionNames(first, second []string) map[string]struct{} {
	out := make(map[string]struct{}, len(first)+len(second))

	for _, name := range first {
		out[name] = struct{}{}
	}

	for _, name := range second {
		out[name] = struct{}{}
	}

	return out
}

func desiredPair(node *yaml.Node, key string) mappingPair {
	for idx := consts.IndexZero; idx < len(node.Content); idx += yamlMappingPairKeyValue {
		if node.Content[idx].Value == key {
			return mappingPair{key: node.Content[idx], value: node.Content[idx+consts.IndexOne]}
		}
	}

	return mappingPair{}
}

func findMappingPair(node *yaml.Node, key string) mappingPair {
	value := findMappingValue(node, key)

	if value == nil {
		return mappingPair{}
	}

	for idx := consts.IndexZero; idx < len(node.Content); idx += yamlMappingPairKeyValue {
		if node.Content[idx].Value == key {
			return mappingPair{key: node.Content[idx], value: value}
		}
	}

	return mappingPair{}
}

func renderRootSection(
	section string,
	node *yaml.Node,
	rawVars rawModuleVars,
	tasks []string,
) ([]byte, error) {
	out := []byte(section + ":\n")
	pairs := make([]mappingPair, consts.IndexZero, len(node.Content)/yamlMappingPairKeyValue)

	for idx := consts.IndexZero; idx < len(node.Content); idx += yamlMappingPairKeyValue {
		pairs = append(
			pairs,
			mappingPair{key: node.Content[idx], value: node.Content[idx+consts.IndexOne]},
		)
	}

	if section == keyVars {
		text, err := renderRootVarPairs(pairs, rawVars, tasks)
		if err != nil {
			return nil, err
		}

		return append(out, text...), nil
	}

	for i := range pairs {
		text, err := renderMappingPair(pairs[i])
		if err != nil {
			return nil, err
		}

		out = append(out, text...)
	}

	return out, nil
}

func renderRootVarPairs(
	pairs []mappingPair,
	rawVars rawModuleVars,
	tasks []string,
) ([]byte, error) {
	out := make([]byte, consts.IndexZero)

	for i := range pairs {
		var raw []byte

		if tasks != nil {
			raw = firstRawVar(tasks, rawVars, pairs[i].key.Value)
		}

		if len(raw) != consts.IndexZero &&
			(pairs[i].value.Kind != yaml.ScalarNode || strings.Contains(string(raw), "| default")) {

			out = append(out, renderRawRootVarPair(pairs[i], raw)...)

			continue
		}

		text, err := renderMappingPair(pairs[i])
		if err != nil {
			return nil, err
		}

		out = append(out, text...)
	}

	return out, nil
}

func renderRawRootVarPair(pair mappingPair, raw []byte) []byte {
	raw = trimOneLineEnding(raw)

	if pair.value.Kind != yaml.ScalarNode && pair.value.Style&yaml.FlowStyle == consts.IndexZero {
		out := []byte("  " + pair.key.Value + ":\n")

		out = append(out, bytes.Repeat([]byte{' '}, 4)...)
		out = append(out, raw...)
		out = append(out, '\n')

		return out
	}

	out := []byte("  " + pair.key.Value + ": ")

	out = append(out, raw...)
	out = append(out, '\n')

	return out
}

func firstRawVar(tasks []string, rawVars rawModuleVars, key string) []byte {
	for _, task := range tasks {
		if values, ok := rawVars[task]; ok {
			if value, ok := values[key]; ok {
				return value
			}
		}
	}

	return nil
}

func renderMappingPair(pair mappingPair) ([]byte, error) {
	mapNode := newYAMLMappingNode()
	appendMappingPair(mapNode, cloneYAMLNode(pair.key), cloneYAMLNode(pair.value))

	encoded, err := marshalNode(mapNode, "marshal mapping pair: %v")
	if err != nil {
		return nil, err
	}

	body := bytes.TrimPrefix(encoded, []byte("---\n"))

	return indentLines(body, 2), nil
}

func indentLines(content []byte, spaces int) []byte {
	prefix := bytes.Repeat([]byte{' '}, spaces)
	out := make([]byte, consts.IndexZero, len(content)+len(prefix))

	for len(content) != consts.IndexZero {
		out = append(out, prefix...)

		idx := bytes.IndexByte(content, '\n')

		if idx < consts.IndexZero {
			out = append(out, content...)

			break
		}

		out = append(out, content[:idx+consts.IndexOne]...)
		content = content[idx+consts.IndexOne:]
	}

	return out
}

func applyRootTextEdits(content []byte, edits []rootTextEdit) []byte {
	slices.SortFunc(edits, func(left, right rootTextEdit) int {
		if left.start != right.start {
			return right.start - left.start
		}

		return right.order - left.order
	})

	out := bytes.Clone(content)

	for _, edit := range edits {
		out = replaceByteSpan(&replaceSpanParams{
			content: out, start: edit.start, end: edit.end, value: string(edit.replacement),
		})
	}

	return out
}

func sourceNodeSpan(content []byte, parent *yaml.Node, pairIndex, parentEnd int) (int, int, error) {
	key := parent.Content[pairIndex]
	value := parent.Content[pairIndex+consts.IndexOne]

	start, err := nodeStartOffset(content, value)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, err
	}

	if value.Kind == yaml.ScalarNode {
		end, err := scalarSourceEnd(content, key, value)
		if err != nil {
			return consts.IndexZero, consts.IndexZero, err
		}

		return start, trimBlankLinesBackward(content, end), nil
	}

	end := parentEnd

	if pairIndex+yamlMappingPairKeyValue < len(parent.Content) {
		next := parent.Content[pairIndex+yamlMappingPairKeyValue]

		end, err = lineStartOffset(content, next.Line)
		if err != nil {
			return consts.IndexZero, consts.IndexZero, err
		}
	}

	return start, trimBlankLinesBackward(content, end), nil
}

func scalarSourceSpan(content []byte, key, value *yaml.Node) (int, int, error) {
	start, err := nodeStartOffset(content, value)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, err
	}

	end, err := scalarSourceEnd(content, key, value)

	return start, end, err
}

func scalarSourceEnd(content []byte, key, value *yaml.Node) (int, error) {
	start, err := nodeStartOffset(content, value)
	if err != nil {
		return consts.IndexZero, err
	}

	if value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != consts.IndexZero {
		return blockScalarEnd(content, key, value)
	}

	if quote, quoted := quoteForYAMLStyle(value.Style); quoted {
		close, err := findClosingQuote(content, start, quote)
		if err != nil {
			return consts.IndexZero, err
		}

		return close + consts.IndexOne, nil
	}

	return start + lineEndOffset(content, start), nil
}

func blockScalarEnd(content []byte, key, value *yaml.Node) (int, error) {
	keyIndent := key.Column - consts.IndexOne
	line := value.Line + consts.IndexOne

	for line <= countLines(content) {
		start, err := lineStartOffset(content, line)
		if err != nil {
			return consts.IndexZero, err
		}

		end := lineEndOffset(content, start)
		lineText := content[start : start+end]

		if len(bytes.TrimSpace(lineText)) == consts.IndexZero {
			line++

			continue
		}

		indent := leadingSpaces(lineText)

		if indent <= keyIndent {
			return start, nil
		}

		line++
	}

	return len(content), nil
}

func nodeStartOffset(content []byte, node *yaml.Node) (int, error) {
	if node == nil || node.Line < consts.IndexOne || node.Column < consts.IndexOne {
		return consts.IndexZero, errors.New("invalid YAML node position")
	}

	start, err := lineStartOffset(content, node.Line)
	if err != nil {
		return consts.IndexZero, err
	}

	return start + node.Column - consts.IndexOne, nil
}

func documentBodyStart(content []byte) int {
	if bytes.HasPrefix(content, []byte("---\r\n")) {
		return len("---\r\n")
	}

	if bytes.HasPrefix(content, []byte("---\n")) {
		return len("---\n")
	}

	return consts.IndexZero
}

func mappingEntrySpan(content []byte, parent *yaml.Node, pairIndex, sectionEnd int) (int, int) {
	key := parent.Content[pairIndex]
	start, _ := lineStartOffset(content, key.Line)
	end := sectionEnd

	if pairIndex+yamlMappingPairKeyValue < len(parent.Content) {
		end, _ = lineStartOffset(content, parent.Content[pairIndex+yamlMappingPairKeyValue].Line)
	}

	return start, trimBlankLinesBackward(content, end)
}

func rootEntrySpan(content []byte, root *yaml.Node, key *yaml.Node) (int, int) {
	for idx := consts.IndexZero; idx < len(root.Content); idx += yamlMappingPairKeyValue {
		if root.Content[idx] != key {
			continue
		}

		return mappingEntrySpan(content, root, idx, len(content))
	}

	return consts.IndexZero, consts.IndexZero
}

func topLevelNodeEnd(content []byte, root, node *yaml.Node) int {
	for idx := consts.IndexZero; idx < len(root.Content); idx += yamlMappingPairKeyValue {
		if root.Content[idx+consts.IndexOne] != node {
			continue
		}

		if idx+yamlMappingPairKeyValue < len(root.Content) {
			start, _ := lineStartOffset(content, root.Content[idx+yamlMappingPairKeyValue].Line)

			return start
		}

		return len(content)
	}

	return len(content)
}

func trimBlankLinesBackward(content []byte, end int) int {
	for end > consts.IndexZero {
		lineEnd := end

		if content[lineEnd-consts.IndexOne] == '\n' {
			lineEnd--
		}

		lineStart := bytes.LastIndexByte(content[:lineEnd], '\n') + consts.IndexOne

		if len(bytes.TrimSpace(content[lineStart:lineEnd])) != consts.IndexZero {
			break
		}

		end = lineStart
	}

	return end
}

func trimOneLineEnding(content []byte) []byte {
	content = bytes.TrimSuffix(content, []byte("\n"))

	return bytes.TrimSuffix(content, []byte("\r"))
}

func isBlockMapping(content []byte, key, value *yaml.Node) bool {
	if value.Kind != yaml.MappingNode || value.Style&yaml.FlowStyle != consts.IndexZero {
		return false
	}

	lineStart, err := lineStartOffset(content, key.Line)
	if err != nil {
		return false
	}

	lineEnd := lineEndOffset(content, lineStart)
	line := content[lineStart : lineStart+lineEnd]
	colon := bytes.IndexByte(line, ':')

	return colon >= consts.IndexZero &&
		len(bytes.TrimSpace(line[colon+consts.IndexOne:])) == consts.IndexZero
}

func leadingSpaces(line []byte) int {
	count := consts.IndexZero

	for count < len(line) && line[count] == ' ' {
		count++
	}

	return count
}

func countLines(content []byte) int {
	return bytes.Count(content, []byte{'\n'}) + consts.IndexOne
}
