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

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/yamlfmt"
	yaml "go.yaml.in/yaml/v3"
)

type (
	// RewriteError reports Taskfile YAML rewrite failures.
	RewriteError struct {
		Message string
	}

	// RootUpdateInput carries data for updating the root Taskfile includes section.
	RootUpdateInput struct {
		Tasks            []string
		TargetFolder     string
		RootTaskfileDir  string
		DestByTask       map[string]string
		ManagedTasks     []string
		ModuleTaskfiles  map[string][]byte
		GeneratedTasks   []GeneratedRootTask
		ManagedRootTasks []string
	}

	// GeneratedRootTask describes a TaskOtter-managed root task that fans out to
	// matching tasks in synced module includes.
	GeneratedRootTask struct {
		Name    string
		Modules []string
	}
)

const (
	rootTaskfileVersion     = "3.5"
	rootTemplate            = "---\nversion: \"" + rootTaskfileVersion + "\"\n"
	yamlMappingPairKeyValue = 2

	dotSlash    = "./"
	dotDotSlash = "../"
	keyIncludes = "includes"
	keyTaskfile = "taskfile"
	keyVars     = "vars"
	keyTasks    = "tasks"
)

var (
	errNoModuleVars = errors.New("module Taskfile has no vars")

	// minSharedVarTasks is the minimum number of tasks that must share a module
	// var before it is promoted to the root Taskfile's shared vars.
	minSharedVarTasks = yamlMappingPairKeyValue
)

// NewRootTemplate returns the minimal root Taskfile used when none exists yet.
func NewRootTemplate() []byte {
	return []byte(rootTemplate)
}

// Error implements the error interface, returning the Taskfile rewrite failure message.
func (e *RewriteError) Error() string {
	return e.Message
}

// RewriteIncludes updates include taskfile paths using sourceToDest mappings.
func RewriteIncludes(content []byte, sourceToDest map[string]string) ([]byte, error) {
	node, root, err := parseTaskfileRoot(content, "parse Taskfile YAML: %v", "empty Taskfile YAML")
	if err != nil {
		return nil, fmt.Errorf("parse taskfile root: %w", err)
	}

	includesNode := findMappingValue(root, keyIncludes)

	if includesNode != nil {
		rewriteIncludesNode(includesNode, sourceToDest)
	}

	out, err := marshalAndValidate(
		node,
		"marshal Taskfile YAML: %v",
		"validate rewritten Taskfile YAML: %v",
	)
	if err != nil {
		return nil, fmt.Errorf("marshal and validate: %w", err)
	}

	return out, nil
}

// parseTaskfileRoot unmarshals content into a YAML document node and returns its
// root mapping node. parseErrMsg and emptyErrMsg format the respective failures.
func parseTaskfileRoot(
	content []byte,
	parseErrMsg, emptyErrMsg string,
) (*yaml.Node, *yaml.Node, error) {
	var node yaml.Node

	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, nil, &RewriteError{Message: fmt.Sprintf(parseErrMsg, err)}
	}

	if len(node.Content) == consts.IndexZero {
		return nil, nil, &RewriteError{Message: emptyErrMsg}
	}

	return &node, node.Content[consts.IndexZero], nil
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

func rewriteIncludesNode(includes *yaml.Node, sourceToDest map[string]string) {
	if includes.Kind != yaml.MappingNode {
		return
	}

	for idx := consts.IndexZero; idx < len(includes.Content); idx += yamlMappingPairKeyValue {
		rewriteOneIncludeEntry(includes.Content[idx+consts.IndexOne], sourceToDest)
	}
}

func rewriteOneIncludeEntry(entry *yaml.Node, sourceToDest map[string]string) {
	if entry.Kind != yaml.MappingNode {
		return
	}

	taskfileNode := findMappingValue(entry, keyTaskfile)

	if taskfileNode == nil || taskfileNode.Kind != yaml.ScalarNode {
		return
	}

	taskfileNode.Value = rewriteIncludePath(taskfileNode.Value, sourceToDest)
}

