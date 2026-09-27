// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
)

type (
	Result struct {
		ResolvedOutput
		PullRequestOutput

		Plan         *syncdomain.Plan
		Ref          storedomain.RefInfo
		StoreVersion string
		SourceRef    string
		SourceSHA    string
		TargetFolder string
		Changed      bool
	}

	ResolvedOutput struct {
		ResolvedTasksJSON    string
		ResolvedDependencies string
	}

	PullRequestOutput struct {
		PullRequestNumber string
		PullRequestURL    string
	}

	ResolvedTask struct {
		SourceModule      string
		DestinationModule string
		Path              string
	}
)
