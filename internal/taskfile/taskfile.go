// Package taskfile rewrites root and module Taskfile YAML during sync.
package taskfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/yamlfmt"
	"gopkg.in/yaml.v3"
)

const (
	rootTaskfileVersion     = "3.5"
	rootTemplate            = "---\nversion: \"" + rootTaskfileVersion + "\"\n"
	yamlMappingPairKeyValue = 2
)

var errNoModuleVars = errors.New("module Taskfile has no vars")

// NewRootTemplate returns the minimal root Taskfile used when none exists yet.
func NewRootTemplate() []byte {
	return []byte(rootTemplate)
}

// RewriteError reports Taskfile YAML rewrite failures.
type RewriteError struct {
	Message string
}

func (e *RewriteError) Error() string {
	return e.Message
}

// RewriteIncludes updates include taskfile paths using sourceToDest mappings.
func RewriteIncludes(content []byte, sourceToDest map[string]string) ([]byte, error) {
	var node yaml.Node

	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf("parse Taskfile YAML: %v", err)}
	}

	if len(node.Content) == 0 {
		return nil, &RewriteError{Message: "empty Taskfile YAML"}
	}

	root := node.Content[0]

	includesNode := findMappingValue(root, "includes")
	if includesNode != nil {
		rewriteIncludesNode(includesNode, sourceToDest)
	}

	out, err := yamlfmt.Marshal(&node)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf("marshal Taskfile YAML: %v", err)}
	}

	var validateNode yaml.Node

	err = yaml.Unmarshal(out, &validateNode)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf("validate rewritten Taskfile YAML: %v", err)}
	}

	return out, nil
}

func rewriteIncludesNode(includes *yaml.Node, sourceToDest map[string]string) {
	if includes.Kind != yaml.MappingNode {
		return
	}

	for idx := 0; idx < len(includes.Content); idx += yamlMappingPairKeyValue {
		entry := includes.Content[idx+1]
		if entry.Kind != yaml.MappingNode {
			continue
		}

		taskfileNode := findMappingValue(entry, "taskfile")
		if taskfileNode == nil || taskfileNode.Kind != yaml.ScalarNode {
			continue
		}

		taskfileNode.Value = rewriteIncludePath(taskfileNode.Value, sourceToDest)
	}
}

func rewriteIncludePath(path string, sourceToDest map[string]string) string {
	normalized := filepath.ToSlash(path)
	if !strings.HasSuffix(normalized, "/Taskfile.yml") {
		return path
	}

	prefix, dir := splitRelativePrefix(strings.TrimSuffix(normalized, "/Taskfile.yml"))
	if dir == "" {
		return path
	}

	dest, ok := sourceToDest[dir]
	if !ok {
		return path
	}

	return prefix + dest + "/Taskfile.yml"
}

// splitRelativePrefix separates the leading ./ or ../ segments from the module
// directory. Namespaced modules such as internal/skipfiles keep their slash in
// the returned directory so it can be matched against source module names, and
// they sit one level deeper, so their siblings are reached through ../../.
func splitRelativePrefix(dir string) (string, string) {
	var prefix strings.Builder

	if rest, ok := strings.CutPrefix(dir, "./"); ok {
		prefix.WriteString("./")

		dir = rest
	}

	for {
		rest, ok := strings.CutPrefix(dir, "../")
		if !ok {
			break
		}

		prefix.WriteString("../")

		dir = rest
	}

	if strings.Contains(dir, "..") {
		return "", ""
	}

	return prefix.String(), dir
}

func findMappingValue(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return nil
	}

	for idx := 0; idx < len(mapNode.Content); idx += yamlMappingPairKeyValue {
		keyNode := mapNode.Content[idx]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			return mapNode.Content[idx+1]
		}
	}

	return nil
}

