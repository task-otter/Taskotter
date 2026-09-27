// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package githubapi

import (
	"errors"
	"net/http"
)

var (
	_ doer = (*http.Client)(nil)

	errGitHubAPIStatus = errors.New("GitHub API status error")
)
