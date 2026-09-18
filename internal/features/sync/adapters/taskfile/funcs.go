// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile/yamlutil"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/yamlfmt"
	yaml "go.yaml.in/yaml/v3"
)

// NewOps returns an Ops wired to the package Taskfile helpers.
func NewOps() Ops {
	return Ops{fns: opsFns{
		newRoot:    NewRootTemplate,
		rewrite:    RewriteIncludes,
		updateRoot: UpdateRootTaskfile,
	}}
}

// NewRootTemplate returns the default root Taskfile template bytes.
func (ops Ops) NewRootTemplate() []byte {
	return ops.fns.newRoot()
}

// RewriteIncludes rewrites module include paths using sourceToDest and fromDest.
func (ops Ops) RewriteIncludes(
	content []byte,
	sourceToDest map[string]string,
	fromDest string,
) ([]byte, error) {
	data, err := ops.fns.rewrite(content, sourceToDest, fromDest)
	if err != nil {
		return nil, fmt.Errorf("rewrite includes: %w", err)
	}

	return data, nil
}

// UpdateRootTaskfile merges managed includes and generated tasks into the root Taskfile.
func (ops Ops) UpdateRootTaskfile(content []byte, input *ports.RootUpdateInput) ([]byte, error) {
	data, err := ops.fns.updateRoot(content, input)
	if err != nil {
		return nil, fmt.Errorf("update root taskfile: %w", err)
	}

	return data, nil
}

func rawModuleVarsByTask(input *rootUpdateInput) (rawModuleVars, error) {
	out := make(rawModuleVars, len(input.Tasks))

	for i := range input.Tasks {
		task := input.Tasks[i]

		values, found, err := rawModuleVarsForTask(input, task)
		if err != nil {
			return nil, fmt.Errorf("raw module vars for %q: %w", task, err)
		}

		if found {
			out[task] = values
		}
	}

	return out, nil
}

func rawModuleVarsForTask(
	input *rootUpdateInput,
	task string,
) (values map[string][]byte, found bool, err error) {
	content := input.ModuleTaskfiles[task]
	root, err := parseModuleTaskfileNode(content)

	if errors.Is(err, errNoModuleVars) {
		return map[string][]byte{}, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("parse module Taskfile for %q: %w", task, err)
	}

	values, found, err = rawVarsForParsedModule(content, root, task)
	if err != nil {
		return nil, false, fmt.Errorf("raw vars for %q: %w", task, err)
	}

	return values, found, nil
}

func rawVarsForParsedModule(
	content []byte,
	root *yaml.Node,
	task string,
) (map[string][]byte, bool, error) {
	varsNode := findMappingValue(root, keyVars)

	if !hasModuleVars(varsNode) {
		return map[string][]byte{}, false, nil
	}

	values, err := rawVarsFromNode(content, root, varsNode)
	if err != nil {
		return nil, false, fmt.Errorf("raw vars for %q: %w", task, err)
	}

	return values, true, nil
}

func rawVarsFromNode(content []byte, root, varsNode *yaml.Node) (map[string][]byte, error) {
	rootEnd := topLevelNodeEnd(content, root, varsNode)
	out := make(map[string][]byte, len(varsNode.Content)/yamlMappingPairKeyValue)

	for idx := consts.IndexZero; idx < len(varsNode.Content); idx += yamlMappingPairKeyValue {
		key := varsNode.Content[idx]

		start, end, err := sourceNodeSpan(&sourceNodeSpanParams{
			content: content, parent: varsNode, pairIndex: idx, parentEnd: rootEnd,
		})
		if err != nil {
			return nil, fmt.Errorf("source span for %q: %w", key.Value, err)
		}

		out[key.Value] = bytes.Clone(content[start:end])
	}

	return out, nil
}

func patchRootTaskfile(params *rootPatchParams) ([]byte, error) {
	edits, err := rootTextEdits(params)
	if err != nil {
		return nil, fmt.Errorf("build root text edits: %w", err)
	}

	normalizeRootEditLineEndings(edits, rootLineEnding(params.content))

	out := applyRootTextEdits(params.content, edits)

	if err := validatePatchedRoot(out); err != nil {
		return nil, fmt.Errorf("validate patched root Taskfile: %w", err)
	}

	return out, nil
}

