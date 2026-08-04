// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package taskfile reads and rewrites Taskfile YAML includes and shared vars.
package taskfile

import (
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
		sharedVars   map[string]struct{}
		input        *rootUpdateInput
	}

	// includeUpsertParams carries state for upserting one managed include entry.
	includeUpsertParams struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		moduleVars   map[string]*yaml.Node
		sharedVars   map[string]struct{}
		input        *rootUpdateInput
		task         string
	}

	// existingIncludeParams carries state for updating an existing include entry.
	existingIncludeParams struct {
		entry        *yaml.Node
		moduleVars   *yaml.Node
		sharedVars   map[string]struct{}
		path         string
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

	// sharedVarParams carries state for promoting shared module vars to the root.
	sharedVarParams struct {
		root       *yaml.Node
		moduleVars map[string]*yaml.Node
		sharedVars map[string]struct{}
		tasks      []string
	}

	// addSharedVarParams carries state for adding one shared var to the root.
	addSharedVarParams struct {
		rootVars   *yaml.Node
		moduleVars map[string]*yaml.Node
		existing   map[string]struct{}
		key        string
		tasks      []string
	}

	// mergeModuleVarParams carries state for merging one module var into an include.
	mergeModuleVarParams struct {
		existingVars *yaml.Node
		keyNode      *yaml.Node
		valueNode    *yaml.Node
		sharedVars   map[string]struct{}
		existingKeys map[string]struct{}
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
)

const (
	rootTaskfileVersion     = "3.5"
	rootTemplate            = "---\nversion: \"" + rootTaskfileVersion + "\"\n"
	yamlMappingPairKeyValue = consts.IndexTwo

	dotSlash       = "./"
	dotDotSlash    = "../"
	keyIncludes    = "includes"
	keyTaskfile    = "taskfile"
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
func RewriteIncludes(
	content []byte,
	sourceToDest map[string]string,
	fromDest string,
) ([]byte, error) {
	node, root, err := parseTaskfileRoot(content, "parse Taskfile YAML: %v", "empty Taskfile YAML")
	if err != nil {
		return nil, fmt.Errorf(errParseTaskfileRoot, err)
	}

	rewriteIncludesInRoot(root, sourceToDest, fromDest)

	out, err := marshalRewrittenTaskfile(node)
	if err != nil {
		return nil, fmt.Errorf("marshal rewritten taskfile: %w", err)
	}

	return out, nil
}

func rewriteIncludesInRoot(root *yaml.Node, sourceToDest map[string]string, fromDest string) {
	includesNode := findMappingValue(root, keyIncludes)

	if includesNode != nil {
		rewriteIncludesNode(includesNode, sourceToDest, fromDest)
	}
}

func marshalRewrittenTaskfile(node *yaml.Node) ([]byte, error) {
	out, err := marshalAndValidate(
		node,
		"marshal Taskfile YAML: %v",
		"validate rewritten Taskfile YAML: %v",
	)
	if err != nil {
		return nil, fmt.Errorf(errMarshalAndValidate, err)
	}

	return out, nil
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

func rewriteIncludesNode(includes *yaml.Node, sourceToDest map[string]string, fromDest string) {
	if includes.Kind != yaml.MappingNode {
		return
	}

	for idx := consts.IndexZero; idx < len(includes.Content); idx += yamlMappingPairKeyValue {
		rewriteOneIncludeEntry(includes.Content[idx+consts.IndexOne], sourceToDest, fromDest)
	}
}

func rewriteOneIncludeEntry(entry *yaml.Node, sourceToDest map[string]string, fromDest string) {
	if entry.Kind != yaml.MappingNode {
		return
	}

	taskfileNode := findMappingValue(entry, keyTaskfile)

	if taskfileNode == nil || taskfileNode.Kind != yaml.ScalarNode {
		return
	}

	taskfileNode.Value = rewriteIncludePath(taskfileNode.Value, sourceToDest, fromDest)
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

	if rootDir == consts.Empty || rootDir == "." {
		return target
	}

	rel, err := filepath.Rel(filepath.FromSlash(rootDir), filepath.FromSlash(target))
	if err != nil {
		return target
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
	moduleVars, sharedVars, err := applyRootVars(root, input)
	if err != nil {
		return fmt.Errorf("apply root vars: %w", err)
	}

	err = applyRootIncludesAndTasks(root, &includesUpdateParams{
		includesNode: nil,
		existing:     nil,
		input:        input,
		moduleVars:   moduleVars,
		sharedVars:   sharedVars,
	})
	if err != nil {
		return fmt.Errorf("apply root includes and tasks: %w", err)
	}

	return nil
}

//nolint:gocritic // single-line sig for whitespace
func applyRootVars(root *yaml.Node, input *rootUpdIn) (yamlNodeMap, strSet, error) {
	moduleVars, err := moduleVarsByTask(input)
	if err != nil {
		return nil, nil, fmt.Errorf("module vars by task: %w", err)
	}

	sharedVars := sharedModuleVarNames(input.Tasks, moduleVars)

	err = upsertRootSharedVars(&sharedVarParams{
		root: root, tasks: input.Tasks, moduleVars: moduleVars, sharedVars: sharedVars,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("upsert root shared vars: %w", err)
	}

	return moduleVars, sharedVars, nil
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
			sharedVars:   params.sharedVars,
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
		moduleVars:   params.moduleVars[params.task],
		sharedVars:   params.sharedVars,
		managedTasks: params.input.ManagedTasks,
		task:         params.task,
	})
	if err != nil {
		return fmt.Errorf("update existing include for task %q: %w", params.task, err)
	}

	return nil
}

func appendNewInclude(params *includeUpsertParams, path string, moduleVars *yaml.Node) {
	entry := newIncludeEntry(path, moduleVars, params.sharedVars)
	appendMappingPair(params.includesNode, yamlScalar(params.task), entry)
}

func updateExistingInclude(params *existingIncludeParams) error {
	err := ensureManagedInclude(params)
	if err != nil {
		return fmt.Errorf("ensure managed include for task %q: %w", params.task, err)
	}

	setIncludePath(params.entry, params.path)
	mergeIncludeVars(params.entry, params.moduleVars, params.sharedVars)

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

func setIncludePath(entry *yaml.Node, path string) {
	taskfileNode := findMappingValue(entry, keyTaskfile)

	if taskfileNode == nil {
		appendMappingPair(entry, yamlScalar(keyTaskfile), yamlScalar(path))

		return
	}

	taskfileNode.Value = path
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

func sharedModuleVarNames(tasks []string, modVars yamlNodeMap) strSet {
	counts := countVarKeysPerTask(tasks, modVars)
	shared := make(map[string]struct{})

	for key := range counts {
		if counts[key] >= yamlMappingPairKeyValue {
			shared[key] = struct{}{}
		}
	}

	return shared
}

func countVarKeysPerTask(tasks []string, moduleVarsByTask map[string]*yaml.Node) map[string]int {
	counts := make(map[string]int)

	for i := range tasks {
		for key := range varKeySet(moduleVarsByTask[tasks[i]]) {
			counts[key]++
		}
	}

	return counts
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

func upsertRootSharedVars(params *sharedVarParams) error {
	if len(params.sharedVars) == consts.IndexZero {
		return nil
	}

	rootVars, err := rootVarsMappingNode(params.root)
	if err != nil {
		return fmt.Errorf("root vars mapping node: %w", err)
	}

	addSharedVarsToRoot(rootVars, params)

	return nil
}

func rootVarsMappingNode(root *yaml.Node) (*yaml.Node, error) {
	rootVars, err := findOrCreateMappingNode(root, keyVars, "root Taskfile vars must be a mapping")
	if err != nil {
		return nil, fmt.Errorf("find or create vars mapping node: %w", err)
	}

	return rootVars, nil
}

func addSharedVarsToRoot(rootVars *yaml.Node, params *sharedVarParams) {
	existing := mappingKeys(rootVars)

	keys := sortedKeys(params.sharedVars)

	for i := range keys {
		addMissingSharedVar(&addSharedVarParams{
			rootVars: rootVars, tasks: params.tasks, moduleVars: params.moduleVars,
			key: keys[i], existing: existing,
		})
	}
}

func addMissingSharedVar(params *addSharedVarParams) {
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

func mergeIncludeVars(entry, moduleVars *yaml.Node, sharedVars map[string]struct{}) {
	if moduleVars == nil || moduleVars.Kind != yaml.MappingNode {
		return
	}

	existingVars, ok := resolveExistingVars(entry, moduleVars, sharedVars)

	if !ok {
		return
	}

	mergeModuleVarsInto(existingVars, moduleVars, sharedVars)
}

func mergeModuleVarsInto(existVars, modVars *yaml.Node, shr strSet) {
	existingKeys := mappingKeys(existVars)

	for idx := consts.IndexZero; idx < len(modVars.Content); idx += yamlMappingPairKeyValue {
		mergeOneModuleVar(&mergeModuleVarParams{
			existingVars: existVars,
			keyNode:      modVars.Content[idx],
			valueNode:    modVars.Content[idx+consts.IndexOne],
			sharedVars:   shr,
			existingKeys: existingKeys,
		})
	}
}

// resolveExistingVars returns the entry's existing vars mapping, creating one from
// moduleVars when absent. ok is false when there is nothing left to merge (either a
// fresh vars node was just created, or the existing one isn't a mapping).
func resolveExistingVars(entry, modVars *yaml.Node, shr strSet) (*yaml.Node, bool) {
	existingVars := findMappingValue(entry, keyVars)

	if existingVars == nil {
		appendMappingPair(entry, yamlScalar(keyVars), includeVarsNode(modVars, shr))

		return nil, false
	}

	if existingVars.Kind != yaml.MappingNode {
		return nil, false
	}

	return existingVars, true
}

func mergeOneModuleVar(params *mergeModuleVarParams) {
	key := params.keyNode.Value

	if mergeSharedModuleVar(params, key) {
		return
	}

	if skipExistingModuleVar(params, key) {
		return
	}

	appendMappingPair(
		params.existingVars,
		cloneYAMLNode(params.keyNode),
		cloneYAMLNode(params.valueNode),
	)
}

func mergeSharedModuleVar(params *mergeModuleVarParams, key string) bool {
	sharedVal, shared := params.sharedVars[key]
	iox.Discard(sharedVal)

	if !shared {
		return false
	}

	setMappingValue(params.existingVars, key, rootVarReference(key))

	return true
}

func skipExistingModuleVar(params *mergeModuleVarParams, key string) bool {
	existingKeyVal, ok := params.existingKeys[key]
	iox.Discard(existingKeyVal)

	return ok
}

func includeVarsNode(moduleVars *yaml.Node, sharedVars map[string]struct{}) *yaml.Node {
	out := newYAMLMappingNode()

	for idx := consts.IndexZero; idx < len(moduleVars.Content); idx += yamlMappingPairKeyValue {
		key := moduleVars.Content[idx].Value

		value := cloneYAMLNode(moduleVars.Content[idx+consts.IndexOne])

		sharedVal, shared := sharedVars[key]
		iox.Discard(sharedVal)

		if shared {
			value = rootVarReference(key)
		}

		appendMappingPair(out, cloneYAMLNode(moduleVars.Content[idx]), value)
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

func newIncludeEntry(path string, modVars *yaml.Node, shr strSet) *yaml.Node {
	entry := newYAMLMappingNode()
	appendMappingPair(entry, yamlScalar(keyTaskfile), yamlScalar(path))

	if modVars != nil {
		appendMappingPair(entry, yamlScalar(keyVars), includeVarsNode(modVars, shr))
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
