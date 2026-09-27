// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"
)

type (
	// PullRequest is a minimal view of a GitHub pull request.
	PullRequest = struct {
		URL    string
		Number int
	}

	// CreatePRRequest opens a pull request from branch into base.
	CreatePRRequest = struct {
		Branch string
		Base   string
		Body   string
	}

	// PRClient finds and mutates TaskOtter sync pull requests.
	PRClient interface {
		FindOpenPR(ctx context.Context, branch, base string) (*PullRequest, error)
		CreatePR(ctx context.Context, req *CreatePRRequest) (*PullRequest, error)
		UpdatePRBody(ctx context.Context, number int, body string) error
	}
)