func validatePatchedRoot(content []byte) error {
	parsedRoot, parsedContent, err := parseTaskfileRoot(
		content,
		"validate patched root Taskfile YAML: %v",
		"empty patched root Taskfile YAML",
	)
	iox.Discard2(parsedRoot, parsedContent)

	if err != nil {
		return fmt.Errorf("parse patched root Taskfile: %w", err)
	}

	return nil
}

func normalizeRootEditLineEndings(edits []rootTextEdit, ending []byte) {
	if bytes.Equal(ending, []byte(lineFeed)) {
		return
	}

	for i := range edits {
		replacement := bytes.ReplaceAll(
			edits[i].replacement,
			[]byte(carriageReturnLineFeed),
			[]byte(lineFeed),
		)

		edits[i].replacement = bytes.ReplaceAll(replacement, []byte(lineFeed), ending)
	}
}

func rootLineEnding(content []byte) []byte {
	if bytes.Contains(content, []byte(carriageReturnLineFeed)) {
		return []byte(carriageReturnLineFeed)
	}

	return []byte(lineFeed)
}

func rootTextEdits(params *rootPatchParams) ([]rootTextEdit, error) {
	edits, err := appendVersionEdit(nil, params)
	if err != nil {
		return nil, fmt.Errorf("version edit: %w", err)
	}

	for _, section := range []string{keyVars, keyIncludes, keyTasks} {
		edits, err = appendSectionEdits(edits, params, section)
		if err != nil {
			return nil, fmt.Errorf("section %s: %w", section, err)
		}
	}

	return orderRootEdits(edits), nil
}

func appendSectionEdits(
	edits []rootTextEdit,
	params *rootPatchParams,
	section string,
) ([]rootTextEdit, error) {
	sectionEdits, err := editsForRootSection(params, section)
	if err != nil {
		return nil, fmt.Errorf("%s edits: %w", section, err)
	}

	return append(edits, sectionEdits...), nil
}

func orderRootEdits(edits []rootTextEdit) []rootTextEdit {
	for index := range edits {
		edits[index].order = index
	}

	return edits
}

func appendVersionEdit(edits []rootTextEdit, params *rootPatchParams) ([]rootTextEdit, error) {
	desired := findMappingPair(params.desiredRoot, versionKey)

	if desired.value == nil {
		return edits, nil
	}

	result, err := appendExistingVersionEdit(edits, params)
	if err != nil {
		return nil, fmt.Errorf("existing version edit: %w", err)
	}

	return result, nil
}

func appendExistingVersionEdit(
	edits []rootTextEdit,
	params *rootPatchParams,
) ([]rootTextEdit, error) {
	original := findMappingPair(params.originalRoot, versionKey)
	desired := findMappingPair(params.desiredRoot, versionKey)

	if original.value == nil {
		return append(edits, missingVersionEdit(params, desired.value)), nil
	}

	if original.value.Value == desired.value.Value {
		return edits, nil
	}

	edit, err := changedVersionEdit(params, original, desired)
	if err != nil {
		return nil, fmt.Errorf("changed version edit: %w", err)
	}

	return append(edits, edit), nil
}

func missingVersionEdit(params *rootPatchParams, value *yaml.Node) rootTextEdit {
	start := documentBodyStart(params.content)

	return rootTextEdit{
		start: start, end: start,
		replacement: []byte(versionKey + ": \"" + value.Value + "\"" + lineFeed),
	}
}

func changedVersionEdit(
	params *rootPatchParams,
	original, desired mappingPair,
) (rootTextEdit, error) {
	start, end, err := scalarSourceSpan(params.content, original.key, original.value)
	if err != nil {
		return rootTextEdit{}, fmt.Errorf("locate version span: %w", err)
	}

	return rootTextEdit{
		start: start, end: end,
		replacement: []byte(`"` + desired.value.Value + `"`),
	}, nil
}

func editsForRootSection(params *rootPatchParams, section string) ([]rootTextEdit, error) {
	original := findMappingPair(params.originalRoot, section)
	desired := findMappingPair(params.desiredRoot, section)

	if desired.value == nil {
		return nil, nil
	}

	if original.value == nil {
		result, err := appendRootSection(params, section, desired.value)
		if err != nil {
			return nil, fmt.Errorf("append %s section: %w", section, err)
		}

		return result, nil
	}

	if !isBlockMapping(params.content, original.key, original.value) {
		result, err := replaceRootSection(&rootSectionReplacementParams{
			params: params, section: section, original: original, desired: desired,
		})
		if err != nil {
			return nil, fmt.Errorf("replace %s section: %w", section, err)
		}

		return result, nil
	}

	result, err := editsForExistingRootSection(&rootSectionEditParams{
		params: params, section: section, original: original.value, desired: desired.value,
	})
	if err != nil {
		return nil, fmt.Errorf("edit %s section: %w", section, err)
	}

	return result, nil
}

