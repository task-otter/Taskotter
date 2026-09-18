// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"github.com/task-otter/Taskotter/internal/features/resolve/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
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
)
