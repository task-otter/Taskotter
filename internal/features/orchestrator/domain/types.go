// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"github.com/task-otter/Taskotter/internal/features/plan"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
)

type (
	// Result describes the result.
	//nolint:reusability // The action's public result is intentionally tailored to its output contract.
	Result struct {
		ResolvedOutput
		PullRequestOutput

		Plan         *plan.Plan
		Ref          storedomain.RefInfo
		StoreVersion string
		SourceRef    string
		SourceSHA    string
		TargetFolder string
		Changed      bool
	}

	// ResolvedOutput describes the resolved \1utput.
	ResolvedOutput struct {
		ResolvedTasksJSON    string
		ResolvedDependencies string
	}

	// PullRequestOutput describes the pull \1equest \1utput.
	PullRequestOutput struct {
		PullRequestNumber string
		PullRequestURL    string
	}

	// ResolvedTask describes the resolved \1ask.
	ResolvedTask struct {
		SourceModule      string
		DestinationModule string
		Path              string
	}
)