func appendRootSection(
	params *rootPatchParams,
	section string,
	node *yaml.Node,
) ([]rootTextEdit, error) {
	replacement, err := renderRootSection(&renderSectionParams{
		section: section, node: node, rawVars: params.rawVars, tasks: params.input.Tasks,
	})
	if err != nil {
		return nil, fmt.Errorf(errRenderSection, section, err)
	}

	start := len(params.content)

	if start != consts.IndexZero && params.content[start-consts.IndexOne] != '\n' {
		replacement = append([]byte{'\n'}, replacement...)
	}

	return []rootTextEdit{{start: start, end: start, replacement: replacement}}, nil
}

func replaceRootSection(
	input *rootSectionReplacementParams,
) ([]rootTextEdit, error) {
	start, end := rootEntrySpan(input.params.content, input.params.originalRoot, input.original.key)

	replacement, err := renderRootSection(&renderSectionParams{
		section: input.section, node: input.desired.value,
		rawVars: input.params.rawVars, tasks: input.params.input.Tasks,
	})
	if err != nil {
		return nil, fmt.Errorf("render %s section: %w", input.section, err)
	}

	return []rootTextEdit{{start: start, end: end, replacement: replacement}}, nil
}

func editsForExistingRootSection(
	input *rootSectionEditParams,
) ([]rootTextEdit, error) {
	if input.section == keyVars {
		result, err := editsForRootVars(input.params, input.original, input.desired)
		if err != nil {
			return nil, fmt.Errorf("edit root vars: %w", err)
		}

		return result, nil
	}

	managed := managedNamesForSection(input.params.input, input.section)

	if managed == nil {
		return nil, nil
	}

	edits, err := editsForManagedMapping(&managedMappingParams{
		params: input.params, original: input.original, desired: input.desired, managed: managed,
	})
	if err != nil {
		return nil, fmt.Errorf("edit managed %s: %w", input.section, err)
	}

	return edits, nil
}

func managedNamesForSection(input *rootUpdateInput, section string) map[string]struct{} {
	if section == keyIncludes {
		return managedIncludeNames(input)
	}

	if section == keyTasks {
		return managedRootTaskNames(input)
	}

	return nil
}

func editsForRootVars(
	params *rootPatchParams,
	original, desired *yaml.Node,
) ([]rootTextEdit, error) {
	newPairs := newRootVarPairs(original, desired)

	if len(newPairs) == consts.IndexZero {
		return nil, nil
	}

	result, err := insertRootVarPairs(params, original, newPairs)
	if err != nil {
		return nil, fmt.Errorf("insert root vars: %w", err)
	}

	return result, nil
}

func newRootVarPairs(original, desired *yaml.Node) []mappingPair {
	originalKeys := mappingKeys(original)

	var newPairs []mappingPair

	for idx := consts.IndexZero; idx < len(desired.Content); idx += yamlMappingPairKeyValue {
		key := desired.Content[idx].Value

		if _, exists := originalKeys[key]; exists {
			continue
		}

		newPairs = append(newPairs, mappingPair{
			key: desired.Content[idx], value: desired.Content[idx+consts.IndexOne],
		})
	}

	return newPairs
}

func insertRootVarPairs(
	params *rootPatchParams,
	original *yaml.Node,
	pairs []mappingPair,
) ([]rootTextEdit, error) {
	sectionEnd := topLevelNodeEnd(params.content, params.originalRoot, original)
	insertAt := trimBlankLinesBackward(params.content, sectionEnd)

	text, err := renderRootVarPairs(pairs, params.rawVars, params.input.Tasks)
	if err != nil {
		return nil, fmt.Errorf("render root vars: %w", err)
	}

	return []rootTextEdit{{start: insertAt, end: insertAt, replacement: text}}, nil
}

