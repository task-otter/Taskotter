// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"
)

type (
	// BranchChecker reads branch metadata used for ownership checks.
	BranchChecker interface {
		BranchExists(ctx context.Context, branch string) (bool, error)
		LastCommitMessage(ctx context.Context, branch string) (string, error)
	}

	// Brancher manages local branch refs.
	Brancher interface {
		CheckoutBranch(ctx context.Context, branch string) error
		CreateOrResetBranch(ctx context.Context, branch string) error
		BranchExists(ctx context.Context, branch string) (bool, error)
		LastCommitMessage(ctx context.Context, branch string) (string, error)
		DefaultBranch(ctx context.Context) (string, error)
	}

	// Indexer inspects and stages the working tree.
	Indexer interface {
		HasUnrelatedChanges(ctx context.Context, set map[string]struct{}) (bool, error)
		Stage(ctx context.Context, paths []string) error
		Commit(ctx context.Context, message string) error
	}

	// Publisher pushes branches to origin.
	Publisher interface {
		Push(ctx context.Context, branch string) error
		PushForceWithLease(ctx context.Context, branch string) error
	}

	// Workspace prepares a git workspace before sync operations.
	Workspace interface {
		EnsureSafeDirectory()
		ConfigureCredentials(ctx context.Context, token, repository string) error
	}
)
