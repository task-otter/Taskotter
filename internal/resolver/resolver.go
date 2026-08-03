// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package resolver maps logical task names to their resolved source modules.
package resolver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/variants"
)

type (

	// Resolution records the resolved source module for a logical task.
	Resolution struct {
		LogicalTask  string
		SourceModule string
	}

	// ResolveError reports task resolution failures with optional close matches.
	ResolveError struct {
		LogicalTask  string
		Attempted    string
		Message      string
		CloseMatches []string
	}

	// resolveContext bundles the catalog and JS settings shared across one task resolution.
	resolveContext struct {
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

// Error implements the error interface, returning the task resolution failure message.
func (e *ResolveError) Error() string {
	msg := fmt.Sprintf(`task %q`, e.LogicalTask)

	if e.Attempted != "" {
		msg += fmt.Sprintf(" (attempted source module %q)", e.Attempted)
	}

	msg += ": " + e.Message

	if len(e.CloseMatches) > consts.IndexZero {
		msg += "; close matches: " + strings.Join(e.CloseMatches, ", ")
	}

	return msg
}

// ResolveAll resolves each task against the store catalog.
func ResolveAll(
	tasks []string,
	catalog map[string]struct{},
	packageManager config.PackageManager,
	versionManager config.VersionManager,
) ([]Resolution, error) {
	var out []Resolution

	for i := range tasks {
		res, err := Resolve(tasks[i], catalog, packageManager, versionManager)
		if err != nil {
			return nil, fmt.Errorf("resolve task %q: %w", tasks[i], err)
		}

		out = append(out, res)
	}

	return out, nil
}

// Resolve maps one logical task to a store source module.
func Resolve(
	task string,
	catalog map[string]struct{},
	packageManager config.PackageManager,
	versionManager config.VersionManager,
) (Resolution, error) {
	rc := resolveContext{
		catalog:        catalog,
		packageManager: packageManager,
		versionManager: versionManager,
	}

	nodeVariants := findVariants(task, rc.catalog)

	if len(nodeVariants) == consts.IndexZero {
		res, err := resolvePlainTask(task, rc.catalog)
		if err != nil {
			return Resolution{}, fmt.Errorf("resolve plain task: %w", err)
		}

		return res, nil
	}

	res, err := resolveNodeVariant(task, &rc)
	if err != nil {
		return Resolution{}, fmt.Errorf("resolve node variant: %w", err)
	}

	return res, nil
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

func resolveNodeVariant(task string, rc *resolveContext) (Resolution, error) {
	if rc.packageManager == consts.Empty {
		return Resolution{}, &ResolveError{
			LogicalTask: task,
			Attempted:   consts.Empty,
			Message: fmt.Sprintf(
				`Task %q requires js configuration for Node tasks. Set js.runtime to bun or nodejs.`,
				task,
			),
			CloseMatches: nil,
		}
	}

	attempted, err := variants.BuildSourceModule(task, rc.packageManager, rc.versionManager)
	if err != nil {
		return Resolution{}, &ResolveError{
			LogicalTask:  task,
			Attempted:    consts.Empty,
			Message:      err.Error(),
			CloseMatches: nil,
		}
	}

	if _, ok := rc.catalog[attempted]; !ok {
		return Resolution{}, &ResolveError{
			LogicalTask:  task,
			Attempted:    attempted,
			Message:      "source module not found in store",
			CloseMatches: closeMatches(attempted, catalogKeys(rc.catalog), maxCloseMatches),
		}
	}

	return Resolution{LogicalTask: task, SourceModule: attempted}, nil
}

func findVariants(task string, catalog map[string]struct{}) []string {
	var out []string

	for name := range catalog {
		if variants.IsNodeToolVariant(name, task) {
			out = append(out, name)
		}
	}

	sort.Strings(out)

	return out
}

func catalogKeys(catalog map[string]struct{}) []string {
	keys := make([]string, consts.IndexZero, len(catalog))

	for key := range catalog {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func closeMatches(query string, candidates []string, limit int) []string {
	scores := scoreCandidates(query, candidates)
	sortByScoreDesc(scores)

	return topNames(scores, limit)
}

func scoreCandidates(query string, candidates []string) []scoredCandidate {
	var scores []scoredCandidate

	for i := range candidates {
		candidate := candidates[i]
		score := similarity(query, candidate)

		if score > consts.IndexZero {
			scores = append(scores, scoredCandidate{name: candidate, score: score})
		}
	}

	return scores
}

func sortByScoreDesc(scores []scoredCandidate) {
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].name < scores[j].name
		}

		return scores[i].score > scores[j].score
	})
}

func topNames(scores []scoredCandidate, limit int) []string {
	var out []string

	for i := consts.IndexZero; i < len(scores) && i < limit; i++ {
		out = append(out, scores[i].name)
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

func (s *dpState) computeRow(row int) {
	s.curr[consts.IndexZero] = row

	for col := consts.IndexOne; col <= len(s.right); col++ {
		cost := consts.IndexOne

		if s.left[row-consts.IndexOne] == s.right[col-consts.IndexOne] {
			cost = consts.IndexZero
		}

		s.curr[col] = minInt3(s.curr[col-1]+1, s.prev[col]+1, s.prev[col-1]+cost)
	}

	s.prev, s.curr = s.curr, s.prev
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