func editsForManagedMapping(input *managedMappingParams) ([]rootTextEdit, error) {
	params := input.params
	sectionEnd := topLevelNodeEnd(params.content, params.originalRoot, input.original)

	removeOrUpdate, err := replaceManagedEntries(input, sectionEnd)
	if err != nil {
		return nil, fmt.Errorf("replace managed entries: %w", err)
	}

	result, err := appendManagedEntries(removeOrUpdate, input, sectionEnd)
	if err != nil {
		return nil, fmt.Errorf("append managed entries: %w", err)
	}

	return result, nil
}

func replaceManagedEntries(input *managedMappingParams, sectionEnd int) ([]rootTextEdit, error) {
	var edits []rootTextEdit

	for idx := consts.IndexZero; idx < len(input.original.Content); idx += yamlMappingPairKeyValue {
		key := input.original.Content[idx].Value

		edit, changed, err := managedEntryEdit(&managedEntryParams{
			input: input, key: key, pairIndex: idx, sectionEnd: sectionEnd,
		})
		if err != nil {
			return nil, fmt.Errorf("managed entry %q: %w", key, err)
		}

		if changed {
			edits = append(edits, edit)
		}
	}

	return edits, nil
}

func managedEntryEdit(params *managedEntryParams) (rootTextEdit, bool, error) {
	if _, isManaged := params.input.managed[params.key]; !isManaged {
		return rootTextEdit{}, false, nil
	}

	start, end := mappingEntrySpan(&mappingEntrySpanParams{
		content: params.input.params.content, parent: params.input.original,
		pairIndex: params.pairIndex, sectionEnd: params.sectionEnd,
	})
	newPair := desiredPair(params.input.desired, params.key)

	if newPair.value == nil {
		return rootTextEdit{start: start, end: end}, true, nil
	}

	edit, changed, err := renderManagedEntry(params, newPair, start, end)
	if err != nil {
		return rootTextEdit{}, false, fmt.Errorf("render managed entry: %w", err)
	}

	return edit, changed, nil
}

func renderManagedEntry(
	params *managedEntryParams,
	newPair mappingPair,
	start, end int,
) (rootTextEdit, bool, error) {
	replacement, err := renderMappingPair(newPair)
	if err != nil {
		return rootTextEdit{}, false, fmt.Errorf("render managed entry %q: %w", params.key, err)
	}

	if bytes.Equal(params.input.params.content[start:end], replacement) {
		return rootTextEdit{}, false, nil
	}

	return rootTextEdit{start: start, end: end, replacement: replacement}, true, nil
}

func appendManagedEntries(
	edits []rootTextEdit,
	input *managedMappingParams,
	sectionEnd int,
) ([]rootTextEdit, error) {
	entries := newManagedEntries(input)

	if len(entries) == consts.IndexZero {
		return edits, nil
	}

	text, err := renderManagedEntries(entries)
	if err != nil {
		return nil, fmt.Errorf("render managed entries: %w", err)
	}

	insertAt := trimBlankLinesBackward(input.params.content, sectionEnd)

	return append(edits, rootTextEdit{start: insertAt, end: insertAt, replacement: text}), nil
}

func newManagedEntries(input *managedMappingParams) []mappingPair {
	originalKeys := mappingKeys(input.original)

	var entries []mappingPair

	for idx := consts.IndexZero; idx < len(input.desired.Content); idx += yamlMappingPairKeyValue {
		key := input.desired.Content[idx].Value

		if _, already := originalKeys[key]; already {
			continue
		}

		if _, isManaged := input.managed[key]; isManaged {
			entries = append(entries, mappingPair{
				key: input.desired.Content[idx], value: input.desired.Content[idx+consts.IndexOne],
			})
		}
	}

	return entries
}

