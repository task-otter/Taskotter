// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
)

type (
	// Result captures sync outcomes for logging, GitHub Actions output, and PR metadata.
	Result struct {
		Plan                 *syncdomain.Plan
		Ref                  storedomain.RefInfo
		StoreVersion         string
		SourceRef            string
		SourceSHA            string
		TargetFolder         string
		ResolvedTasksJSON    string
		ResolvedDependencies string
		PullRequestNumber    string
		PullRequestURL       string
		Changed              bool
	}

	// ResolvedTask is the JSON representation of a resolved task module mapping.
	ResolvedTask struct {
		SourceModule      string
		DestinationModule string
		Path              string
	}
)
