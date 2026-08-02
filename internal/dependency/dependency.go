// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package dependency

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var errModuleNotDefined = errors.New("module is not defined in .deps.yml")

// CycleError reports a circular dependency chain.
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	return "dependency cycle detected: " + stringsJoinArrow(e.Path)
}

// MissingDependencyError reports a dependency missing from .deps.yml.
type MissingDependencyError struct {
	Module     string
	Dependency string
}

func (e *MissingDependencyError) Error() string {
	return fmt.Sprintf("module %q depends on missing module %q", e.Module, e.Dependency)
}

func stringsJoinArrow(parts []string) string {
	return strings.Join(parts, " -> ")
}

// ResolveTransitive returns dependency modules required by requested, excluding requested modules themselves.
func ResolveTransitive(requested []string, deps map[string][]string) ([]string, error) {
	needed := make(map[string]struct{})

	for _, module := range requested {
		err := visitModule(module, nil, deps, needed)
		if err != nil {
			return nil, err
		}
	}

	return transitiveDependencies(requested, needed), nil
}

func visitModule(
	module string,
	stack []string,
	deps map[string][]string,
	needed map[string]struct{},
) error {
	if _, ok := deps[module]; !ok {
		return fmt.Errorf("%w: %q", errModuleNotDefined, module)
	}

	for i, s := range stack {
		if s == module {
			cycle := append(append([]string{}, stack[i:]...), module)

			return &CycleError{Path: cycle}
		}
	}

	if _, ok := needed[module]; ok {
		return nil
	}

	needed[module] = struct{}{}

	for _, dep := range deps[module] {
		if _, ok := deps[dep]; !ok {
			return &MissingDependencyError{Module: module, Dependency: dep}
		}

		err := visitModule(dep, append(stack, module), deps, needed)
		if err != nil {
			return err
		}
	}

	return nil
}

func transitiveDependencies(requested []string, needed map[string]struct{}) []string {
	requestedSet := make(map[string]struct{}, len(requested))

	for _, module := range requested {
		requestedSet[module] = struct{}{}
	}

	var dependencies []string

	for module := range needed {
		if _, ok := requestedSet[module]; ok {
			continue
		}

		dependencies = append(dependencies, module)
	}

	sort.Strings(dependencies)

	return dependencies
}