func renderManagedEntries(entries []mappingPair) ([]byte, error) {
	var text []byte

	for i := range entries {
		entry, err := renderMappingPair(entries[i])
		if err != nil {
			return nil, fmt.Errorf("render new managed entry %q: %w", entries[i].key.Value, err)
		}

		text = append(text, entry...)
	}

	return text, nil
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

	for i := range first {
		out[first[i]] = struct{}{}
	}

	for i := range second {
		out[second[i]] = struct{}{}
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

func renderRootSection(params *renderSectionParams) ([]byte, error) {
	out := []byte(params.section + ":\n")
	pairs := mappingPairs(params.node)

	text, err := renderRootSectionBody(params, pairs)
	if err != nil {
		return nil, fmt.Errorf("render root section body: %w", err)
	}

	return append(out, text...), nil
}

func renderRootSectionBody(params *renderSectionParams, pairs []mappingPair) ([]byte, error) {
	if params.section == keyVars {
		result, err := renderRootVarPairs(pairs, params.rawVars, params.tasks)
		if err != nil {
			return nil, fmt.Errorf("render root vars: %w", err)
		}

		return result, nil
	}

	result, err := renderMappingPairs(pairs)
	if err != nil {
		return nil, fmt.Errorf("render mapping pairs: %w", err)
	}

	return result, nil
}

func mappingPairs(node *yaml.Node) []mappingPair {
	pairs := make([]mappingPair, consts.IndexZero, len(node.Content)/yamlMappingPairKeyValue)

	for idx := consts.IndexZero; idx < len(node.Content); idx += yamlMappingPairKeyValue {
		pairs = append(
			pairs,
			mappingPair{key: node.Content[idx], value: node.Content[idx+consts.IndexOne]},
		)
	}

	return pairs
}

func renderMappingPairs(pairs []mappingPair) ([]byte, error) {
	var out []byte

	for i := range pairs {
		text, err := renderMappingPair(pairs[i])
		if err != nil {
			return nil, fmt.Errorf(errRenderMappingPair, err)
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
	var out []byte

	for i := range pairs {
		text, err := renderRootVarPair(pairs[i], rawVars, tasks)
		if err != nil {
			return nil, fmt.Errorf("render root var pair: %w", err)
		}

		out = append(out, text...)
	}

	return out, nil
}

func renderRootVarPair(pair mappingPair, rawVars rawModuleVars, tasks []string) ([]byte, error) {
	raw := firstRawVar(tasks, rawVars, pair.key.Value)

	if len(raw) != consts.IndexZero &&
		(pair.value.Kind != yaml.ScalarNode || strings.Contains(string(raw), defaultMarker)) {

		return renderRawRootVarPair(pair, raw), nil
	}

	text, err := renderMappingPair(pair)
	if err != nil {
		return nil, fmt.Errorf("render mapping pair: %w", err)
	}

	return text, nil
}

func renderRawRootVarPair(pair mappingPair, raw []byte) []byte {
	raw = trimOneLineEnding(raw)

	if pair.value.Kind != yaml.ScalarNode && pair.value.Style&yaml.FlowStyle == consts.IndexZero {
		return renderBlockRawVarPair(pair, raw)
	}

	out := []byte("  " + pair.key.Value + ": ")

	out = append(out, raw...)
	out = append(out, '\n')

	return out
}

func renderBlockRawVarPair(pair mappingPair, raw []byte) []byte {
	out := []byte("  " + pair.key.Value + ":\n")

	out = append(out, bytes.Repeat([]byte{' '}, rootVarIndent)...)
	out = append(out, raw...)

	return append(out, '\n')
}

func firstRawVar(tasks []string, rawVars rawModuleVars, key string) []byte {
	for i := range tasks {
		if value, found := rawVarForTask(rawVars, tasks[i], key); found {
			return value
		}
	}

	return nil
}

func rawVarForTask(rawVars rawModuleVars, task, key string) ([]byte, bool) {
	values, ok := rawVars[task]

	if !ok {
		return nil, false
	}

	value, found := values[key]

	return value, found
}

func renderMappingPair(pair mappingPair) ([]byte, error) {
	mapNode := newYAMLMappingNode()
	appendMappingPair(mapNode, cloneYAMLNode(pair.key), cloneYAMLNode(pair.value))

	encoded, err := marshalNode(mapNode, "marshal mapping pair: %v")
	if err != nil {
		return nil, fmt.Errorf("marshal mapping pair: %w", err)
	}

	body := bytes.TrimPrefix(encoded, []byte(documentStartLineFeed))

	return indentLines(body, mappingPairIndent), nil
}

func indentLines(content []byte, spaces int) []byte {
	prefix := bytes.Repeat([]byte{' '}, spaces)
	out := make([]byte, consts.IndexZero, len(content)+len(prefix))

	for len(content) != consts.IndexZero {
		out = append(out, prefix...)

		line, rest, done := nextIndentedLine(content)

		out = append(out, line...)

		if done {
			break
		}

		content = rest
	}

	return out
}

func nextIndentedLine(content []byte) (line, rest []byte, done bool) {
	idx := bytes.IndexByte(content, '\n')

	if idx < consts.IndexZero {
		return content, nil, true
	}

	return content[:idx+consts.IndexOne], content[idx+consts.IndexOne:], false
}

func applyRootTextEdits(content []byte, edits []rootTextEdit) []byte {
	slices.SortFunc(edits, func(left, right rootTextEdit) int {
		if left.start != right.start {
			return right.start - left.start
		}

		return right.order - left.order
	})

	out := bytes.Clone(content)

	for i := range edits {
		out = replaceByteSpan(&replaceSpanParams{
			content: out,
			start:   edits[i].start,
			end:     edits[i].end,
			value:   string(edits[i].replacement),
		})
	}

	return out
}

func sourceNodeSpan(params *sourceNodeSpanParams) (startOffset, endOffset int, err error) {
	key := params.parent.Content[params.pairIndex]
	value := params.parent.Content[params.pairIndex+consts.IndexOne]

	start, err := nodeStartOffset(params.content, value)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf(nodeStartOffsetError, err)
	}

	return sourceNodeEnd(params, key, value, start)
}

func sourceNodeEnd(
	params *sourceNodeSpanParams,
	key, value *yaml.Node,
	start int,
) (startOffset, endOffset int, err error) {
	if value.Kind == yaml.ScalarNode {
		startOffset, endOffset, err = scalarNodeSpan(&scalarNodeSpanParams{
			content: params.content, key: key, value: value, start: start,
		})
		if err != nil {
			return 0, 0, fmt.Errorf("scalar node span: %w", err)
		}

		return startOffset, endOffset, nil
	}

	end, err := collectionNodeEnd(params)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf("collection node end: %w", err)
	}

	return start, trimBlankLinesBackward(params.content, end), nil
}

func scalarNodeSpan(params *scalarNodeSpanParams) (startOffset, endOffset int, err error) {
	end, err := scalarSourceEnd(params.content, params.key, params.value)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf(errScalarSourceEnd, err)
	}

	return params.start, trimBlankLinesBackward(params.content, end), nil
}

func collectionNodeEnd(params *sourceNodeSpanParams) (int, error) {
	if params.pairIndex+yamlMappingPairKeyValue >= len(params.parent.Content) {
		return params.parentEnd, nil
	}

	next := params.parent.Content[params.pairIndex+yamlMappingPairKeyValue]

	end, err := lineStartOffset(params.content, next.Line)
	if err != nil {
		return consts.IndexZero, fmt.Errorf(lineStartOffsetError, err)
	}

	return end, nil
}

func scalarSourceSpan(
	content []byte,
	key, value *yaml.Node,
) (startOffset, endOffset int, err error) {
	start, err := nodeStartOffset(content, value)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf(nodeStartOffsetError, err)
	}

	end, err := scalarSourceEnd(content, key, value)
	if err != nil {
		return consts.IndexZero, consts.IndexZero, fmt.Errorf("scalar source end: %w", err)
	}

	return start, end, nil
}

