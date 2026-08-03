// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package normalizer maps store module source paths to normalized destination paths.
package normalizer

import (
	"errors"
	"fmt"
	"sort"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/variants"
)

type (

	// CollisionError reports two source modules normalizing to the same destination.
	CollisionError struct {
		SourceA     string
		SourceB     string
		Destination string
	}

	// Mapping records a source module and its normalized destination name.
	Mapping struct {
		Source      string
		Destination string
	}
)

var errEmptyNormalizedName = errors.New("normalized name is empty")

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
		next, changed := variants.StripOneSuffix(current)

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

	sort.Strings(out)

	return out
}
