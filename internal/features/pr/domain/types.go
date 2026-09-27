// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// PullRequest is a minimal view of a GitHub pull request.
	PullRequest = struct {
		URL    string
		Number int
	}

	// StoreRef is the store version metadata shown in a PR body.
	StoreRef = struct {
		SourceRef      string
		ResolvedCommit string
		DefaultBranch  string
	}

	// CreatePRRequest opens a pull request from branch into base.
	CreatePRRequest = struct {
		Branch string
		Base   string
		Body   string
	}
)