func scalarSourceEnd(content []byte, key, value *yaml.Node) (int, error) {
	start, err := nodeStartOffset(content, value)
	if err != nil {
		return consts.IndexZero, fmt.Errorf(nodeStartOffsetError, err)
	}

	if value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != consts.IndexZero {
		end, err := blockScalarSourceEnd(content, key, value)
		if err != nil {
			return 0, fmt.Errorf("block scalar source end: %w", err)
		}

		return end, nil
	}

	if quote, quoted := quoteForYAMLStyle(value.Style); quoted {
		end, err := quotedScalarSourceEnd(content, start, quote)
		if err != nil {
			return 0, fmt.Errorf("quoted scalar source end: %w", err)
		}

		return end, nil
	}

	return start + lineEndOffset(content, start), nil
}

func blockScalarSourceEnd(content []byte, key, value *yaml.Node) (int, error) {
	end, err := blockScalarEnd(content, key, value)
	if err != nil {
		return consts.IndexZero, fmt.Errorf("block scalar end: %w", err)
	}

	return end, nil
}

func quotedScalarSourceEnd(content []byte, start int, quote byte) (int, error) {
	closeIndex, err := findClosingQuote(content, start, quote)
	if err != nil {
		return consts.IndexZero, fmt.Errorf(errFindClosingQuote, err)
	}

	return closeIndex + consts.IndexOne, nil
}

