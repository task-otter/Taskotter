// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
	"errors"
)

var (
	errArchiveAuthFailed = errors.New("authentication failed downloading store archive")

	errArchiveRateLimit = errors.New("GitHub rate limit exceeded downloading store archive")

	errArchiveDownloadFailed = errors.New("download store archive failed")

	errGitHubAPIFailed = errors.New("GitHub API request failed")

	errDefaultBranchEmpty = errors.New("store repository default branch is empty")

	errStoreTagNotFound = errors.New("store tag does not exist")

	errResolveTagFailed = errors.New("resolve tag failed")
)