// RootUpdateInput carries data for updating the root Taskfile includes section.
type RootUpdateInput struct {
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
type GeneratedRootTask struct {
	Name    string
	Modules []string
}

// moduleIncludePath returns the include taskfile path for a synced module,
// expressed relative to the directory that holds the aggregator Taskfile.
// When the aggregator sits at the workspace root the path is workspace-relative
// (for example taskfiles/go/Taskfile.yml); when it sits inside the target
// folder the path collapses to the module directory (for example go/Taskfile.yml).
func moduleIncludePath(rootDir, targetFolder, dest string) string {
	target := filepath.ToSlash(filepath.Join(targetFolder, dest, "Taskfile.yml"))
	if rootDir == "" || rootDir == "." {
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
	var node yaml.Node

	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf("parse root Taskfile YAML: %v", err)}
	}

	if len(node.Content) == 0 {
		return nil, &RewriteError{Message: "empty root Taskfile YAML"}
	}

	root := node.Content[0]
	setRootTaskfileVersion(root)

	moduleVars, err := moduleVarsByTask(input)
	if err != nil {
		return nil, err
	}

	sharedVars := sharedModuleVarNames(input.Tasks, moduleVars)

	err = upsertRootSharedVars(root, input.Tasks, moduleVars, sharedVars)
	if err != nil {
		return nil, err
	}

	includesNode, existing, err := prepareIncludesNode(root, input)
	if err != nil {
		return nil, err
	}

	err = upsertManagedIncludes(includesNode, input, existing, moduleVars, sharedVars)
	if err != nil {
		return nil, err
	}

	err = updateGeneratedRootTasks(root, input)
	if err != nil {
		return nil, err
	}

	out, err := yamlfmt.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("marshal root Taskfile YAML: %w", err)
	}

	var validateNode yaml.Node

	err = yaml.Unmarshal(out, &validateNode)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf("validate root Taskfile YAML: %v", err)}
	}

	return out, nil
}

func prepareIncludesNode(
	root *yaml.Node,
	input RootUpdateInput,
) (*yaml.Node, map[string]*yaml.Node, error) {
	managedSet := make(map[string]struct{}, len(input.Tasks))
	for _, task := range input.Tasks {
		managedSet[task] = struct{}{}
	}

	includesNode := findMappingValue(root, "includes")
	if includesNode == nil {
		includesNode = newYAMLMappingNode()
		appendMappingPair(root, yamlScalar("includes"), includesNode)
	}

	if includesNode.Kind != yaml.MappingNode {
		return nil, nil, &RewriteError{Message: "root Taskfile includes must be a mapping"}
	}

	existing := make(map[string]*yaml.Node)

	for idx := 0; idx < len(includesNode.Content); idx += yamlMappingPairKeyValue {
		keyNode := includesNode.Content[idx]
		existing[keyNode.Value] = includesNode.Content[idx+1]
	}

	pruneRemovedManagedIncludes(includesNode, existing, managedSet, input.ManagedTasks)

	return includesNode, existing, nil
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
		dest, ok := input.DestByTask[task]
		if !ok {
			return &RewriteError{Message: fmt.Sprintf("missing destination for task %q", task)}
		}

		path := moduleIncludePath(input.RootTaskfileDir, input.TargetFolder, dest)

		moduleVars := moduleVarsByTask[task]

		if entry, ok := existing[task]; ok {
			if !isManagedInclude(entry, path, input.ManagedTasks, task) {
				return &RewriteError{
					Message: fmt.Sprintf(
						"include alias %q already exists and is not managed by TaskOtter",
						task,
					),
				}
			}

			setIncludePath(entry, path)
			mergeIncludeVars(entry, moduleVars, sharedVars)

			continue
		}

		entry := newIncludeEntry(path, moduleVars, sharedVars)
		appendMappingPair(includesNode, yamlScalar(task), entry)
	}

	return nil
}

func isManagedInclude(
	entry *yaml.Node,
	expectedPath string,
	managedTasks []string,
	task string,
) bool {
	taskfileNode := findMappingValue(entry, "taskfile")
	if taskfileNode != nil {
		return taskfileNode.Value == expectedPath
	}

	if entry.Kind == yaml.ScalarNode {
		return entry.Value == expectedPath
	}

	return containsString(managedTasks, task)
}

func setIncludePath(entry *yaml.Node, path string) {
	taskfileNode := findMappingValue(entry, "taskfile")
	if taskfileNode == nil {
		appendMappingPair(entry, yamlScalar("taskfile"), yamlScalar(path))

		return
	}

	taskfileNode.Value = path
}

