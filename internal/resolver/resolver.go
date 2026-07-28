// Package resolver maps logical task names to store source modules.
package resolver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/variants"
)

const (
	maxCloseMatches      = 5
	scoreExactMatch      = 1000
	scorePrefixMatchBase = 500
	scoreIdenticalString = 100
	maxSimilarityScore   = 100
)

// Resolution records the resolved source module for a logical task.
type Resolution struct {
	LogicalTask  string
	SourceModule string
}

// ResolveError reports task resolution failures with optional close matches.
type ResolveError struct {
	LogicalTask  string
	Attempted    string
	Message      string
	CloseMatches []string
}

func (e *ResolveError) Error() string {
	msg := fmt.Sprintf(`task %q`, e.LogicalTask)
	if e.Attempted != "" {
		msg += fmt.Sprintf(" (attempted source module %q)", e.Attempted)
	}

	msg += ": " + e.Message
	if len(e.CloseMatches) > 0 {
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

	for _, task := range tasks {
		res, err := Resolve(task, catalog, packageManager, versionManager)
		if err != nil {
			return nil, err
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
	nodeVariants := findVariants(task, catalog)
	if len(nodeVariants) == 0 {
		if _, ok := catalog[task]; ok {
			return Resolution{LogicalTask: task, SourceModule: task}, nil
		}

		return Resolution{}, &ResolveError{
			LogicalTask:  task,
			Attempted:    "",
			Message:      "task not found in store",
			CloseMatches: closeMatches(task, catalogKeys(catalog), maxCloseMatches),
		}
	}

	if packageManager == "" {
		return Resolution{}, &ResolveError{
			LogicalTask: task,
			Attempted:   "",
			Message: fmt.Sprintf(
				`Task %q requires js configuration for Node tasks. Set js.runtime to bun or nodejs.`,
				task,
			),
			CloseMatches: nil,
		}
	}

	attempted, err := variants.BuildSourceModule(task, packageManager, versionManager)
	if err != nil {
		return Resolution{}, &ResolveError{
			LogicalTask:  task,
			Attempted:    "",
			Message:      err.Error(),
			CloseMatches: nil,
		}
	}

	if _, ok := catalog[attempted]; !ok {
		return Resolution{}, &ResolveError{
			LogicalTask:  task,
			Attempted:    attempted,
			Message:      "source module not found in store",
			CloseMatches: closeMatches(attempted, catalogKeys(catalog), maxCloseMatches),
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
	keys := make([]string, 0, len(catalog))
	for key := range catalog {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func closeMatches(query string, candidates []string, limit int) []string {
	type scored struct {
		name  string
		score int
	}

	var scores []scored

	for _, candidate := range candidates {
		score := similarity(query, candidate)
		if score > 0 {
			scores = append(scores, scored{name: candidate, score: score})
		}
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].name < scores[j].name
		}

		return scores[i].score > scores[j].score
	})

	var out []string
	for i := 0; i < len(scores) && i < limit; i++ {
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
	if leftLen == 0 || rightLen == 0 {
		return 0
	}

	prev := make([]int, rightLen+1)

	curr := make([]int, rightLen+1)
	for col := 0; col <= rightLen; col++ {
		prev[col] = col
	}

	for row := 1; row <= leftLen; row++ {
		curr[0] = row

		for col := 1; col <= rightLen; col++ {
			cost := 1
			if left[row-1] == right[col-1] {
				cost = 0
			}

			curr[col] = minInt3(curr[col-1]+1, prev[col]+1, prev[col-1]+cost)
		}

		prev, curr = curr, prev
	}

	dist := prev[rightLen]
	maxLen := max(leftLen, rightLen)

	return max(0, maxSimilarityScore-(dist*maxSimilarityScore/maxLen))
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
