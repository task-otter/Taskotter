// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

// Resolve implements DepsResolver.
func (transitiveResolver) Resolve(requested []string, deps map[string][]string) ([]string, error) {
	resolved, err := ResolveTransitive(requested, deps)
	if err != nil {
		return nil, fmt.Errorf("resolve transitive dependencies: %w", err)
	}

	return resolved, nil
}

// Error implements the error interface, returning the cyclic dependency chain.
func (e *CycleError) Error() string {
	return "dependency cycle detected: " + stringsJoinArrow(e.Path)
}

// Error implements the error interface, returning the missing dependency message.
func (e *MissingDependencyError) Error() string {
	return fmt.Sprintf("module %q depends on missing module %q", e.Module, e.Dependency)
}

func stringsJoinArrow(parts []string) string {
	return strings.Join(parts, " -> ")
}

func markVisited(state *visitState, module string) bool {
	if _, ok := state.needed[module]; ok {
		return true
	}

	state.needed[module] = struct{}{}

	return false
}

// ResolveTransitive returns dependency modules required by requested, excluding requested modules themselves.
func ResolveTransitive(requested []string, deps map[string][]string) ([]string, error) {
	state := &visitState{deps: deps, needed: make(map[string]struct{})}

	for i := range requested {
		err := visitModule(requested[i], nil, state)
		if err != nil {
			return nil, fmt.Errorf(errFmtVisitModule, requested[i], err)
		}
	}

	return transitiveDependencies(requested, state.needed), nil
}

func visitModule(module string, stack []string, state *visitState) error {
	err := validateVisitModule(module, stack, state)
	if err != nil {
		return fmt.Errorf("validate visit module %q: %w", module, err)
	}

	if markVisited(state, module) {
		return nil
	}

	err = visitModuleChildren(module, stack, state)
	if err != nil {
		return fmt.Errorf("visit module %q children: %w", module, err)
	}

	return nil
}

func validateVisitModule(module string, stack []string, state *visitState) error {
	err := checkModuleDefined(module, state.deps)
	if err != nil {
		return fmt.Errorf("check module defined: %w", err)
	}

	err = detectCycle(module, stack)
	if err != nil {
		return fmt.Errorf("detect cycle: %w", err)
	}

	return nil
}

func visitModuleChildren(module string, stack []string, state *visitState) error {
	err := visitDependencies(module, stack, state)
	if err != nil {
		return fmt.Errorf("visit module %q deps: %w", module, err)
	}

	return nil
}

func checkModuleDefined(module string, deps map[string][]string) error {
	if _, ok := deps[module]; !ok {
		return fmt.Errorf("%w: %q", errModuleNotDefined, module)
	}

	return nil
}

func detectCycle(module string, stack []string) error {
	for i := range stack {
		if stack[i] == module {
			cycle := append(append([]string{}, stack[i:]...), module)

			return &CycleError{Path: cycle}
		}
	}

	return nil
}

func visitDependencies(module string, stack []string, state *visitState) error {
	moduleDeps := state.deps[module]

	for i := range moduleDeps {
		dep := moduleDeps[i]

		ctx := &visitContext{state: state, stack: stack}

		err := visitDependency(dep, module, ctx)
		if err != nil {
			return fmt.Errorf("visit dependency %q: %w", dep, err)
		}
	}

	return nil
}

func visitDependency(dep, module string, ctx *visitContext) error {
	if _, ok := ctx.state.deps[dep]; !ok {
		return &MissingDependencyError{Module: module, Dependency: dep}
	}

	err := visitModule(dep, append(ctx.stack, module), ctx.state)
	if err != nil {
		return fmt.Errorf(errFmtVisitModule, dep, err)
	}

	return nil
}

func transitiveDependencies(requested []string, needed map[string]struct{}) []string {
	requestedSet := make(map[string]struct{}, len(requested))

	for i := range requested {
		requestedSet[requested[i]] = struct{}{}
	}

	dependencies := make([]string, consts.IndexZero, len(needed))

	for module := range needed {
		if _, ok := requestedSet[module]; ok {
			continue
		}

		dependencies = append(dependencies, module)
	}

	slices.Sort(dependencies)

	return dependencies
}

// Error implements the error interface, returning the destination collision message.
func (e *CollisionError) Error() string {
	return fmt.Sprintf(
		`Destination collision: %q and %q both normalize to destination module %q.`,
		e.SourceA, e.SourceB, e.Destination,
	)
}

// Normalize strips package-manager and version-manager suffixes from a source module name.
func Normalize(source string) (string, error) {
	current := source

	for {
		next, changed := StripOneSuffix(current)

		if !changed {
			break
		}

		current = next
	}

	if current == "" {
		return "", fmt.Errorf("%w for %q", errEmptyNormalizedName, source)
	}

	return current, nil
}

