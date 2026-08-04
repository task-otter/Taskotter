// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package domain holds PR-facing DTOs.
package domain

import (
	"errors"
)

type (
	// PullRequest is a minimal view of a GitHub pull request.
	PullRequest struct {
		URL    string
		Number int
	}

	// StoreRef is the store version metadata shown in a PR body.
	StoreRef struct {
		SourceRef      string
		ResolvedCommit string
		DefaultBranch  string
	}

	// CreatePRRequest opens a pull request from branch into base.
	CreatePRRequest struct {
		Branch string
		Base   string
		Body   string
	}
)

// ErrPullRequestNotFound indicates no open pull request exists for the branch.
var ErrPullRequestNotFound = errors.New("open pull request not found")