func extractVarsNode(content []byte) (*yaml.Node, error) {
	if len(content) == 0 {
		return nil, errNoModuleVars
	}

	var node yaml.Node

	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, &RewriteError{Message: fmt.Sprintf("parse module Taskfile YAML: %v", err)}
	}

	if len(node.Content) == 0 {
		return nil, errNoModuleVars
	}

	varsNode := findMappingValue(node.Content[0], "vars")
	if varsNode == nil || varsNode.Kind != yaml.MappingNode || len(varsNode.Content) == 0 {
		return nil, errNoModuleVars
	}

	return cloneYAMLNode(varsNode), nil
}

func moduleVarsByTask(input RootUpdateInput) (map[string]*yaml.Node, error) {
	out := make(map[string]*yaml.Node, len(input.Tasks))
	for _, task := range input.Tasks {
		moduleVars, err := extractVarsNode(input.ModuleTaskfiles[task])
		if err != nil {
			if errors.Is(err, errNoModuleVars) {
				continue
			}

			return nil, err
		}

		out[task] = moduleVars
	}

	return out, nil
}

func sharedModuleVarNames(
	tasks []string,
	moduleVarsByTask map[string]*yaml.Node,
) map[string]struct{} {
	counts := make(map[string]int)
	for _, task := range tasks {
		varsNode := moduleVarsByTask[task]
		if varsNode == nil || varsNode.Kind != yaml.MappingNode {
			continue
		}

		seen := make(map[string]struct{})
		for idx := 0; idx < len(varsNode.Content); idx += yamlMappingPairKeyValue {
			seen[varsNode.Content[idx].Value] = struct{}{}
		}

		for key := range seen {
			counts[key]++
		}
	}

	shared := make(map[string]struct{})
	for key, count := range counts {
		if count >= 2 {
			shared[key] = struct{}{}
		}
	}

	return shared
}

func upsertRootSharedVars(
	root *yaml.Node,
	tasks []string,
	moduleVarsByTask map[string]*yaml.Node,
	sharedVars map[string]struct{},
) error {
	if len(sharedVars) == 0 {
		return nil
	}

	rootVars := findMappingValue(root, "vars")
	if rootVars == nil {
		rootVars = newYAMLMappingNode()
		appendMappingPair(root, yamlScalar("vars"), rootVars)
	}

	if rootVars.Kind != yaml.MappingNode {
		return &RewriteError{Message: "root Taskfile vars must be a mapping"}
	}

	existing := mappingKeys(rootVars)
	for _, key := range sortedKeys(sharedVars) {
		if _, ok := existing[key]; ok {
			continue
		}

		value := firstVarValue(tasks, moduleVarsByTask, key)
		if value == nil {
			continue
		}

		appendMappingPair(rootVars, yamlScalar(key), value)
	}

	return nil
}

func firstVarValue(
	tasks []string,
	moduleVarsByTask map[string]*yaml.Node,
	key string,
) *yaml.Node {
	for _, task := range tasks {
		varsNode := moduleVarsByTask[task]
		if varsNode == nil || varsNode.Kind != yaml.MappingNode {
			continue
		}

		for idx := 0; idx < len(varsNode.Content); idx += yamlMappingPairKeyValue {
			if varsNode.Content[idx].Value == key {
				return cloneYAMLNode(varsNode.Content[idx+1])
			}
		}
	}

	return nil
}

func mergeIncludeVars(entry *yaml.Node, moduleVars *yaml.Node, sharedVars map[string]struct{}) {
	if moduleVars == nil || moduleVars.Kind != yaml.MappingNode {
		return
	}

	existingVars := findMappingValue(entry, "vars")
	if existingVars == nil {
		appendMappingPair(entry, yamlScalar("vars"), includeVarsNode(moduleVars, sharedVars))

		return
	}

	if existingVars.Kind != yaml.MappingNode {
		return
	}

	existingKeys := make(map[string]struct{}, len(existingVars.Content)/yamlMappingPairKeyValue)
	for idx := 0; idx < len(existingVars.Content); idx += yamlMappingPairKeyValue {
		existingKeys[existingVars.Content[idx].Value] = struct{}{}
	}

	for idx := 0; idx < len(moduleVars.Content); idx += yamlMappingPairKeyValue {
		key := moduleVars.Content[idx].Value
		if _, shared := sharedVars[key]; shared {
			setMappingValue(existingVars, key, rootVarReference(key))

			continue
		}

		if _, ok := existingKeys[key]; ok {
			continue
		}

		appendMappingPair(
			existingVars,
			cloneYAMLNode(moduleVars.Content[idx]),
			cloneYAMLNode(moduleVars.Content[idx+1]),
		)
	}
}

