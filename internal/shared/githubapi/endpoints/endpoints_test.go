// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package endpoints

import (
	"testing"
)

const (
	testPullNumber = 7
	testOwner      = "owner"
	testRepo       = "repo"
)

// TestPullPaths verifies pull-request endpoint construction.
func TestPullPaths(t *testing.T) {
	t.Parallel()

	if got := PullsPath(testOwner, testRepo); got != "/repos/owner/repo/pulls" {
		t.Fatalf("pulls path = %q", got)
	}

	if got := PullPath(testOwner, testRepo, testPullNumber); got != "/repos/owner/repo/pulls/7" {
		t.Fatalf("pull path = %q", got)
	}
}

// TestSplitPathQuery verifies relative path and query splitting.
func TestSplitPathQuery(t *testing.T) {
	t.Parallel()

	path, query := SplitPathQuery("/repos/o/r/pulls?state=open")

	if path != "repos/o/r/pulls" || query != "state=open" {
		t.Fatalf("split = %q/%q", path, query)
	}
}

func TestListOpenPRAndRelativeURL(t *testing.T) {
	t.Parallel()

	path := ListOpenPRPath(testOwner, testRepo, &OpenPRQuery{Head: "feature", Base: "main"})

	if path != "/repos/owner/repo/pulls?base=main&head=feature&state=open" {
		t.Fatalf("ListOpenPRPath() = %q", path)
	}

	url := RelativeURL("repos/owner", "state=open")

	if url.Path != "repos/owner" || url.RawQuery != "state=open" {
		t.Fatalf("RelativeURL() = %#v", url)
	}

	path, query := SplitPathQuery("repos/o/r")

	if path != "repos/o/r" || query != emptyQuery {
		t.Fatalf("split without query = %q/%q", path, query)
	}
}
