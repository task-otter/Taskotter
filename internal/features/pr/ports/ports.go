// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package ports defines PR feature outbound dependencies.
package ports

import (
	"context"

	"github.com/task-otter/Taskotter/internal/features/pr/domain"
)

type (
	// PRClient finds and mutates TaskOtter sync pull requests.
	PRClient interface {
		FindOpenPR(ctx context.Context, branch, base string) (*domain.PullRequest, error)
		CreatePR(ctx context.Context, req *domain.CreatePRRequest) (*domain.PullRequest, error)
		UpdatePRBody(ctx context.Context, number int, body string) error
	}
)