func blockScalarEnd(content []byte, key, value *yaml.Node) (int, error) {
	keyIndent := key.Column - consts.IndexOne

	for line := value.Line + consts.IndexOne; line <= countLines(content); line++ {
		start, inside, err := blockScalarLine(content, keyIndent, line)
		if err != nil {
			return consts.IndexZero, fmt.Errorf("block scalar line: %w", err)
		}

		if !inside {
			return start, nil
		}
	}

	return len(content), nil
}

func blockScalarLine(content []byte, keyIndent, line int) (int, bool, error) {
	start, err := lineStartOffset(content, line)
	if err != nil {
		return consts.IndexZero, false, fmt.Errorf(lineStartOffsetError, err)
	}

	end := lineEndOffset(content, start)
	lineText := content[start : start+end]

	if len(bytes.TrimSpace(lineText)) == consts.IndexZero {
		return start, true, nil
	}

	return start, leadingSpaces(lineText) > keyIndent, nil
}

func nodeStartOffset(content []byte, node *yaml.Node) (int, error) {
	if node == nil || node.Line < consts.IndexOne || node.Column < consts.IndexOne {
		return consts.IndexZero, errInvalidYAMLNodePosition
	}

	start, err := lineStartOffset(content, node.Line)
	if err != nil {
		return consts.IndexZero, fmt.Errorf(lineStartOffsetError, err)
	}

	return start + node.Column - consts.IndexOne, nil
}

func documentBodyStart(content []byte) int {
	if bytes.HasPrefix(content, []byte(documentStartCRLF)) {
		return len(documentStartCRLF)
	}

	if bytes.HasPrefix(content, []byte(documentStartLineFeed)) {
		return len(documentStartLineFeed)
	}

	return consts.IndexZero
}

func mappingEntrySpan(params *mappingEntrySpanParams) (startOffset, endOffset int) {
	key := params.parent.Content[params.pairIndex]

	start, err := lineStartOffset(params.content, key.Line)
	if err != nil {
		return consts.IndexZero, consts.IndexZero
	}

	end := params.sectionEnd

	if params.pairIndex+yamlMappingPairKeyValue < len(params.parent.Content) {
		end, err = lineStartOffset(
			params.content,
			params.parent.Content[params.pairIndex+yamlMappingPairKeyValue].Line,
		)
		if err != nil {
			return consts.IndexZero, consts.IndexZero
		}
	}

	return start, trimBlankLinesBackward(params.content, end)
}

func rootEntrySpan(content []byte, root *yaml.Node, key *yaml.Node) (startOffset, endOffset int) {
	for idx := consts.IndexZero; idx < len(root.Content); idx += yamlMappingPairKeyValue {
		if root.Content[idx] != key {
			continue
		}

		return mappingEntrySpan(&mappingEntrySpanParams{
			content: content, parent: root, pairIndex: idx, sectionEnd: len(content),
		})
	}

	return consts.IndexZero, consts.IndexZero
}

func topLevelNodeEnd(content []byte, root, node *yaml.Node) int {
	for idx := consts.IndexZero; idx < len(root.Content); idx += yamlMappingPairKeyValue {
		if root.Content[idx+consts.IndexOne] != node {
			continue
		}

		return nextTopLevelNodeStart(content, root, idx)
	}

	return len(content)
}

