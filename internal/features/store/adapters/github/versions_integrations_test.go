// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/store/adapters/github"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	latestVersionCase struct {
		name string
		want string
		tags []string
	}
)

const (
	storeTagsPath            = storeRepoPath + "/tags"
	versionTag19             = "v1.9.0"
	versionTag110            = "v1.10.0"
	versionTag14             = "v1.4.0"
	versionTag14Bare         = "1.4.0"
	versionTagBuild          = "v1.2.3+build"
	versionTagBare           = "1.2.3"
	versionPageKey           = "page"
	versionPageSizeText      = "100"
	versionPageSize          = 100
	versionLookupErrorFormat = "LatestStableVersion() = %q, %v; want empty version and error"
)

// TestLatestStableVersion verifies semantic ordering and filtering across unsorted tags.
//
//nolint:dupl // Selection and failure tables exercise distinct HTTP outcomes.
func TestLatestStableVersion(t *testing.T) {
	t.Parallel()

	cases := latestVersionCases()

	for i := range cases {
		testCase := &cases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertLatestStableVersion(t, testCase)
		})
	}
}

// TestLatestStableVersionPagination verifies later pages are considered before choosing the newest tag.
func TestLatestStableVersionPagination(t *testing.T) {
	t.Parallel()

	client := newStoreTestClient(t, tagPagesHandler(t))
	assertLatestVersion(t, client, versionTag110)
}

// TestLatestStableVersionErrors verifies failed and malformed lookups never return a partial latest tag.
//
//nolint:dupl // Selection and failure tables exercise distinct HTTP outcomes.
func TestLatestStableVersionErrors(t *testing.T) {
	t.Parallel()

	cases := latestVersionErrorCases()

	for i := range cases {
		testCase := &cases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertLatestVersionError(t, testCase)
		})
	}
}

// TestLatestStableVersionLaterPageFailure verifies an incomplete listing is reported as a failed check.
func TestLatestStableVersionLaterPageFailure(t *testing.T) {
	t.Parallel()

	client := newStoreTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get(versionPageKey) == "1" {
			writeTagsResponse(t, writer, fullTagsPage())

			return
		}

		writer.WriteHeader(http.StatusTooManyRequests)
	})
	got, err := client.LatestStableVersion(t.Context())

	if err == nil || got != "" {
		t.Fatalf(versionLookupErrorFormat, got, err)
	}
}

// TestLatestStableVersionCancelled verifies tag requests honor the caller's context.
func TestLatestStableVersionCancelled(t *testing.T) {
	t.Parallel()

	client := newStoreTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		http.NotFound(writer, req)
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	got, err := client.LatestStableVersion(ctx)

	if !errors.Is(err, context.Canceled) || got != "" {
		t.Fatalf("LatestStableVersion() = %q, %v; want empty version and cancellation", got, err)
	}
}

func assertTagsRequest(t *testing.T, req *http.Request, page string) {
	t.Helper()

	want := fmt.Sprintf("%s?per_page=%s&page=%s", storeTagsPath, versionPageSizeText, page)

	if req.Method != http.MethodGet || req.URL.RequestURI() != want {
		t.Errorf("unexpected tag request: %s %s", req.Method, req.URL)
	}

	if req.Header.Get(headerAuthorization) != "Bearer "+testToken {
		t.Error("tag request did not use the configured authentication")
	}
}

func writeTagsResponse(t *testing.T, writer http.ResponseWriter, tags []string) {
	t.Helper()

	payload := make([]map[string]string, consts.IndexZero, len(tags))

	for i := range tags {
		payload = append(payload, map[string]string{"name": tags[i]})
	}

	err := json.NewEncoder(writer).Encode(payload)
	if err != nil {
		t.Errorf("write tags response: %v", err)
	}
}

func assertLatestStableVersion(t *testing.T, testCase *latestVersionCase) {
	t.Helper()

	client := newStoreTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		assertTagsRequest(t, req, "1")
		writeTagsResponse(t, writer, testCase.tags)
	})
	assertLatestVersion(t, client, testCase.want)
}

func assertLatestVersion(t *testing.T, client *github.Client, want string) {
	t.Helper()

	got, err := client.LatestStableVersion(t.Context())

	if err != nil || got != want {
		t.Fatalf("LatestStableVersion() = %q, %v; want %q, nil", got, err, want)
	}
}

func assertLatestVersionError(t *testing.T, testCase *refErrCase) {
	t.Helper()

	client := newStoreTestClient(t, statusBodyHandler(t, &statusHandlerArgs{
		path: storeTagsPath, status: testCase.status, body: testCase.body,
	}))
	got, err := client.LatestStableVersion(t.Context())

	if err == nil || got != "" {
		t.Fatalf(versionLookupErrorFormat, got, err)
	}
}

func fullTagsPage() []string {
	tags := make([]string, consts.IndexZero, versionPageSize)

	for range versionPageSize {
		tags = append(tags, versionTag19)
	}

	return tags
}

//nolint:funlen,maintidx // Explicit tag cases keep ordering and filtering expectations together.
func latestVersionCases() []latestVersionCase {
	return []latestVersionCase{
		{
			name: "numeric order",
			tags: []string{versionTag19, versionTag110, testTagV123},
			want: versionTag110,
		},
		{
			name: "stable only",
			tags: []string{"v2.0.0-rc.1", "release", versionTag14, "v01.5.0"},
			want: versionTag14,
		},
		{name: "no prefix", tags: []string{testTagV123, versionTag14Bare}, want: versionTag14Bare},
		{
			name: "equal versions",
			tags: []string{versionTagBuild, testTagV123, versionTagBare},
			want: versionTagBare,
		},
		{
			name: "equal versions reversed",
			tags: []string{versionTagBare, testTagV123, versionTagBuild},
			want: versionTagBare,
		},
		{name: "no stable tags", tags: []string{"v2.0.0-beta", "custom"}, want: ""},
		{name: "empty store", tags: []string{}, want: ""},
	}
}

func latestVersionErrorCases() []refErrCase {
	return []refErrCase{
		{name: "unauthorized", status: http.StatusUnauthorized, body: bodyEmptyJSON},
		{name: "forbidden", status: http.StatusForbidden, body: bodyEmptyJSON},
		{name: "rate limited", status: http.StatusTooManyRequests, body: bodyEmptyJSON},
		{name: "tags server failure", status: http.StatusInternalServerError, body: bodyEmptyJSON},
		{name: "invalid JSON", status: http.StatusOK, body: "{"},
		{name: "invalid payload", status: http.StatusOK, body: `[{"name":42}]`},
	}
}

func tagPagesHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get(versionPageKey) == "1" {
			assertTagsRequest(t, req, "1")
			writeTagsResponse(t, writer, fullTagsPage())

			return
		}

		assertTagsRequest(t, req, "2")
		writeTagsResponse(t, writer, []string{versionTag110})
	}
}
