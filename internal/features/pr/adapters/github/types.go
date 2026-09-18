// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
	"context"

	"github.com/task-otter/Taskotter/internal/features/pr/ports"
	"github.com/task-otter/Taskotter/internal/shared/githubapi"
)

type (
	clientFns = struct {
		createPR   func(context.Context, *ports.CreatePRRequest) (*ports.PullRequest, error)
		findOpenPR func(context.Context, string, string) (*ports.PullRequest, error)
		updateBody func(context.Context, int, string) error
	}

	listOpenPRArgs = struct {
		api    *githubapi.Client
		owner  string
		repo   string
		branch string
		base   string
	}

	// Client wraps the GitHub API for TaskOtter sync operations.
	Client struct {
		fns clientFns
	}
)
