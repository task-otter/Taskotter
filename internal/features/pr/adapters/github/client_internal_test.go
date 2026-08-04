// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/pr/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/githubapi"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	errBodyNope    = "nope"
	testSyncBranch = "taskotter/sync-abc"
	testMainBranch = "main"
	testPRBody     = "body"
	testOwner      = "owner"
	testRepoName   = "repo"
	testToken      = "token"
	pathReposPulls = "/repos/owner/repo/pulls"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (client *Client, cleanup func()) {
	t.Helper()

	srv := httptest.NewServer(handler)

	api, err := githubapi.NewClientWithHTTP(srv.URL+"/", srv.Client())
	if err != nil {
		t.Fatal(err)
	}

	return &Client{api: api, owner: testOwner, repo: testRepoName}, srv.Close
}

func writeTestResponse(writer http.ResponseWriter, payload []byte) {
	writeErr := iox.WriteFull(writer, payload)
	if writeErr != nil {
		return
	}
}

// TestNewClientParsesRepository verifies the owner and repo name are parsed from the coordinate.
func TestNewClientParsesRepository(t *testing.T) {
	t.Parallel()

	client, err := NewClient(t.Context(), testToken, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}

	if client.owner != testOwner || client.repo != testRepoName {
		t.Fatalf("NewClient() = %#v", client)
	}
}

// TestNewClientRejectsInvalidRepository verifies an invalid repository coordinate returns an error.
func TestNewClientRejectsInvalidRepository(t *testing.T) {
	t.Parallel()

	client, err := NewClient(t.Context(), testToken, testOwner)
	iox.Discard(client)

	if err == nil {
		t.Fatal("NewClient() error = nil, want error")
	}
}

func findOpenPRHandler(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != pathReposPulls {
		writer.WriteHeader(http.StatusNotFound)

		return
	}

	writeTestResponse(writer, []byte(`[{"number":7,"html_url":"https://example/pr/7"}]`))
}

// TestFindOpenPR verifies an open pull request is returned for the branch/base pair.
func TestFindOpenPR(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, findOpenPRHandler)

	t.Cleanup(cleanup)

	pullRequest, err := client.FindOpenPR(t.Context(), testSyncBranch, testMainBranch)
	if err != nil {
		t.Fatal(err)
	}

	if pullRequest.Number != consts.IndexSeven || pullRequest.URL == consts.Empty {
		t.Fatalf("FindOpenPR() = %#v", pullRequest)
	}
}

// TestFindOpenPRNotFound verifies ErrPullRequestNotFound is returned when no PR matches.
func TestFindOpenPRNotFound(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writeTestResponse(writer, []byte(`[]`))
	})

	t.Cleanup(cleanup)

	pullRequest, err := client.FindOpenPR(t.Context(), testSyncBranch, testMainBranch)
	iox.Discard(pullRequest)

	if !errors.Is(err, domain.ErrPullRequestNotFound) {
		t.Fatalf("FindOpenPR() error = %v, want ErrPullRequestNotFound", err)
	}
}

// TestFindOpenPRError verifies API failures propagate from FindOpenPR.
func TestFindOpenPRError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		writeTestResponse(writer, []byte(errBodyNope))
	})

	t.Cleanup(cleanup)

	pullRequest, err := client.FindOpenPR(t.Context(), testSyncBranch, testMainBranch)
	iox.Discard(pullRequest)

	if err == nil {
		t.Fatal("FindOpenPR() error = nil, want error")
	}
}

func testCreatePRRequest() *domain.CreatePRRequest {
	return &domain.CreatePRRequest{
		Branch: testSyncBranch,
		Base:   testMainBranch,
		Body:   testPRBody,
	}
}

// TestCreatePR verifies CreatePR returns the created pull request.
func TestCreatePR(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writeTestResponse(writer, []byte(`{"number":9,"html_url":"https://example/pr/9"}`))
	})

	t.Cleanup(cleanup)

	pullRequest, err := client.CreatePR(t.Context(), testCreatePRRequest())
	if err != nil {
		t.Fatal(err)
	}

	if pullRequest.Number != consts.IndexNine {
		t.Fatalf("CreatePR() = %#v", pullRequest)
	}
}

// TestCreatePRError verifies API failures propagate from CreatePR.
func TestCreatePRError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		writeTestResponse(writer, []byte(errBodyNope))
	})

	t.Cleanup(cleanup)

	res, err := client.CreatePR(t.Context(), testCreatePRRequest())
	iox.Discard(res)

	if err == nil {
		t.Fatal("CreatePR() error = nil, want error")
	}
}

// TestUpdatePRBody verifies UpdatePRBody succeeds for a matching pull request.
func TestUpdatePRBody(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		writeTestResponse(writer, []byte(`{}`))
	})

	t.Cleanup(cleanup)

	err := client.UpdatePRBody(t.Context(), consts.IndexSeven, testPRBody)
	if err != nil {
		t.Fatal(err)
	}
}

// TestUpdatePRBodyError verifies API failures propagate from UpdatePRBody.
func TestUpdatePRBodyError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		writeTestResponse(writer, []byte(errBodyNope))
	})

	t.Cleanup(cleanup)

	err := client.UpdatePRBody(t.Context(), consts.IndexSeven, testPRBody)
	if err == nil {
		t.Fatal("UpdatePRBody() error = nil, want error")
	}
}
