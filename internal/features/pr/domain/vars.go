// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"errors"
)

// ErrPullRequestNotFound indicates no open pull request exists for the branch.
var ErrPullRequestNotFound = errors.New("open pull request not found")
