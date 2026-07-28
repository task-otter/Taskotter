// Package normalizer maps store module names to destination folder names.
package normalizer

import (
	"errors"
	"fmt"
	"sort"

	"github.com/task-otter/Taskotter/internal/variants"
)

var errEmptyNormalizedName = errors.New("normalized name is empty")

// CollisionError reports two source modules normalizing to the same destination.
type CollisionError struct {
	SourceA     string
	SourceB     string
	Destination string
}

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

// Mapping records a source module and its normalized destination name.
type Mapping struct {
	Source      string
	Destination string
}

// BuildDestinationMap normalizes each source module and rejects destination collisions.
func BuildDestinationMap(sources []string) (map[string]string, error) {
	destToSource := make(map[string]string, len(sources))
	result := make(map[string]string, len(sources))

	for _, source := range sources {
		dest, err := Normalize(source)
		if err != nil {
			return nil, err
		}

		if existing, ok := destToSource[dest]; ok && existing != source {
			return nil, &CollisionError{SourceA: existing, SourceB: source, Destination: dest}
		}

		destToSource[dest] = source
		result[source] = dest
	}

	return result, nil
}

// SortedSources returns map keys sorted lexicographically.
func SortedSources(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for source := range m {
		out = append(out, source)
	}

	sort.Strings(out)

	return out
}
