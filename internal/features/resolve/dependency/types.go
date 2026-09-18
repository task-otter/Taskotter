// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package dependency

type (
	// DepsResolver resolves transitive module dependencies.
	DepsResolver interface {
		Resolve(requested []string, deps map[string][]string) ([]string, error)
	}

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
	visitState = struct {
		deps   map[string][]string
		needed map[string]struct{}
	}

	// visitContext holds stack and state for dependency traversal.
	visitContext = struct {
		state *visitState
		stack []string
	}

	transitiveResolver struct{}
)