func rewriteIncludePath(path string, sourceToDest map[string]string) string {
	normalized := filepath.ToSlash(path)

	if !strings.HasSuffix(normalized, consts.TaskfileSuffix) {
		return path
	}

	prefix, dir := splitRelativePrefix(strings.TrimSuffix(normalized, consts.TaskfileSuffix))

	if dir == consts.Empty {
		return path
	}

	dest, ok := sourceToDest[dir]

	if !ok {
		return path
	}

	return prefix + dest + consts.TaskfileSuffix
}

// splitRelativePrefix separates the leading ./ or ../ segments from the module
// directory. Namespaced modules such as internal/skipfiles keep their slash in
// the returned directory so it can be matched against source module names, and
// they sit one level deeper, so their siblings are reached through ../../.
func splitRelativePrefix(dir string) (string, string) {
	prefix := consts.Empty

	if rest, ok := strings.CutPrefix(dir, dotSlash); ok {
		prefix += dotSlash

		dir = rest
	}

	prefix, dir = stripDotDotSlashes(prefix, dir)

	if strings.Contains(dir, consts.PathParent) {
		return consts.Empty, consts.Empty
	}

	return prefix, dir
}

func stripDotDotSlashes(prefix, dir string) (string, string) {
	var prefixSb strings.Builder

	_, _ = prefixSb.WriteString(prefix) //nolint:errcheck // strings.Builder cannot fail

	for {
		rest, ok := strings.CutPrefix(dir, dotDotSlash)

		if !ok {
			return prefixSb.String(), dir
		}

		_, _ = prefixSb.WriteString(dotDotSlash) //nolint:errcheck // strings.Builder cannot fail

		dir = rest
	}
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
func UpdateRootTaskfile(content []byte, input RootUpdateInput) ([]byte, error) {
	node, root, err := parseTaskfileRoot(
		content,
		"parse root Taskfile YAML: %v",
		"empty root Taskfile YAML",
	)
	if err != nil {
		return nil, fmt.Errorf("parse taskfile root: %w", err)
	}

	setRootTaskfileVersion(root)

	err = applyRootUpdates(root, input)
	if err != nil {
		return nil, fmt.Errorf("apply root updates: %w", err)
	}

	out, err := marshalAndValidate(
		node,
		"marshal root Taskfile YAML: %v",
		"validate root Taskfile YAML: %v",
	)
	if err != nil {
		return nil, fmt.Errorf("marshal and validate: %w", err)
	}

	return out, nil
}

func applyRootUpdates(root *yaml.Node, input RootUpdateInput) error {
	moduleVars, sharedVars, err := applyRootVars(root, input)
	if err != nil {
		return fmt.Errorf("apply root vars: %w", err)
	}

	err = applyRootIncludesAndTasks(root, input, moduleVars, sharedVars)
	if err != nil {
		return fmt.Errorf("apply root includes and tasks: %w", err)
	}

	return nil
}

func applyRootVars(
	root *yaml.Node,
	input RootUpdateInput,
) (map[string]*yaml.Node, map[string]struct{}, error) {
	moduleVars, err := moduleVarsByTask(input)
	if err != nil {
		return nil, nil, fmt.Errorf("module vars by task: %w", err)
	}

	sharedVars := sharedModuleVarNames(input.Tasks, moduleVars)

	err = upsertRootSharedVars(root, input.Tasks, moduleVars, sharedVars)
	if err != nil {
		return nil, nil, fmt.Errorf("upsert root shared vars: %w", err)
	}

	return moduleVars, sharedVars, nil
}

func applyRootIncludesAndTasks(
	root *yaml.Node,
	input RootUpdateInput,
	moduleVars map[string]*yaml.Node,
	sharedVars map[string]struct{},
) error {
	includesNode, existing, err := prepareIncludesNode(root, input)
	if err != nil {
		return fmt.Errorf("prepare includes node: %w", err)
	}

	err = upsertManagedIncludes(includesNode, input, existing, moduleVars, sharedVars)
	if err != nil {
		return fmt.Errorf("upsert managed includes: %w", err)
	}

	err = updateGeneratedRootTasks(root, input)
	if err != nil {
		return fmt.Errorf("update generated root tasks: %w", err)
	}

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

func prepareIncludesNode(
	root *yaml.Node,
	input RootUpdateInput,
) (*yaml.Node, map[string]*yaml.Node, error) {
	includesNode, err := findOrCreateMappingNode(
		root,
		keyIncludes,
		"root Taskfile includes must be a mapping",
	)
	if err != nil {
		return nil, nil, fmt.Errorf("find or create includes mapping node: %w", err)
	}

	managedSet := managedTaskSet(input.Tasks)
	existing := existingIncludeEntries(includesNode)

	pruneRemovedManagedIncludes(includesNode, existing, managedSet, input.ManagedTasks)

	return includesNode, existing, nil
}

func managedTaskSet(tasks []string) map[string]struct{} {
	managedSet := make(map[string]struct{}, len(tasks))

	for _, task := range tasks {
		managedSet[task] = struct{}{}
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

func pruneRemovedManagedIncludes(
	includesNode *yaml.Node,
	existing map[string]*yaml.Node,
	managedSet map[string]struct{},
	managedTasks []string,
) {
	for alias := range existing {
		if _, managed := managedSet[alias]; managed {
			continue
		}

		if containsString(managedTasks, alias) {
			deleteMappingKey(includesNode, alias)
		}
	}
}

func upsertManagedIncludes(
	includesNode *yaml.Node,
	input RootUpdateInput,
	existing map[string]*yaml.Node,
	moduleVarsByTask map[string]*yaml.Node,
	sharedVars map[string]struct{},
) error {
	for _, task := range input.Tasks {
		err := upsertOneInclude(includesNode, input, existing, moduleVarsByTask, sharedVars, task)
		if err != nil {
			return fmt.Errorf("upsert include for task %q: %w", task, err)
		}
	}

	return nil
}

func upsertOneInclude(
	includesNode *yaml.Node,
	input RootUpdateInput,
	existing map[string]*yaml.Node,
	moduleVarsByTask map[string]*yaml.Node,
	sharedVars map[string]struct{},
	task string,
) error {
	dest, ok := input.DestByTask[task]

	if !ok {
		return &RewriteError{Message: fmt.Sprintf("missing destination for task %q", task)}
	}

	path := moduleIncludePath(input.RootTaskfileDir, input.TargetFolder, dest)
	moduleVars := moduleVarsByTask[task]

	if entry, ok := existing[task]; ok {
		err := updateExistingInclude(entry, path, moduleVars, sharedVars, input.ManagedTasks, task)
		if err != nil {
			return fmt.Errorf("update existing include: %w", err)
		}

		return nil
	}

	entry := newIncludeEntry(path, moduleVars, sharedVars)
	appendMappingPair(includesNode, yamlScalar(task), entry)

	return nil
}

func updateExistingInclude(
	entry *yaml.Node,
	path string,
	moduleVars *yaml.Node,
	sharedVars map[string]struct{},
	managedTasks []string,
	task string,
) error {
	if !isManagedInclude(entry, path, managedTasks, task) {
		return &RewriteError{
			Message: fmt.Sprintf(
				"include alias %q already exists and is not managed by TaskOtter",
				task,
			),
		}
	}

	setIncludePath(entry, path)
	mergeIncludeVars(entry, moduleVars, sharedVars)

	return nil
}

func isManagedInclude(
	entry *yaml.Node,
	expectedPath string,
	managedTasks []string,
	task string,
) bool {
	taskfileNode := findMappingValue(entry, keyTaskfile)

	if taskfileNode != nil {
		return taskfileNode.Value == expectedPath
	}

	if entry.Kind == yaml.ScalarNode {
		return entry.Value == expectedPath
	}

	return containsString(managedTasks, task)
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

	if varsNode == nil || varsNode.Kind != yaml.MappingNode ||
		len(varsNode.Content) == consts.IndexZero {
		return nil, errNoModuleVars
	}

	return cloneYAMLNode(varsNode), nil
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

func moduleVarsByTask(input RootUpdateInput) (map[string]*yaml.Node, error) {
	out := make(map[string]*yaml.Node, len(input.Tasks))

	for _, task := range input.Tasks {
		moduleVars, ok, err := tryExtractVarsNode(input.ModuleTaskfiles[task])
		if err != nil {
			return nil, fmt.Errorf("try extract vars node for task %q: %w", task, err)
		}

		if ok {
			out[task] = moduleVars
		}
	}

	return out, nil
}

func tryExtractVarsNode(content []byte) (*yaml.Node, bool, error) {
	moduleVars, err := extractVarsNode(content)
	if err != nil {
		if errors.Is(err, errNoModuleVars) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("extract vars node: %w", err)
	}

	return moduleVars, true, nil
}

func sharedModuleVarNames(
	tasks []string,
	moduleVarsByTask map[string]*yaml.Node,
) map[string]struct{} {
	counts := countVarKeysPerTask(tasks, moduleVarsByTask)
	shared := make(map[string]struct{})

	for key, count := range counts {
		if count >= minSharedVarTasks {
			shared[key] = struct{}{}
		}
	}

	return shared
}

func countVarKeysPerTask(tasks []string, moduleVarsByTask map[string]*yaml.Node) map[string]int {
	counts := make(map[string]int)

	for _, task := range tasks {
		for key := range varKeySet(moduleVarsByTask[task]) {
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

func upsertRootSharedVars(
	root *yaml.Node,
	tasks []string,
	moduleVarsByTask map[string]*yaml.Node,
	sharedVars map[string]struct{},
) error {
	if len(sharedVars) == consts.IndexZero {
		return nil
	}

	rootVars, err := findOrCreateMappingNode(root, keyVars, "root Taskfile vars must be a mapping")
	if err != nil {
		return fmt.Errorf("find or create vars mapping node: %w", err)
	}

	existing := mappingKeys(rootVars)

	for _, key := range sortedKeys(sharedVars) {
		addMissingSharedVar(rootVars, tasks, moduleVarsByTask, key, existing)
	}

	return nil
}

func addMissingSharedVar(
	rootVars *yaml.Node,
	tasks []string,
	moduleVarsByTask map[string]*yaml.Node,
	key string,
	existing map[string]struct{},
) {
	if _, ok := existing[key]; ok {
		return
	}

	value := firstVarValue(tasks, moduleVarsByTask, key)

	if value == nil {
		return
	}

	appendMappingPair(rootVars, yamlScalar(key), value)
}

func firstVarValue(
	tasks []string,
	moduleVarsByTask map[string]*yaml.Node,
	key string,
) *yaml.Node {
	for _, task := range tasks {
		value := varValueIn(moduleVarsByTask[task], key)

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

func mergeIncludeVars(entry *yaml.Node, moduleVars *yaml.Node, sharedVars map[string]struct{}) {
	if moduleVars == nil || moduleVars.Kind != yaml.MappingNode {
		return
	}

	existingVars, ok := resolveExistingVars(entry, moduleVars, sharedVars)

	if !ok {
		return
	}

	existingKeys := mappingKeys(existingVars)

	for idx := consts.IndexZero; idx < len(moduleVars.Content); idx += yamlMappingPairKeyValue {
		mergeOneModuleVar(
			existingVars,
			moduleVars.Content[idx],
			moduleVars.Content[idx+consts.IndexOne],
			sharedVars,
			existingKeys,
		)
	}
}

// resolveExistingVars returns the entry's existing vars mapping, creating one from
// moduleVars when absent. ok is false when there is nothing left to merge (either a
// fresh vars node was just created, or the existing one isn't a mapping).
func resolveExistingVars(
	entry *yaml.Node,
	moduleVars *yaml.Node,
	sharedVars map[string]struct{},
) (*yaml.Node, bool) {
	existingVars := findMappingValue(entry, keyVars)

	if existingVars == nil {
		appendMappingPair(entry, yamlScalar(keyVars), includeVarsNode(moduleVars, sharedVars))

		return nil, false
	}

	if existingVars.Kind != yaml.MappingNode {
		return nil, false
	}

	return existingVars, true
}

func mergeOneModuleVar(
	existingVars *yaml.Node,
	keyNode, valueNode *yaml.Node,
	sharedVars map[string]struct{},
	existingKeys map[string]struct{},
) {
	key := keyNode.Value

	if _, shared := sharedVars[key]; shared {
		setMappingValue(existingVars, key, rootVarReference(key))

		return
	}

	if _, ok := existingKeys[key]; ok {
		return
	}

	appendMappingPair(existingVars, cloneYAMLNode(keyNode), cloneYAMLNode(valueNode))
}

func includeVarsNode(moduleVars *yaml.Node, sharedVars map[string]struct{}) *yaml.Node {
	out := newYAMLMappingNode()

	for idx := consts.IndexZero; idx < len(moduleVars.Content); idx += yamlMappingPairKeyValue {
		key := moduleVars.Content[idx].Value

		value := cloneYAMLNode(moduleVars.Content[idx+consts.IndexOne])

		if _, shared := sharedVars[key]; shared {
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

func newIncludeEntry(
	path string,
	moduleVars *yaml.Node,
	sharedVars map[string]struct{},
) *yaml.Node {
	entry := newYAMLMappingNode()
	appendMappingPair(entry, yamlScalar(keyTaskfile), yamlScalar(path))

	if moduleVars != nil {
		appendMappingPair(entry, yamlScalar(keyVars), includeVarsNode(moduleVars, sharedVars))
	}

	return entry
}

func updateGeneratedRootTasks(root *yaml.Node, input RootUpdateInput) error {
	if len(input.GeneratedTasks) == consts.IndexZero &&
		len(input.ManagedRootTasks) == consts.IndexZero {
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

	generatedSet := generatedTaskSet(input.GeneratedTasks)

	removeStaleGeneratedTasks(tasksNode, input.ManagedRootTasks, generatedSet)
	applyGeneratedTasks(tasksNode, input.GeneratedTasks)

	return nil
}

func generatedTaskSet(generatedTasks []GeneratedRootTask) map[string]struct{} {
	generatedSet := make(map[string]struct{}, len(generatedTasks))

	for _, generated := range generatedTasks {
		generatedSet[generated.Name] = struct{}{}
	}

	return generatedSet
}

func removeStaleGeneratedTasks(
	tasksNode *yaml.Node,
	managedRootTasks []string,
	generatedSet map[string]struct{},
) {
	for _, old := range managedRootTasks {
		if _, stillGenerated := generatedSet[old]; stillGenerated {
			continue
		}

		deleteMappingKey(tasksNode, old)
	}
}

func applyGeneratedTasks(tasksNode *yaml.Node, generatedTasks []GeneratedRootTask) {
	for _, generated := range generatedTasks {
		deleteMappingKey(tasksNode, generated.Name)
		appendMappingPair(tasksNode, yamlScalar(generated.Name), newGeneratedTaskEntry(generated))
	}
}

func newGeneratedTaskEntry(generated GeneratedRootTask) *yaml.Node {
	entry := newYAMLMappingNode()
	appendMappingPair(
		entry,
		yamlScalar("desc"),
		yamlScalar("Run "+generated.Name+" for synced TaskOtter modules"),
	)

	cmds := newYAMLSequenceNode()

	for _, module := range generated.Modules {
		cmd := newYAMLMappingNode()
		appendMappingPair(cmd, yamlScalar("task"), yamlScalar(module+":"+generated.Name))

		cmds.Content = append(cmds.Content, cmd)
	}

	appendMappingPair(entry, yamlScalar("cmds"), cmds)

	return entry
}

func newYAMLMappingNode() *yaml.Node {
	return &yaml.Node{
		Kind:        yaml.MappingNode,
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

func newYAMLSequenceNode() *yaml.Node {
	return &yaml.Node{
		Kind:        yaml.SequenceNode,
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

func appendMappingPair(mapNode *yaml.Node, key, value *yaml.Node) {
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
				mapNode.Content[idx+yamlMappingPairKeyValue:]...)

			return
		}
	}
}

func containsString(list []string, target string) bool {
	return slices.Contains(list, target)
}
