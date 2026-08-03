// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package dependency resolves module dependency graphs declared in .deps.yml.
package dependency

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type (

	// CycleError reports a circular dependency chain.
	CycleError struct {
		Path []string
	}

	// MissingDependencyError reports a dependency missing from .deps.yml.
	MissingDependencyError struct {
		Module     string
		Dependency string
	}

	// visitState holds the dependency graph and visited-module set shared across a visitModule recursion.
	visitState struct {
		deps   map[string][]string
		needed map[string]struct{}
	}
)

var errModuleNotDefined = errors.New("module is not defined in .deps.yml")

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

func (s *visitState) markVisited(module string) bool {
	if _, ok := s.needed[module]; ok {
		return true
	}

	s.needed[module] = struct{}{}

	return false
}

// ResolveTransitive returns dependency modules required by requested, excluding requested modules themselves.
func ResolveTransitive(requested []string, deps map[string][]string) ([]string, error) {
	state := &visitState{deps: deps, needed: make(map[string]struct{})}

	for i := range requested {
		err := visitModule(requested[i], nil, state)
		if err != nil {
			return nil, fmt.Errorf("visit module %q: %w", requested[i], err)
		}
	}

	return transitiveDependencies(requested, state.needed), nil
}

func visitModule(module string, stack []string, state *visitState) error {
	err := checkModuleDefined(module, state.deps)
	if err != nil {
		return fmt.Errorf("check module defined: %w", err)
	}

	err = detectCycle(module, stack)
	if err != nil {
		return fmt.Errorf("detect cycle: %w", err)
	}

	if state.markVisited(module) {
		return nil
	}

	err = visitDependencies(module, stack, state)
	if err != nil {
		return fmt.Errorf("visit dependencies of %q: %w", module, err)
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

		err := visitDependency(dep, module, stack, state)
		if err != nil {
			return fmt.Errorf("visit dependency %q: %w", dep, err)
		}
	}

	return nil
}

func visitDependency(dep, module string, stack []string, state *visitState) error {
	if _, ok := state.deps[dep]; !ok {
		return &MissingDependencyError{Module: module, Dependency: dep}
	}

	err := visitModule(dep, append(stack, module), state)
	if err != nil {
		return fmt.Errorf("visit module %q: %w", dep, err)
	}

	return nil
}

func transitiveDependencies(requested []string, needed map[string]struct{}) []string {
	requestedSet := make(map[string]struct{}, len(requested))

	for i := range requested {
		requestedSet[requested[i]] = struct{}{}
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
