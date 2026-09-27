// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:revive // These public domain types form the resolver's explicit API.
package service

import (
	"github.com/task-otter/Taskotter/internal/features/resolve/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

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

	// CollisionError reports two source modules normalizing to the same destination.
	CollisionError struct {
		SourceA     string
		SourceB     string
		Destination string
	}

	// taskContext bundles the catalog and JS settings shared across one task resolution.
	taskContext = struct {
		catalog        map[string]struct{}
		packageManager config.PackageManager
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

	pkgMgr = config.PackageManager

	// Resolution is a resolved logical task and its source module.
	Resolution = domain.Resolution
	// ResolveError is a user-facing module resolution failure.
	ResolveError = domain.ResolveError

	// ResolveInput selects one logical task and JS runtime settings.
	ResolveInput = struct {
		Task           string
		Catalog        map[string]struct{}
		PackageManager config.PackageManager
	}

	// ResolveAllInput resolves multiple logical tasks against one catalog.
	ResolveAllInput = struct {
		Catalog        map[string]struct{}
		PackageManager config.PackageManager
		Tasks          []string
	}
)
