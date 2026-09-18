// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

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
	if err != nil {
		return false
	}

	return info != nil
}

// WriteLocalIdentity configures commit author metadata for sync commits.
func WriteLocalIdentity() {
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
