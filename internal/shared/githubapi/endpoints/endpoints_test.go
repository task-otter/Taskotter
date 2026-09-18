// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package endpoints

import (
	"testing"
)

// TestPullPaths verifies pull-request endpoint construction.
func TestPullPaths(t *testing.T) {
	if got := PullsPath("owner", "repo"); got != "/repos/owner/repo/pulls" {
		t.Fatalf("pulls path = %q", got)
	}

	if got := PullPath("owner", "repo", 7); got != "/repos/owner/repo/pulls/7" {
		t.Fatalf("pull path = %q", got)
	}
}

// TestSplitPathQuery verifies relative path and query splitting.
func TestSplitPathQuery(t *testing.T) {
	path, query := SplitPathQuery("/repos/o/r/pulls?state=open")

	if path != "repos/o/r/pulls" || query != "state=open" {
		t.Fatalf("split = %q/%q", path, query)
	}
}
