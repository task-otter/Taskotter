// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package ports defines git branch, index, and publish dependencies.
package ports

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/task-otter/Taskotter/internal/shared/iox"
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

const (
	// SyncCommitMessage is the commit message TaskOtter uses for sync branches.
	SyncCommitMessage = "chore(taskotter): sync taskfiles"
)

var errBranchNotOwned = errors.New("branch exists but is not owned by TaskOtter")

// AllowedPathSet converts staged path strings into a lookup set.
func AllowedPathSet(paths []string) map[string]struct{} {
	out := make(map[string]struct{}, len(paths))

	for i := range paths {
		out[filepath.ToSlash(paths[i])] = struct{}{}
	}

	return out
}

// EnsureBranchOwned allows new sync branches and rejects foreign branch reuse.
func EnsureBranchOwned(ctx context.Context, ops BranchChecker, branch string) error {
	exists, err := ops.BranchExists(ctx, branch)
	if err != nil {
		return fmt.Errorf("check branch exists: %w", err)
	}

	if !exists {
		return nil
	}

	err = verifyExistingBranchOwned(ctx, ops, branch)
	if err != nil {
		return fmt.Errorf("verify existing branch owned: %w", err)
	}

	return nil
}

// IsGitRepo reports whether workspace contains a .git directory.
func IsGitRepo(workspace string) bool {
	info, err := os.Stat(filepath.Join(workspace, ".git"))
	iox.Discard(info)

	return err == nil
}

// WriteLocalIdentity configures commit author metadata for sync commits.
func WriteLocalIdentity() {
	// Commit identity is applied per command via -c; config files are not writable
	// in GitHub Actions Docker containers.
}

func verifyExistingBranchOwned(ctx context.Context, ops BranchChecker, branch string) error {
	msg, err := ops.LastCommitMessage(ctx, branch)
	if err != nil {
		return fmt.Errorf("read last commit message: %w", err)
	}

	err = checkBranchOwnership(msg, branch)
	if err != nil {
		return fmt.Errorf("check branch ownership: %w", err)
	}

	return nil
}

func checkBranchOwnership(msg, branch string) error {
	if msg != SyncCommitMessage {
		return fmt.Errorf("%w: %q", errBranchNotOwned, branch)
	}

	return nil
}