// BuildDestinationMap normalizes each source module and rejects destination collisions.
func BuildDestinationMap(sources []string) (map[string]string, error) {
	destToSource := make(map[string]string, len(sources))
	result := make(map[string]string, len(sources))

	for i := range sources {
		err := recordDestination(sources[i], destToSource, result)
		if err != nil {
			return nil, fmt.Errorf("record destination for %q: %w", sources[i], err)
		}
	}

	return result, nil
}

func recordDestination(source string, destToSource, result map[string]string) error {
	dest, err := Normalize(source)
	if err != nil {
		return fmt.Errorf("normalize %q: %w", source, err)
	}

	if existing, ok := destToSource[dest]; ok && existing != source {
		return &CollisionError{SourceA: existing, SourceB: source, Destination: dest}
	}

	destToSource[dest] = source
	result[source] = dest

	return nil
}

// SortedSources returns map keys sorted lexicographically.
func SortedSources(m map[string]string) []string {
	out := make([]string, consts.IndexZero, len(m))

	for source := range m {
		out = append(out, source)
	}

	slices.Sort(out)

	return out
}

// ResolveAll resolves each task against the store catalog.
func ResolveAll(input *ResolveAllInput) ([]Resolution, error) {
	out := make([]Resolution, consts.IndexZero, len(input.Tasks))

	for idx := range input.Tasks {
		task := input.Tasks[idx]

		res, err := Resolve(&ResolveInput{
			Task:           task,
			Catalog:        input.Catalog,
			PackageManager: input.PackageManager,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve task %q: %w", task, err)
		}

		out = append(out, res)
	}

	return out, nil
}

// Resolve maps one logical task to a store source module.
func Resolve(input *ResolveInput) (Resolution, error) {
	taskCtx := taskContext{
		catalog:        input.Catalog,
		packageManager: input.PackageManager,
	}

	res, err := resolveWithTaskContext(input.Task, &taskCtx)
	if err != nil {
		return Resolution{}, fmt.Errorf("resolve task: %w", err)
	}

	return res, nil
}

func resolveWithTaskContext(task string, taskCtx *taskContext) (Resolution, error) {
	resolveFn := pickTaskResolver(task, taskCtx)

	res, err := resolveFn()
	if err != nil {
		return Resolution{}, fmt.Errorf("resolve module: %w", err)
	}

	return res, nil
}

func pickTaskResolver(task string, taskCtx *taskContext) func() (Resolution, error) {
	if len(findVariants(task, taskCtx.catalog)) == consts.IndexZero {
		return func() (Resolution, error) {
			return resolvePlainTask(task, taskCtx.catalog)
		}
	}

	return func() (Resolution, error) {
		return resolveNodeVariant(task, taskCtx)
	}
}

func resolvePlainTask(task string, catalog map[string]struct{}) (Resolution, error) {
	if _, ok := catalog[task]; ok {
		return Resolution{LogicalTask: task, SourceModule: task}, nil
	}

	return Resolution{}, &ResolveError{
		LogicalTask:  task,
		Attempted:    consts.Empty,
		Message:      "task not found in store",
		CloseMatches: closeMatches(task, catalogKeys(catalog), maxCloseMatches),
	}
}

func resolveNodeVariant(task string, taskCtx *taskContext) (Resolution, error) {
	if taskCtx.packageManager == consts.Empty {
		return Resolution{}, nodeVariantMissingJSConfigError(task)
	}

	attempted, err := BuildSourceModule(task, taskCtx.packageManager)
	if err != nil {
		return Resolution{}, nodeVariantBuildError(task, err)
	}

	res, err := resolveAttemptedModule(task, attempted, taskCtx.catalog)
	if err != nil {
		return Resolution{}, fmt.Errorf("resolve attempted module: %w", err)
	}

	return res, nil
}

func nodeVariantMissingJSConfigError(task string) *ResolveError {
	return &ResolveError{
		LogicalTask: task,
		Attempted:   consts.Empty,
		Message: fmt.Sprintf(
			`Task %q requires js configuration for Node tasks. Set js.runtime to bun or nodejs.`,
			task,
		),
		CloseMatches: nil,
	}
}

func nodeVariantBuildError(task string, buildErr error) *ResolveError {
	return &ResolveError{
		LogicalTask:  task,
		Attempted:    consts.Empty,
		Message:      buildErr.Error(),
		CloseMatches: nil,
	}
}

func resolveAttemptedModule(task, module string, catalog map[string]struct{}) (Resolution, error) {
	if _, ok := catalog[module]; ok {
		return Resolution{LogicalTask: task, SourceModule: module}, nil
	}

	return Resolution{}, &ResolveError{
		LogicalTask:  task,
		Attempted:    module,
		Message:      "source module not found in store",
		CloseMatches: closeMatches(module, catalogKeys(catalog), maxCloseMatches),
	}
}

func findVariants(task string, catalog map[string]struct{}) []string {
	out := make([]string, consts.IndexZero, len(catalog))

	for name := range catalog {
		if IsNodeToolVariant(name, task) {
			out = append(out, name)
		}
	}

	slices.Sort(out)

	return out
}

func catalogKeys(catalog map[string]struct{}) []string {
	keys := make([]string, consts.IndexZero, len(catalog))

	for key := range catalog {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

func closeMatches(query string, candidates []string, limit int) []string {
	scores := scoreCandidates(query, candidates)
	sortByScoreDesc(scores)

	return topNames(scores, limit)
}

func scoreCandidates(query string, candidates []string) []scoredCandidate {
	scores := make([]scoredCandidate, consts.IndexZero, len(candidates))

	for idx := range candidates {
		candidate := candidates[idx]
		score := similarity(query, candidate)

		if score > consts.IndexZero {
			scores = append(scores, scoredCandidate{name: candidate, score: score})
		}
	}

	return scores
}

func sortByScoreDesc(scores []scoredCandidate) {
	slices.SortFunc(scores, compareScoredCandidates)
}

func compareScoredCandidates(left, right scoredCandidate) int {
	if left.score == right.score {
		return strings.Compare(left.name, right.name)
	}

	if left.score > right.score {
		return -consts.IndexOne
	}

	return consts.IndexOne
}

func topNames(scores []scoredCandidate, limit int) []string {
	capacity := min(len(scores), limit)
	out := make([]string, consts.IndexZero, capacity)

	for idx := consts.IndexZero; idx < len(scores) && idx < limit; idx++ {
		out = append(out, scores[idx].name)
	}

	return out
}

func similarity(left, right string) int {
	if left == right {
		return scoreExactMatch
	}

	if strings.HasPrefix(right, left) || strings.HasPrefix(left, right) {
		return scorePrefixMatchBase + min(len(left), len(right))
	}

	return levenshteinDistance(left, right)
}

// Levenshtein calculates the string distance similarity score between left and right.
func Levenshtein(left, right string) int {
	return levenshteinDistance(left, right)
}

func levenshteinDistance(left, right string) int {
	if left == right {
		return scoreIdenticalString
	}

	leftLen, rightLen := len(left), len(right)

	if leftLen == consts.IndexZero || rightLen == consts.IndexZero {
		return consts.IndexZero
	}

	dist := editDistance(left, right)
	maxLen := max(leftLen, rightLen)

	return max(consts.IndexZero, scoreIdenticalString-(dist*scoreIdenticalString/maxLen))
}

func editDistance(left, right string) int {
	rightLen := len(right)
	state := &dpState{
		left:  left,
		right: right,
		prev:  make([]int, rightLen+1),
		curr:  make([]int, rightLen+1),
	}

	for col := consts.IndexZero; col <= rightLen; col++ {
		state.prev[col] = col
	}

	for row := consts.IndexOne; row <= len(left); row++ {
		state.computeRow(row)
	}

	return state.prev[rightLen]
}

func (state *dpState) computeRow(row int) {
	state.curr[consts.IndexZero] = row

	for col := consts.IndexOne; col <= len(state.right); col++ {
		cost := consts.IndexOne

		if state.left[row-consts.IndexOne] == state.right[col-consts.IndexOne] {
			cost = consts.IndexZero
		}

		state.curr[col] = minInt3(state.curr[col-1]+1, state.prev[col]+1, state.prev[col-1]+cost)
	}

	state.prev, state.curr = state.curr, state.prev
}

func minInt3(first, second, third int) int {
	return min(first, min(second, third))
}

func nodeToolSuffixes() []string {
	return []string{
		"node/npm", "node/yarn", "node/pnpm",
		"bun",
	}
}

func stripSuffixes() []string {
	return []string{
		"/node/npm",
		"/node/yarn",
		"/node/pnpm",
		"/bun",
	}
}

// IsNodeToolVariant reports whether moduleName is a Node variant of logicalTask.
func IsNodeToolVariant(moduleName, logicalTask string) bool {
	prefix := logicalTask + "/"

	if len(moduleName) <= len(prefix) {
		return false
	}

	if moduleName[:len(prefix)] != prefix {
		return false
	}

	suffix := strings.TrimPrefix(moduleName[len(prefix):], "/")

	return slices.Contains(nodeToolSuffixes(), suffix)
}

// BuildSourceModule constructs the store module name for a logical task and JS configuration.
func BuildSourceModule(task string, packageManager pkgMgr) (string, error) {
	switch packageManager {
	case config.JSRuntimeBun:
		return path.Join(task, packageManager), nil
	case config.PMNPM, config.PMYarn, config.PMPnpm:
		return path.Join(task, "node", packageManager), nil
	default:
		return consts.Empty, fmt.Errorf(fmtWrapQuoted, errInvalidPackageManager, packageManager)
	}
}

// StripOneSuffix removes one known suffix from the end of name.
func StripOneSuffix(name string) (string, bool) {
	suffixes := stripSuffixes()

	for i := range suffixes {
		suffix := suffixes[i]

		if !hasSuffixLongerThan(name, suffix) {
			continue
		}

		return name[:len(name)-len(suffix)], true
	}

	return name, false
}

func hasSuffixLongerThan(name, suffix string) bool {
	return len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix
}
