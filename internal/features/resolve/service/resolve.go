// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package service maps logical task names to source modules and dependencies.
package service

import (
	"fmt"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/features/resolve/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	// Resolution is a resolved logical task and its source module.
	Resolution = domain.Resolution
	// ResolveError is a user-facing module resolution failure.
	ResolveError = domain.ResolveError

	// ResolveInput selects one logical task and JS runtime settings.
	ResolveInput struct {
		Task           string
		Catalog        map[string]struct{}
		PackageManager config.PackageManager
		VersionManager config.VersionManager
	}

	// ResolveAllInput resolves multiple logical tasks against one catalog.
	ResolveAllInput struct {
		Catalog        map[string]struct{}
		PackageManager config.PackageManager
		VersionManager config.VersionManager
		Tasks          []string
	}

	// taskContext bundles the catalog and JS settings shared across one task resolution.
	taskContext struct {
		catalog        map[string]struct{}
		packageManager config.PackageManager
		versionManager config.VersionManager
	}

	scoredCandidate struct {
		name  string
		score int
	}

	// dpState holds the rolling Levenshtein distance rows shared across computeRow calls.
	dpState struct {
		left, right string
		prev, curr  []int
	}
)

const (
	maxCloseMatches      = 5
	scoreExactMatch      = 1000
	scorePrefixMatchBase = 500
	scoreIdenticalString = 100
)

// ResolveAll resolves each task against the store catalog.
func ResolveAll(input *ResolveAllInput) ([]Resolution, error) {
	out := make([]Resolution, consts.IndexZero, len(input.Tasks))

	for idx := range input.Tasks {
		task := input.Tasks[idx]

		res, err := Resolve(&ResolveInput{
			Task:           task,
			Catalog:        input.Catalog,
			PackageManager: input.PackageManager,
			VersionManager: input.VersionManager,
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
		versionManager: input.VersionManager,
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

	attempted, err := BuildSourceModule(
		task,
		taskCtx.packageManager,
		taskCtx.versionManager,
	)
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
		return scorePrefixMatchBase + minInt(len(left), len(right))
	}

	return levenshtein(left, right)
}

func levenshtein(left, right string) int {
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

func minInt(first, second int) int {
	if first < second {
		return first
	}

	return second
}