func includeVarsNode(moduleVars *yaml.Node, sharedVars map[string]struct{}) *yaml.Node {
	out := newYAMLMappingNode()
	for idx := 0; idx < len(moduleVars.Content); idx += yamlMappingPairKeyValue {
		key := moduleVars.Content[idx].Value
		value := cloneYAMLNode(moduleVars.Content[idx+1])
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
	if err != nil || len(out.Content) == 0 {
		return nil
	}

	return out.Content[0]
}

func newIncludeEntry(path string, moduleVars *yaml.Node, sharedVars map[string]struct{}) *yaml.Node {
	entry := newYAMLMappingNode()
	appendMappingPair(entry, yamlScalar("taskfile"), yamlScalar(path))

	if moduleVars != nil {
		appendMappingPair(entry, yamlScalar("vars"), includeVarsNode(moduleVars, sharedVars))
	}

	return entry
}

func updateGeneratedRootTasks(root *yaml.Node, input RootUpdateInput) error {
	if len(input.GeneratedTasks) == 0 && len(input.ManagedRootTasks) == 0 {
		return nil
	}

	tasksNode := findMappingValue(root, "tasks")
	if tasksNode == nil {
		tasksNode = newYAMLMappingNode()
		appendMappingPair(root, yamlScalar("tasks"), tasksNode)
	}

	if tasksNode.Kind != yaml.MappingNode {
		return &RewriteError{Message: "root Taskfile tasks must be a mapping"}
	}

	generatedSet := make(map[string]struct{}, len(input.GeneratedTasks))
	for _, generated := range input.GeneratedTasks {
		generatedSet[generated.Name] = struct{}{}
	}

	for _, old := range input.ManagedRootTasks {
		if _, stillGenerated := generatedSet[old]; stillGenerated {
			continue
		}

		deleteMappingKey(tasksNode, old)
	}

	for _, generated := range input.GeneratedTasks {
		deleteMappingKey(tasksNode, generated.Name)
		appendMappingPair(tasksNode, yamlScalar(generated.Name), newGeneratedTaskEntry(generated))
	}

	return nil
}

func newGeneratedTaskEntry(generated GeneratedRootTask) *yaml.Node {
	entry := newYAMLMappingNode()
	appendMappingPair(
		entry,
		yamlScalar("desc"),
		yamlScalar("Run "+generated.Name+" for synced TaskOtter modules"),
	)

	cmds := &yaml.Node{Kind: yaml.SequenceNode}
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
		Style:       0,
		Tag:         "",
		Value:       "",
		Anchor:      "",
		Alias:       nil,
		Content:     nil,
		HeadComment: "",
		LineComment: "",
		FootComment: "",
		Line:        0,
		Column:      0,
	}
}

func yamlScalar(value string) *yaml.Node {
	return &yaml.Node{
		Kind:        yaml.ScalarNode,
		Style:       0,
		Tag:         "",
		Value:       value,
		Anchor:      "",
		Alias:       nil,
		Content:     nil,
		HeadComment: "",
		LineComment: "",
		FootComment: "",
		Line:        0,
		Column:      0,
	}
}

func appendMappingPair(mapNode *yaml.Node, key, value *yaml.Node) {
	mapNode.Content = append(mapNode.Content, key, value)
}

func mappingKeys(mapNode *yaml.Node) map[string]struct{} {
	keys := make(map[string]struct{}, len(mapNode.Content)/yamlMappingPairKeyValue)
	for idx := 0; idx < len(mapNode.Content); idx += yamlMappingPairKeyValue {
		keys[mapNode.Content[idx].Value] = struct{}{}
	}

	return keys
}

func setMappingValue(mapNode *yaml.Node, key string, value *yaml.Node) {
	for idx := 0; idx < len(mapNode.Content); idx += yamlMappingPairKeyValue {
		if mapNode.Content[idx].Value == key {
			mapNode.Content[idx+1] = value

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
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

func deleteMappingKey(mapNode *yaml.Node, key string) {
	for idx := 0; idx < len(mapNode.Content); idx += yamlMappingPairKeyValue {
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