func nextTopLevelNodeStart(content []byte, root *yaml.Node, idx int) int {
	if idx+yamlMappingPairKeyValue >= len(root.Content) {
		return len(content)
	}

	start, err := lineStartOffset(content, root.Content[idx+yamlMappingPairKeyValue].Line)
	if err != nil {
		return consts.IndexZero
	}

	return start
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
	content = bytes.TrimSuffix(content, []byte(lineFeed))

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
// (for example "eslint" or "eslint/node"), used to recompute relative
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
		return consts.IndexZero, fmt.Errorf(lineStartOffsetError, err)
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
func quoteForYAMLStyle(style yaml.Style) (quote byte, quoted bool) {
	if style == yaml.DoubleQuotedStyle {
		return '"', true
	}

	if style == yaml.SingleQuotedStyle {
		return '\'', true
	}

	return byte(consts.IndexZero), false
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

// marshalNode serializes node to YAML. marshalErrMsg formats the failure. The
// encoder only emits well-formed YAML, so the result needs no re-parse.
func marshalNode(node *yaml.Node, marshalErrMsg string) ([]byte, error) {
	out, err := yamlfmt.Marshal(node)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf(marshalErrMsg, err)}
	}

	return out, nil
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
// directory. Slashed variant modules such as eslint/node keep their slash in
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

	writeErr := iox.WriteStringFull(&prefixSb, prefix)
	iox.Discard(writeErr)

	for {
		rest, ok := strings.CutPrefix(dir, dotDotSlash)

		if !ok {
			return finalizeRelativePrefix(prefixSb.String(), dir)
		}

		writeErr = iox.WriteStringFull(&prefixSb, dotDotSlash)
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

	result, err := updateParsedRootTaskfile(&parsedRootTaskfileParams{
		content: content, input: input, node: node, root: root,
	})
	if err != nil {
		return nil, fmt.Errorf("update parsed root taskfile: %w", err)
	}

	return result, nil
}

func updateParsedRootTaskfile(params *parsedRootTaskfileParams) ([]byte, error) {
	originalRoot := cloneYAMLNode(params.root)

	rawModuleVars, err := rawModuleVarsByTask(params.input)
	if err != nil {
		return nil, fmt.Errorf("read raw module vars: %w", err)
	}

	result, err := applyParsedRootUpdates(params, originalRoot, rawModuleVars)
	if err != nil {
		return nil, fmt.Errorf("apply parsed root updates: %w", err)
	}

	return result, nil
}

func applyParsedRootUpdates(
	params *parsedRootTaskfileParams, originalRoot *yaml.Node, rawModuleVars rawModuleVars,
) ([]byte, error) {
	setRootTaskfileVersion(params.root)

	err := applyRootUpdates(params.root, params.input)
	if err != nil {
		return nil, fmt.Errorf("apply root updates: %w", err)
	}

	out, err := patchRootTaskfile(&rootPatchParams{
		content: params.content, originalRoot: originalRoot, desiredRoot: params.root,
		input: params.input, rawVars: rawModuleVars,
	})
	if err != nil {
		return nil, fmt.Errorf("patch root taskfile: %w", err)
	}

	iox.Discard(params.node)

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

	appendMappingPair(
		params.rootVars,
		yamlScalar(params.key),
		overridableRootVar(params.key, value),
	)
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

// overridableRootVar wraps a newly promoted scalar as Task's overridable default form
// ('{{.KEY | default "..."}}'). Non-scalars and values that already use | default are
// returned unchanged. Defaults containing " use Go-template raw backticks.
func overridableRootVar(key string, value *yaml.Node) *yaml.Node {
	if value == nil || value.Kind != yaml.ScalarNode {
		return value
	}

	if strings.Contains(value.Value, "| default") {
		return value
	}

	node := yamlScalar("{{." + key + " | default " + overridableDefaultArg(value.Value) + "}}")

	node.Style = yaml.SingleQuotedStyle

	return node
}

func overridableDefaultArg(value string) string {
	if strings.Contains(value, doubleQuote) {
		return "`" + value + "`"
	}

	return doubleQuote + value + doubleQuote
}

// cloneYAMLNode returns a deep copy of node so promoted values can be reused
// without aliasing the parsed module document.
func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	return yamlutil.Clone(node)
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

func newYAMLMappingNode() *yaml.Node {
	return yamlutil.Mapping()
}

func newYAMLSequenceNode() *yaml.Node {
	return yamlutil.Sequence()
}

func yamlScalar(value string) *yaml.Node {
	return yamlutil.Scalar(value)
}

func appendMappingPair(mapNode, key, value *yaml.Node) {
	yamlutil.AppendMappingPair(mapNode, key, value)
}

func mappingKeys(mapNode *yaml.Node) map[string]struct{} {
	return yamlutil.Keys(mapNode)
}

func setMappingValue(mapNode *yaml.Node, key string, value *yaml.Node) {
	yamlutil.SetValue(mapNode, key, value)
}

func setRootTaskfileVersion(root *yaml.Node) {
	setMappingValue(root, versionKey, taskfileVersionScalar(rootTaskfileVersion))
}

func taskfileVersionScalar(value string) *yaml.Node {
	node := yamlScalar(value)

	node.Style = yaml.DoubleQuotedStyle

	return node
}

func sortedKeys(m map[string]struct{}) []string {
	return yamlutil.SortedKeys(m)
}

func deleteMappingKey(mapNode *yaml.Node, key string) {
	yamlutil.DeleteKey(mapNode, key)
}

func containsString(list []string, target string) bool {
	return slices.Contains(list, target)
}
